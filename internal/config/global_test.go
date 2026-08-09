package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGlobalFile_MissingEmpty(t *testing.T) {
	base := t.TempDir()
	g, err := LoadGlobalFile(filepath.Join(base, "nope.yaml"), base)
	if err != nil {
		t.Fatal(err)
	}
	if g.MatchKeep(filepath.Join(base, "node_modules"), true) {
		t.Fatal("empty global should not match")
	}
}

func TestParseGlobal_KeepDelete(t *testing.T) {
	base := t.TempDir()
	g, err := ParseGlobal(base, []byte(`
keep:
  - vendor/
delete:
  - custom_cache
`))
	if err != nil {
		t.Fatal(err)
	}
	if !g.MatchKeep(filepath.Join(base, "pkg", "vendor"), true) {
		t.Fatal("expected global keep match")
	}
	e, ok := g.MatchDelete(filepath.Join(base, "custom_cache"), true)
	if !ok || e.Reason != "listed in global config" {
		t.Fatalf("delete: ok=%v entry=%+v", ok, e)
	}
}

func TestChain_GlobalLowestPrecedence(t *testing.T) {
	root := t.TempDir()
	g, err := ParseGlobal(root, []byte("keep:\n  - node_modules\n"))
	if err != nil {
		t.Fatal(err)
	}
	local, err := ParseControl(root, []byte("delete:\n  - node_modules\n"))
	if err != nil {
		t.Fatal(err)
	}
	ch := (&Chain{Global: g}).WithLocal(local)
	nm := filepath.Join(root, "node_modules")
	keep, del := ch.Classify(nm, true)
	if keep || del == nil {
		t.Fatalf("nearest delete should beat global keep: keep=%v del=%v", keep, del)
	}
}

func TestLoadControl_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".reclaim.yaml")
	if err := os.WriteFile(path, []byte("mode: merge\nkeep:\n  - dist\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadControl(dir)
	if err != nil || c == nil {
		t.Fatalf("c=%v err=%v", c, err)
	}
	if !c.MatchKeep(filepath.Join(dir, "dist"), true) {
		t.Fatal("expected keep match")
	}
}
