package exec

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/rules"
)

// Status values for a single target outcome.
const (
	StatusRemoved = "removed"
	StatusTrashed = "trashed"
	StatusFailed  = "failed"
	StatusSkipped = "skipped" // interrupted before attempt
)

// Options configures a mutating run.
type Options struct {
	Root    string
	ToTrash bool
	Yes     bool // recorded in journal (prompt was skipped)
	Trash   TrashFunc
	Journal string // empty → default state path
	Context context.Context
	Warn    io.Writer // cross-device / other warnings; default stderr
}

// Outcome is the result of attempting to remove one target.
type Outcome struct {
	Path      string
	Project   string
	Framework string
	Size      int64
	Status    string
	Err       error
}

// Result aggregates outcomes for a run.
type Result struct {
	Outcomes    []Outcome
	Interrupted bool
}

// Failed reports whether any target failed or the run was interrupted.
func (r *Result) Failed() bool {
	if r == nil {
		return false
	}
	if r.Interrupted {
		return true
	}
	for _, o := range r.Outcomes {
		if o.Status == StatusFailed {
			return true
		}
	}
	return false
}

// RemovedCount returns how many targets were successfully removed or trashed.
func (r *Result) RemovedCount() int {
	if r == nil {
		return 0
	}
	n := 0
	for _, o := range r.Outcomes {
		if o.Status == StatusRemoved || o.Status == StatusTrashed {
			n++
		}
	}
	return n
}

type pending struct {
	decision plan.Decision
	id       fileID
}

// Run deletes or trashes all VerdictDelete targets in p.
func Run(p *plan.Plan, opts Options) (*Result, error) {
	if p == nil {
		return nil, fmt.Errorf("plan is nil")
	}
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	warn := opts.Warn
	if warn == nil {
		warn = os.Stderr
	}
	root := opts.Root
	if root == "" {
		root = p.Root
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	deletes := filterDeletes(p)
	sortDeepestFirst(deletes)

	pendingList := make([]pending, 0, len(deletes))
	result := &Result{}

	for _, d := range deletes {
		id, err := identityFromPath(d.Target.Path)
		if err != nil {
			result.Outcomes = append(result.Outcomes, outcomeFrom(d, StatusFailed, err))
			continue
		}
		pendingList = append(pendingList, pending{decision: d, id: id})
	}

	for i, item := range pendingList {
		select {
		case <-ctx.Done():
			result.Interrupted = true
			for _, left := range pendingList[i:] {
				result.Outcomes = append(result.Outcomes, outcomeFrom(left.decision, StatusSkipped, ctx.Err()))
			}
			goto done
		default:
		}

		if err := revalidate(rootAbs, item.decision.Target.Path, item.id); err != nil {
			result.Outcomes = append(result.Outcomes, outcomeFrom(item.decision, StatusFailed, err))
			continue
		}

		err := trashOrRemove(item.decision.Target.Path, opts.ToTrash, opts.Trash, warn)
		if err != nil {
			result.Outcomes = append(result.Outcomes, outcomeFrom(item.decision, StatusFailed, err))
			continue
		}
		status := StatusRemoved
		if opts.ToTrash {
			status = StatusTrashed
		}
		result.Outcomes = append(result.Outcomes, outcomeFrom(item.decision, status, nil))
	}

done:
	rec := JournalRecord{
		Timestamp:   time.Now().UTC(),
		Root:        rootAbs,
		ToTrash:     opts.ToTrash,
		Yes:         opts.Yes,
		Interrupted: result.Interrupted,
		Outcomes:    toJournalOutcomes(result.Outcomes),
	}
	if err := appendJournal(opts.Journal, rec); err != nil {
		return result, fmt.Errorf("journal: %w", err)
	}
	return result, nil
}

func filterDeletes(p *plan.Plan) []plan.Decision {
	var out []plan.Decision
	for _, d := range p.Decisions {
		if d.Verdict == plan.VerdictDelete {
			out = append(out, d)
		}
	}
	return out
}

func sortDeepestFirst(ds []plan.Decision) {
	sort.SliceStable(ds, func(i, j int) bool {
		di := pathDepth(ds[i].Target.Path)
		dj := pathDepth(ds[j].Target.Path)
		if di != dj {
			return di > dj
		}
		return ds[i].Target.Path > ds[j].Target.Path
	})
}

func pathDepth(p string) int {
	p = filepath.Clean(p)
	if p == "" || p == "." || p == string(filepath.Separator) {
		return 0
	}
	n := 0
	for _, c := range p {
		if c == filepath.Separator {
			n++
		}
	}
	return n
}

func revalidate(rootAbs, path string, expected fileID) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path no longer exists")
		}
		return fmt.Errorf("stat: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("path is a symlink")
	}
	id, err := identityFromPath(path)
	if err != nil {
		return fmt.Errorf("identity: %w", err)
	}
	if id != expected {
		return fmt.Errorf("path identity changed since plan")
	}
	if reason := rules.CheckTarget(rootAbs, path); reason != "" {
		return fmt.Errorf("safety: %s", reason)
	}
	// Logical containment (CheckTarget already covers symlink escape when resolvable).
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(filepath.Clean(abs)+string(filepath.Separator), filepath.Clean(rootAbs)+string(filepath.Separator)) &&
		filepath.Clean(abs) != filepath.Clean(rootAbs) {
		return fmt.Errorf("path escapes scan root")
	}
	return nil
}

func outcomeFrom(d plan.Decision, status string, err error) Outcome {
	project := ""
	framework := ""
	if d.Project != nil {
		project = d.Project.Root
		framework = d.Project.Framework
	}
	return Outcome{
		Path:      d.Target.Path,
		Project:   project,
		Framework: framework,
		Size:      d.Target.Size,
		Status:    status,
		Err:       err,
	}
}

func toJournalOutcomes(outcomes []Outcome) []JournalOutcome {
	out := make([]JournalOutcome, 0, len(outcomes))
	for _, o := range outcomes {
		jo := JournalOutcome{
			Path:      o.Path,
			Project:   o.Project,
			Framework: o.Framework,
			Size:      o.Size,
			Status:    o.Status,
		}
		if o.Err != nil {
			jo.Error = o.Err.Error()
		}
		out = append(out, jo)
	}
	return out
}

// SummaryLine returns a short human summary of the run.
func SummaryLine(r *Result) string {
	if r == nil {
		return ""
	}
	removed := r.RemovedCount()
	failed := 0
	skipped := 0
	for _, o := range r.Outcomes {
		switch o.Status {
		case StatusFailed:
			failed++
		case StatusSkipped:
			skipped++
		}
	}
	switch {
	case r.Interrupted:
		return fmt.Sprintf("interrupted: removed %d, failed %d, remaining %d", removed, failed, skipped)
	case failed > 0:
		return fmt.Sprintf("completed with partial failures: removed %d, failed %d", removed, failed)
	default:
		return fmt.Sprintf("removed %d targets", removed)
	}
}
