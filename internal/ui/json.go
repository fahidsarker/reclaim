package ui

import (
	"encoding/json"
	"io"
	"path/filepath"
	"time"

	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/rules"
)

// JSONOptions controls machine-readable plan output.
type JSONOptions struct {
	NoSize    bool
	ScannedAt time.Time // zero → time.Now()
}

type jsonPlan struct {
	Version   int           `json:"version"`
	Root      string        `json:"root"`
	ScannedAt string        `json:"scannedAt"`
	Projects  []jsonProject `json:"projects"`
	Skipped   []jsonSkipped `json:"skipped"`
	Totals    jsonTotals    `json:"totals"`
}

type jsonProject struct {
	Root      string       `json:"root"`
	Framework string       `json:"framework"`
	Targets   []jsonTarget `json:"targets"`
}

type jsonTarget struct {
	Path       string `json:"path"`
	RelPath    string `json:"relPath"`
	Size       *int64 `json:"size"` // null when unknown / --no-size
	ModTime    string `json:"modTime,omitempty"`
	Reason     string `json:"reason"`
	Regenerate string `json:"regenerate,omitempty"`
	Verdict    string `json:"verdict"`
}

type jsonSkipped struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Hint   string `json:"hint,omitempty"`
}

type jsonTotals struct {
	Projects int   `json:"projects"`
	Targets  int   `json:"targets"`
	Bytes    int64 `json:"bytes"`
}

// WriteJSON emits the stable plan schema from spec.md §10.
func WriteJSON(w io.Writer, p *plan.Plan, opts JSONOptions) error {
	if p == nil {
		return nil
	}
	at := opts.ScannedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}

	byRoot := map[string]*jsonProject{}
	var order []string
	var skipped []jsonSkipped
	var totalBytes int64
	var deleteTargets int
	unknownBytes := false

	for _, d := range p.Decisions {
		switch d.Verdict {
		case plan.VerdictSkipped, plan.VerdictKept:
			hint := rules.HintForReason(d.Reason, filepath.Base(d.Target.Path))
			skipped = append(skipped, jsonSkipped{
				Path:   d.Target.Path,
				Reason: d.Reason,
				Hint:   hint,
			})
		case plan.VerdictDelete:
			deleteTargets++
			projRoot := p.Root
			framework := ""
			if d.Project != nil {
				projRoot = d.Project.Root
				framework = d.Project.Framework
			}
			jp, ok := byRoot[projRoot]
			if !ok {
				jp = &jsonProject{Root: projRoot, Framework: framework}
				byRoot[projRoot] = jp
				order = append(order, projRoot)
			}
			jt := jsonTarget{
				Path:       d.Target.Path,
				RelPath:    d.Target.RelPath,
				Reason:     d.Reason,
				Regenerate: d.Target.Regenerate,
				Verdict:    d.Verdict.String(),
			}
			if !opts.NoSize && d.Target.Size >= 0 {
				sz := d.Target.Size
				jt.Size = &sz
				totalBytes += sz
			} else if !opts.NoSize && d.Target.Size == plan.SizeUnknown {
				unknownBytes = true
				_ = unknownBytes
			}
			if !d.Target.ModTime.IsZero() {
				jt.ModTime = d.Target.ModTime.UTC().Format(time.RFC3339)
			}
			jp.Targets = append(jp.Targets, jt)
		}
	}

	projects := make([]jsonProject, 0, len(order))
	for _, r := range order {
		projects = append(projects, *byRoot[r])
	}

	out := jsonPlan{
		Version:   1,
		Root:      p.Root,
		ScannedAt: at.Format(time.RFC3339),
		Projects:  projects,
		Skipped:   skipped,
		Totals: jsonTotals{
			Projects: len(projects),
			Targets:  deleteTargets,
			Bytes:    totalBytes,
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
