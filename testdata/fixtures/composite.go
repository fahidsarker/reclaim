package fixtures

import (
	"path/filepath"
)

// Composite builds a multi-framework tree for golden plan tests:
//
//	root/
//	  web/          nextjs (ignored artifacts)
//	  api/          nodejs (ignored)
//	  rust-svc/     rust (ignored)
//	  py/           python (ignored)
//	  legacy/       nodejs WITHOUT .gitignore → skipped by git
func Composite(root string) error {
	if err := NextJS(filepath.Join(root, "web")); err != nil {
		return err
	}
	if err := NodeJS(filepath.Join(root, "api")); err != nil {
		return err
	}
	if err := Rust(filepath.Join(root, "rust-svc")); err != nil {
		return err
	}
	if err := Python(filepath.Join(root, "py")); err != nil {
		return err
	}
	// legacy: real project + node_modules, but no ignore → git skip
	legacy := filepath.Join(root, "legacy")
	if err := WriteFile(filepath.Join(legacy, "package.json"), `{"name":"legacy"}`); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(legacy, "node_modules")); err != nil {
		return err
	}
	// Root git repo: commit manifests only (not artifacts). legacy/node_modules stays untracked+unignored.
	return GitInit(root,
		"web/package.json",
		"web/next.config.js",
		"web/.gitignore",
		"api/package.json",
		"api/.gitignore",
		"rust-svc/Cargo.toml",
		"rust-svc/.gitignore",
		"py/pyproject.toml",
		"py/.gitignore",
		"legacy/package.json",
	)
}
