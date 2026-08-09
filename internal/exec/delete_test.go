package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/plan"
)

func TestRun_PermanentDelete(t *testing.T) {
	root := t.TempDir()
	journal := filepath.Join(t.TempDir(), "history.jsonl")
	target := filepath.Join(root, "node_modules")
	mustMkdir(t, filepath.Join(target, "x"))

	p := &plan.Plan{
		Root:      root,
		Decisions: []plan.Decision{deleteDecision(root, "nodejs", target)},
	}
	res, err := Run(p, Options{Root: root, Journal: journal, Warn: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed() || res.RemovedCount() != 1 {
		t.Fatalf("result=%+v", res)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists: %v", err)
	}
	assertJournal(t, journal, 1, StatusRemoved)
}

func TestRun_DeepestFirst(t *testing.T) {
	root := t.TempDir()
	journal := filepath.Join(t.TempDir(), "history.jsonl")

	outer := filepath.Join(root, "node_modules")
	inner := filepath.Join(root, "packages", "api", "node_modules")
	mustMkdir(t, filepath.Join(outer, "pkg"))
	mustMkdir(t, filepath.Join(inner, "pkg"))

	order := make([]string, 0, 2)
	p := &plan.Plan{
		Root: root,
		Decisions: []plan.Decision{
			deleteDecision(root, "nodejs", outer),
			deleteDecision(filepath.Join(root, "packages", "api"), "nodejs", inner),
		},
	}

	res, err := Run(p, Options{
		Root:    root,
		Journal: journal,
		ToTrash: true,
		Warn:    io.Discard,
		Trash: func(path string, warn io.Writer) error {
			order = append(order, path)
			return permanentlyRemove(path)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed() {
		t.Fatalf("unexpected failures: %+v", res.Outcomes)
	}
	if len(order) != 2 {
		t.Fatalf("order len=%d want 2: %v", len(order), order)
	}
	if pathDepth(order[0]) < pathDepth(order[1]) {
		t.Fatalf("expected deepest first, got %v", order)
	}
	assertJournal(t, journal, 2, StatusTrashed)
}

func TestRun_FakeTrash(t *testing.T) {
	root := t.TempDir()
	journal := filepath.Join(t.TempDir(), "history.jsonl")
	trashDir := t.TempDir()
	target := filepath.Join(root, "node_modules")
	mustMkdir(t, filepath.Join(target, "x"))
	mustWrite(t, filepath.Join(target, "x", "f"), "data")

	var trashed []string
	p := &plan.Plan{
		Root:      root,
		Decisions: []plan.Decision{deleteDecision(root, "nodejs", target)},
	}
	res, err := Run(p, Options{
		Root:    root,
		ToTrash: true,
		Journal: journal,
		Warn:    io.Discard,
		Trash: func(path string, warn io.Writer) error {
			trashed = append(trashed, path)
			dest := filepath.Join(trashDir, filepath.Base(path))
			return os.Rename(path, dest)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed() {
		t.Fatalf("failures: %+v", res.Outcomes)
	}
	if len(trashed) != 1 || trashed[0] != target {
		t.Fatalf("trashed=%v", trashed)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("original path still present")
	}
	if _, err := os.Lstat(filepath.Join(trashDir, "node_modules", "x", "f")); err != nil {
		t.Fatalf("trash bucket missing content: %v", err)
	}
	assertJournal(t, journal, 1, StatusTrashed)
}

func TestRun_RevalidationIdentityChanged(t *testing.T) {
	root := t.TempDir()
	journal := filepath.Join(t.TempDir(), "history.jsonl")
	// Same depth: sort is path descending, so z_first is processed before a_second.
	first := filepath.Join(root, "z_first")
	second := filepath.Join(root, "a_second")
	mustMkdir(t, first)
	mustMkdir(t, second)

	oldID, err := identityFromPath(second)
	if err != nil {
		t.Fatal(err)
	}

	p := &plan.Plan{
		Root: root,
		Decisions: []plan.Decision{
			deleteDecision(root, "nodejs", first),
			deleteDecision(root, "nodejs", second),
		},
	}
	res, err := Run(p, Options{
		Root:    root,
		Journal: journal,
		Warn:    io.Discard,
		ToTrash: true,
		Trash: func(path string, warn io.Writer) error {
			if path == first {
				// Linux/ext4 often recycles the freed inode on immediate recreate.
				// Burn sink inodes until a_second gets a different identity.
				changed := false
				for i := 0; i < 32; i++ {
					if err := os.RemoveAll(second); err != nil {
						return err
					}
					sink := filepath.Join(root, fmt.Sprintf(".inode_sink_%d", i))
					if err := os.Mkdir(sink, 0o755); err != nil {
						return err
					}
					mustMkdir(t, second)
					id, err := identityFromPath(second)
					if err != nil {
						return err
					}
					if id != oldID {
						changed = true
						break
					}
				}
				if !changed {
					return fmt.Errorf("could not recreate %s with a new identity", second)
				}
			}
			return permanentlyRemove(path)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed() {
		t.Fatal("expected identity failure")
	}
	ok := false
	for _, o := range res.Outcomes {
		if o.Path == second && o.Status == StatusFailed &&
			strings.Contains(o.Err.Error(), "identity changed") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("expected identity changed on second: %+v", res.Outcomes)
	}
	assertJournalStatuses(t, journal, map[string]bool{StatusFailed: true, StatusTrashed: true})
}

func TestRun_PartialFailureContinues(t *testing.T) {
	root := t.TempDir()
	journal := filepath.Join(t.TempDir(), "history.jsonl")
	good := filepath.Join(root, "good_modules")
	bad := filepath.Join(root, "bad_modules")
	mustMkdir(t, good)
	mustMkdir(t, bad)

	p := &plan.Plan{
		Root: root,
		Decisions: []plan.Decision{
			deleteDecision(root, "nodejs", good),
			deleteDecision(root, "nodejs", bad),
		},
	}
	res, err := Run(p, Options{
		Root:    root,
		Journal: journal,
		Warn:    io.Discard,
		ToTrash: true,
		Trash: func(path string, warn io.Writer) error {
			if path == bad {
				return errors.New("simulated trash failure")
			}
			return permanentlyRemove(path)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed() {
		t.Fatal("expected failed result")
	}
	if _, err := os.Lstat(good); !os.IsNotExist(err) {
		t.Fatal("good target should have been removed")
	}
	if _, err := os.Lstat(bad); err != nil {
		t.Fatal("bad target should remain after trash failure")
	}
	assertJournalStatuses(t, journal, map[string]bool{StatusFailed: true, StatusTrashed: true})
}

func TestRun_InterruptStopsBeforeNext(t *testing.T) {
	root := t.TempDir()
	journal := filepath.Join(t.TempDir(), "history.jsonl")
	a := filepath.Join(root, "z_a")
	b := filepath.Join(root, "a_b")
	mustMkdir(t, a)
	mustMkdir(t, b)

	ctx, cancel := context.WithCancel(context.Background())
	p := &plan.Plan{
		Root: root,
		Decisions: []plan.Decision{
			deleteDecision(root, "nodejs", a),
			deleteDecision(root, "nodejs", b),
		},
	}
	res, err := Run(p, Options{
		Root:    root,
		Journal: journal,
		Warn:    io.Discard,
		Context: ctx,
		ToTrash: true,
		Trash: func(path string, warn io.Writer) error {
			err := permanentlyRemove(path)
			cancel()
			return err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Interrupted || !res.Failed() {
		t.Fatalf("expected interrupted failure: %+v", res)
	}
	var skipped bool
	for _, o := range res.Outcomes {
		if o.Status == StatusSkipped {
			skipped = true
		}
	}
	if !skipped {
		t.Fatalf("expected skipped remaining target: %+v", res.Outcomes)
	}
}

func TestRun_SkipsNonDeleteVerdicts(t *testing.T) {
	root := t.TempDir()
	journal := filepath.Join(t.TempDir(), "history.jsonl")
	target := filepath.Join(root, "node_modules")
	mustMkdir(t, target)

	p := &plan.Plan{
		Root: root,
		Decisions: []plan.Decision{
			{
				Project: &detect.Project{Root: root, Framework: "nodejs"},
				Target:  detect.Target{Path: target, RelPath: "node_modules"},
				Verdict: plan.VerdictSkipped,
				Reason:  "not in .gitignore",
			},
		},
	}
	res, err := Run(p, Options{Root: root, Journal: journal, Warn: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if res.RemovedCount() != 0 || len(res.Outcomes) != 0 {
		t.Fatalf("should not touch skipped: %+v", res)
	}
	if _, err := os.Lstat(target); err != nil {
		t.Fatal("skipped target was removed")
	}
}

func deleteDecision(projectRoot, framework, target string) plan.Decision {
	return plan.Decision{
		Project: &detect.Project{Root: projectRoot, Framework: framework},
		Target: detect.Target{
			Path:    target,
			RelPath: filepath.Base(target),
			Kind:    detect.KindDir,
			Reason:  "test",
		},
		Verdict: plan.VerdictDelete,
		Reason:  "ignored by git",
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertJournal(t *testing.T, path string, n int, status string) {
	t.Helper()
	rec := readJournal(t, path)
	if len(rec.Outcomes) != n {
		t.Fatalf("journal outcomes=%d want %d: %+v", len(rec.Outcomes), n, rec.Outcomes)
	}
	for _, o := range rec.Outcomes {
		if o.Status != status {
			t.Fatalf("status=%s want %s", o.Status, status)
		}
	}
}

func assertJournalStatuses(t *testing.T, path string, want map[string]bool) {
	t.Helper()
	rec := readJournal(t, path)
	got := map[string]bool{}
	for _, o := range rec.Outcomes {
		got[o.Status] = true
	}
	for s := range want {
		if !got[s] {
			t.Fatalf("journal missing status %s: %+v", s, rec.Outcomes)
		}
	}
}

func readJournal(t *testing.T, path string) JournalRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 journal line, got %d", len(lines))
	}
	var rec JournalRecord
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}
