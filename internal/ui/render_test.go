package ui_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/fahid/reclaim/internal/detect"
	"github.com/fahid/reclaim/internal/plan"
	"github.com/fahid/reclaim/internal/rules"
	"github.com/fahid/reclaim/internal/ui"
)

func TestRender_GoldenGrouping(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	p := &plan.Plan{
		Root: "/tmp/code",
		Stats: plan.Stats{
			DirsWalked:   12,
			Projects:     2,
			Depth:        8,
			ScanDuration: 400 * time.Millisecond,
			SizeDuration: 500 * time.Millisecond,
		},
		Decisions: []plan.Decision{
			{
				Project: &detect.Project{
					Root:      "/tmp/code/web",
					Framework: "nextjs",
					Metadata:  map[string]string{"packageManager": "pnpm"},
				},
				Target: detect.Target{
					Path:       "/tmp/code/web/node_modules",
					RelPath:    "node_modules",
					Regenerate: "npm install",
					Size:       412 * 1024 * 1024,
					ModTime:    now.Add(-14 * 24 * time.Hour),
				},
				Verdict: plan.VerdictDelete,
				Reason:  "Node.js dependencies",
			},
			{
				Project: &detect.Project{
					Root:      "/tmp/code/web",
					Framework: "nextjs",
					Metadata:  map[string]string{"packageManager": "pnpm"},
				},
				Target: detect.Target{
					Path:       "/tmp/code/web/.next",
					RelPath:    ".next",
					Regenerate: "next build",
					Size:       88 * 1024 * 1024,
					ModTime:    now.Add(-2 * 24 * time.Hour),
				},
				Verdict: plan.VerdictDelete,
				Reason:  "Next.js build cache",
			},
			{
				Project: &detect.Project{Root: "/tmp/code/api", Framework: "nodejs"},
				Target: detect.Target{
					Path:       "/tmp/code/api/node_modules",
					RelPath:    "node_modules",
					Regenerate: "npm install",
					Size:       203 * 1024 * 1024,
					ModTime:    now.Add(-14 * 24 * time.Hour),
				},
				Verdict: plan.VerdictDelete,
			},
			{
				Target: detect.Target{
					Path: "/tmp/code/legacy/node_modules",
				},
				Verdict: plan.VerdictSkipped,
				Reason:  rules.ReasonNotIgnored,
			},
		},
	}

	// Freeze "now" for age by using ModTime relative to wall clock — ages will vary.
	// Assert structural properties instead of exact age strings.
	var buf bytes.Buffer
	color := false
	if err := ui.Render(&buf, p, ui.RenderOptions{NoColor: true, Color: &color}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "Scanning /tmp/code (depth 8)…") {
		t.Fatalf("missing scan header:\n%s", out)
	}
	if !strings.Contains(out, "Sizing 3 targets… done") {
		t.Fatalf("missing size header:\n%s", out)
	}
	// Larger project (web ~500MB) before smaller (api ~203MB)
	webIdx := strings.Index(out, "/tmp/code/web")
	apiIdx := strings.Index(out, "/tmp/code/api")
	if webIdx < 0 || apiIdx < 0 || webIdx > apiIdx {
		t.Fatalf("want web before api by size:\n%s", out)
	}
	if !strings.Contains(out, "nextjs · pnpm") {
		t.Fatalf("missing framework/meta:\n%s", out)
	}
	if !strings.Contains(out, "node_modules") || !strings.Contains(out, ".next") {
		t.Fatalf("missing targets:\n%s", out)
	}
	if !strings.Contains(out, "Skipped (1)") {
		t.Fatalf("missing skipped:\n%s", out)
	}
	if !strings.Contains(out, "→ add `node_modules` to .gitignore") {
		t.Fatalf("missing hint:\n%s", out)
	}
	if !strings.Contains(out, "reclaimable") {
		t.Fatalf("missing totals:\n%s", out)
	}
	if !strings.Contains(out, "Deletion is permanent") {
		t.Fatalf("missing warning:\n%s", out)
	}
}

func TestRender_NoSizeOmitsSizeColumns(t *testing.T) {
	p := &plan.Plan{
		Root: "/tmp/code",
		Stats: plan.Stats{
			DirsWalked: 3,
			Projects:   2,
			Depth:      4,
		},
		Decisions: []plan.Decision{
			{
				Project: &detect.Project{Root: "/tmp/code/b", Framework: "nodejs"},
				Target: detect.Target{
					Path:       "/tmp/code/b/node_modules",
					RelPath:    "node_modules",
					Regenerate: "npm install",
					Size:       plan.SizeSkipped,
				},
				Verdict: plan.VerdictDelete,
			},
			{
				Project: &detect.Project{Root: "/tmp/code/a", Framework: "nodejs"},
				Target: detect.Target{
					Path:       "/tmp/code/a/node_modules",
					RelPath:    "node_modules",
					Regenerate: "npm install",
					Size:       plan.SizeSkipped,
				},
				Verdict: plan.VerdictDelete,
			},
		},
	}
	var buf bytes.Buffer
	color := false
	if err := ui.Render(&buf, p, ui.RenderOptions{NoSize: true, NoColor: true, Color: &color}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "Sizing") {
		t.Fatalf("should omit sizing line:\n%s", out)
	}
	if strings.Contains(out, "reclaimable") {
		t.Fatalf("should omit byte totals:\n%s", out)
	}
	if strings.Contains(out, "MB") || strings.Contains(out, "GB") || strings.Contains(out, "kB") {
		t.Fatalf("should omit size columns:\n%s", out)
	}
	// Path order when --no-size: /tmp/code/a before /tmp/code/b
	aIdx := strings.Index(out, "/tmp/code/a")
	bIdx := strings.Index(out, "/tmp/code/b")
	if aIdx < 0 || bIdx < 0 || aIdx > bIdx {
		t.Fatalf("want path-sorted projects:\n%s", out)
	}
}
