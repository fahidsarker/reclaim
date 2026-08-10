package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/fahid/reclaim/internal/detect"
	_ "github.com/fahid/reclaim/internal/detect/builtin"
	"github.com/fahid/reclaim/testdata/fixtures"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestPlan_CompositeGolden(t *testing.T) {
	detect.MustLoadEmbedded()
	root := t.TempDir()
	if err := fixtures.Composite(root); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"plan", "--no-size", "--no-color", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got := normalizeGolden(buf.String(), root)
	goldenPath := filepath.Join("..", "..", "testdata", "golden", "composite_plan.txt")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}
	want := normalizeLineEndings(string(wantBytes))
	if got != want {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	// JSON golden shape (normalized root).
	buf.Reset()
	cmd = newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"plan", "--json", "--no-size", "--no-color", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	gotJSON := normalizeGolden(buf.String(), root)
	// Drop timestamps that vary across runs.
	gotJSON = regexp.MustCompile(`"scannedAt": "[^"]*"`).ReplaceAllString(gotJSON, `"scannedAt": "GOLDEN"`)
	gotJSON = regexp.MustCompile(`"modTime": "[^"]*"`).ReplaceAllString(gotJSON, `"modTime": "GOLDEN"`)

	jsonPath := filepath.Join("..", "..", "testdata", "golden", "composite_plan.json")
	if *updateGolden {
		if err := os.WriteFile(jsonPath, []byte(gotJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wantJSONBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json golden (run with -update): %v", err)
	}
	if gotJSON != normalizeLineEndings(string(wantJSONBytes)) {
		t.Fatalf("json golden mismatch\n--- got ---\n%s\n--- want ---\n%s", gotJSON, wantJSONBytes)
	}
}

func TestNormalizeGolden_WindowsJSONPaths(t *testing.T) {
	// Simulate json.Marshal output on Windows: each `\` is escaped as `\\`.
	root := `C:\Users\runner\AppData\Local\Temp\reclaim123`
	in := `{
  "root": "C:\\Users\\runner\\AppData\\Local\\Temp\\reclaim123",
  "projects": [
    {
      "root": "C:\\Users\\runner\\AppData\\Local\\Temp\\reclaim123\\api",
      "targets": [
        {
          "path": "C:\\Users\\runner\\AppData\\Local\\Temp\\reclaim123\\py\\src\\pkg\\__pycache__",
          "relPath": "src\\pkg\\__pycache__"
        }
      ]
    }
  ],
  "skipped": [
    {
      "path": "C:\\Users\\runner\\AppData\\Local\\Temp\\reclaim123\\legacy\\node_modules"
    }
  ]
}`
	got := normalizeGolden(in, root)
	want := `{
  "root": "/FIXTURE",
  "projects": [
    {
      "root": "/FIXTURE/api",
      "targets": [
        {
          "path": "/FIXTURE/py/src/pkg/__pycache__",
          "relPath": "src/pkg/__pycache__"
        }
      ]
    }
  ],
  "skipped": [
    {
      "path": "/FIXTURE/legacy/node_modules"
    }
  ]
}`
	if got != want {
		t.Fatalf("normalizeGolden Windows JSON paths\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func normalizeGolden(s, root string) string {
	// Windows checkouts may rewrite golden files to CRLF (core.autocrlf).
	s = normalizeLineEndings(s)
	// JSON encodes Windows `\` as `\\`; rewrite that form first.
	s = strings.ReplaceAll(s, strings.ReplaceAll(root, `\`, `\\`), "/FIXTURE")
	s = strings.ReplaceAll(s, root, "/FIXTURE")
	// Slash-normalize remaining separators. Replace JSON-escaped `\\` before
	// single `\`, otherwise each backslash becomes `/` and paths get `//`.
	s = strings.ReplaceAll(s, `\\`, `/`)
	s = strings.ReplaceAll(s, `\`, `/`)
	// Duration lines vary slightly; normalize timing.
	reDur := regexp.MustCompile(`\d+(\.\d+)?(ms|s|µs|us|m)`)
	s = reDur.ReplaceAllString(s, "TIME")
	// Project headers use %-56s on absolute temp paths; after /FIXTURE rewrite,
	// residual padding depends on TempDir length. Collapse to a stable gap.
	reHeader := regexp.MustCompile(`(?m)^(/FIXTURE/\S+)\s{2,}(.+)$`)
	s = reHeader.ReplaceAllString(s, "$1  $2")
	return s
}
