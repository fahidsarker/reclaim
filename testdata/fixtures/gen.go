// Package fixtures builds realistic project trees for detector and plan tests.
package fixtures

import (
	"os"
	"path/filepath"
)

// WriteFile writes content to path, creating parent directories.
func WriteFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// Mkdir creates a directory.
func Mkdir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// NodeJS writes a Node project with package.json, node_modules, and .gitignore.
func NodeJS(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"app"}`); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "node_modules")); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), "node_modules/\n")
}

// NextJS writes a Next.js project (extends nodejs targets).
func NextJS(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"web","dependencies":{"next":"14.0.0"}}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "next.config.js"), `module.exports = {}\n`); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "node_modules")); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, ".next")); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), "node_modules/\n.next/\n")
}

// Vite writes a Vite project.
func Vite(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"vite-app"}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "vite.config.ts"), `export default {}\n`); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "dist")); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "node_modules", ".vite")); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), "dist/\nnode_modules/\n")
}

// Turborepo writes a turbo.json project.
func Turborepo(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"mono"}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "turbo.json"), `{"pipeline":{}}`); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, ".turbo")); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), ".turbo/\n")
}

// Python writes a Python project with common caches.
func Python(dir string) error {
	if err := WriteFile(filepath.Join(dir, "pyproject.toml"), "[project]\nname = \"pkg\"\n"); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "src", "pkg", "__pycache__")); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, ".pytest_cache")); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "build")); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "dist")); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "pkg.egg-info")); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), "__pycache__/\n.pytest_cache/\nbuild/\ndist/\n*.egg-info/\n")
}

// Rust writes a Cargo project.
func Rust(dir string) error {
	if err := WriteFile(filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"svc\"\nversion = \"0.1.0\"\n"); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "target")); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), "target/\n")
}

// Go writes a Go module with vendor/bin/dist.
func Go(dir string) error {
	if err := WriteFile(filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n"); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "vendor")); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "bin")); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "dist")); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), "bin/\ndist/\n")
}

// Maven writes a Maven project.
func Maven(dir string) error {
	if err := WriteFile(filepath.Join(dir, "pom.xml"), `<project><modelVersion>4.0.0</modelVersion></project>`); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "target")); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), "target/\n")
}

// Gradle writes a Gradle project.
func Gradle(dir string) error {
	if err := WriteFile(filepath.Join(dir, "build.gradle"), "plugins {}\n"); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, "build")); err != nil {
		return err
	}
	if err := Mkdir(filepath.Join(dir, ".gradle")); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), "build/\n.gradle/\n")
}

// Flutter writes a Flutter app with Pods.
func Flutter(dir string) error {
	if err := WriteFile(filepath.Join(dir, "pubspec.yaml"), "name: app\nflutter:\n  uses-material-design: true\n"); err != nil {
		return err
	}
	for _, p := range []string{
		"build",
		".dart_tool",
		"ios/Pods",
		"ios/.symlinks",
		"android/.gradle",
		"macos/Pods",
	} {
		if err := Mkdir(filepath.Join(dir, p)); err != nil {
			return err
		}
	}
	if err := WriteFile(filepath.Join(dir, ".flutter-plugins"), ""); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, ".flutter-plugins-dependencies"), "{}"); err != nil {
		return err
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), "build/\n.dart_tool/\n")
}

// Decoy writes only the given artifact directories with no manifest.
func Decoy(dir string, artifacts ...string) error {
	for _, a := range artifacts {
		if err := Mkdir(filepath.Join(dir, a)); err != nil {
			return err
		}
	}
	return nil
}
