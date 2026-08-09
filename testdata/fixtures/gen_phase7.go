package fixtures

import "path/filepath"

func writeGitignore(dir string, lines ...string) error {
	var b string
	for _, l := range lines {
		b += l + "\n"
	}
	return WriteFile(filepath.Join(dir, ".gitignore"), b)
}

func mkdirs(dir string, rels ...string) error {
	for _, r := range rels {
		if err := Mkdir(filepath.Join(dir, r)); err != nil {
			return err
		}
	}
	return nil
}

// Nuxt writes a Nuxt project.
func Nuxt(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"nuxt-app","dependencies":{"nuxt":"3.0.0"}}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "nuxt.config.ts"), `export default {}\n`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", ".nuxt", ".output"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", ".nuxt/", ".output/")
}

// SvelteKit writes a SvelteKit project.
func SvelteKit(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"sk","devDependencies":{"@sveltejs/kit":"2.0.0"}}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "svelte.config.js"), `export default {}\n`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", ".svelte-kit"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", ".svelte-kit/")
}

// Astro writes an Astro project.
func Astro(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"astro-app","dependencies":{"astro":"4.0.0"}}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "astro.config.mjs"), `export default {}\n`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", ".astro", "dist"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", ".astro/", "dist/")
}

// Angular writes an Angular project.
func Angular(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"ng"}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "angular.json"), `{"version":1}`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", ".angular/cache", "dist"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", ".angular/", "dist/")
}

// Gatsby writes a Gatsby project.
func Gatsby(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"gatsby-site","dependencies":{"gatsby":"5.0.0"}}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "gatsby-config.js"), `module.exports = {}\n`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", ".cache", "public"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", ".cache/", "public/")
}

// Remix writes a Remix project.
func Remix(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"remix-app","dependencies":{"@remix-run/node":"2.0.0"}}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "remix.config.js"), `module.exports = {}\n`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", "build", ".cache"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", "build/", ".cache/")
}

// Nx writes an Nx monorepo root.
func Nx(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"nx-mono"}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "nx.json"), `{}`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", ".nx/cache", "dist"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", ".nx/", "dist/")
}

// Parcel writes a Parcel project.
func Parcel(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"parcel-app","devDependencies":{"parcel":"2.0.0"}}`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", ".parcel-cache", "dist"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", ".parcel-cache/", "dist/")
}

// Electron writes an Electron project.
func Electron(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"electron-app","devDependencies":{"electron":"28.0.0"}}`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", "dist", "out", "release"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", "dist/", "out/", "release/")
}

// Expo writes an Expo project.
func Expo(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"expo-app"}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "app.json"), `{"expo":{"name":"app"}}`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", ".expo", ".expo-shared"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", ".expo/")
}

// ReactNative writes a React Native project.
func ReactNative(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"rn","dependencies":{"react-native":"0.73.0"}}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "metro.config.js"), `module.exports = {}\n`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", "android/build", "android/.gradle", "ios/Pods", "ios/build"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", "android/build/", "ios/build/")
}

// Storybook writes a Storybook project.
func Storybook(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"sb"}`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", ".storybook", "storybook-static"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", "storybook-static/")
}

// Jest writes a Jest project.
func Jest(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"jest-app","devDependencies":{"jest":"29.0.0"}}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "jest.config.js"), `module.exports = {}\n`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", "coverage"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", "coverage/")
}

// PythonVenv writes a Python venv project marker.
func PythonVenv(dir string) error {
	if err := WriteFile(filepath.Join(dir, ".venv", "pyvenv.cfg"), "home = /usr\n"); err != nil {
		return err
	}
	return writeGitignore(dir, ".venv/")
}

// Tox writes a tox project.
func Tox(dir string) error {
	if err := WriteFile(filepath.Join(dir, "tox.ini"), "[tox]\nenvlist = py312\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, ".tox"); err != nil {
		return err
	}
	return writeGitignore(dir, ".tox/")
}

// Poetry writes a Poetry project (metadata-only detector).
func Poetry(dir string) error {
	return WriteFile(filepath.Join(dir, "pyproject.toml"), "[tool.poetry]\nname = \"pkg\"\nversion = \"0.1.0\"\n")
}

// UV writes a uv lockfile project (metadata-only).
func UV(dir string) error {
	return WriteFile(filepath.Join(dir, "uv.lock"), "version = 1\n")
}

// Android writes an Android Gradle project.
func Android(dir string) error {
	if err := WriteFile(filepath.Join(dir, "build.gradle"), "plugins {}\n"); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "app", "src", "main", "AndroidManifest.xml"), `<manifest package="a.b"/>`); err != nil {
		return err
	}
	if err := mkdirs(dir, "build", ".gradle", ".cxx", "app/build", "captures"); err != nil {
		return err
	}
	return writeGitignore(dir, "build/", ".gradle/", "app/build/")
}

// SBT writes an sbt project.
func SBT(dir string) error {
	if err := WriteFile(filepath.Join(dir, "build.sbt"), `name := "app"`+"\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "target", "project/target"); err != nil {
		return err
	}
	return writeGitignore(dir, "target/", "project/target/")
}

// Dart writes a plain Dart project (no flutter key).
func Dart(dir string) error {
	if err := WriteFile(filepath.Join(dir, "pubspec.yaml"), "name: cli\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, ".dart_tool", "build"); err != nil {
		return err
	}
	return writeGitignore(dir, ".dart_tool/", "build/")
}

// SwiftPM writes a Swift package.
func SwiftPM(dir string) error {
	if err := WriteFile(filepath.Join(dir, "Package.swift"), "// swift-tools-version: 5.9\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, ".build", ".swiftpm"); err != nil {
		return err
	}
	return writeGitignore(dir, ".build/", ".swiftpm/")
}

// CocoaPods writes a CocoaPods project.
func CocoaPods(dir string) error {
	if err := WriteFile(filepath.Join(dir, "Podfile"), "platform :ios\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "Pods"); err != nil {
		return err
	}
	return writeGitignore(dir, "Pods/")
}

// Carthage writes a Carthage project.
func Carthage(dir string) error {
	if err := WriteFile(filepath.Join(dir, "Cartfile"), "github \"a/b\"\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "Carthage/Build"); err != nil {
		return err
	}
	return writeGitignore(dir, "Carthage/")
}

// Xcode writes an Xcode project.
func Xcode(dir string) error {
	if err := Mkdir(filepath.Join(dir, "App.xcodeproj")); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "App.xcodeproj", "project.pbxproj"), ""); err != nil {
		return err
	}
	if err := mkdirs(dir, "build", "DerivedData"); err != nil {
		return err
	}
	return writeGitignore(dir, "build/", "DerivedData/")
}

// Dotnet writes a .NET project.
func Dotnet(dir string) error {
	if err := WriteFile(filepath.Join(dir, "App.csproj"), `<Project Sdk="Microsoft.NET.Sdk"></Project>`); err != nil {
		return err
	}
	if err := mkdirs(dir, "bin", "obj", "packages"); err != nil {
		return err
	}
	return writeGitignore(dir, "bin/", "obj/")
}

// Bundler writes a Bundler project.
func Bundler(dir string) error {
	if err := WriteFile(filepath.Join(dir, "Gemfile"), `source "https://rubygems.org"`+"\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "vendor/bundle", ".bundle", "tmp/cache"); err != nil {
		return err
	}
	return writeGitignore(dir, "vendor/bundle/", ".bundle/", "tmp/")
}

// Rails writes a Rails app.
func Rails(dir string) error {
	if err := WriteFile(filepath.Join(dir, "Gemfile"), `gem "rails"`+"\n"); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "config", "application.rb"), "module App; class Application; end; end\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "tmp/cache", "log", "public/assets", "public/packs", "vendor/bundle", ".bundle"); err != nil {
		return err
	}
	return writeGitignore(dir, "tmp/", "log/", "public/assets/")
}

// Composer writes a Composer project.
func Composer(dir string) error {
	if err := WriteFile(filepath.Join(dir, "composer.json"), `{"name":"app/app"}`); err != nil {
		return err
	}
	if err := mkdirs(dir, "vendor"); err != nil {
		return err
	}
	return writeGitignore(dir, "vendor/")
}

// Laravel writes a Laravel app.
func Laravel(dir string) error {
	if err := WriteFile(filepath.Join(dir, "artisan"), "#!/usr/bin/env php\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "bootstrap/cache", "storage/framework/cache", "storage/framework/sessions", "storage/framework/views"); err != nil {
		return err
	}
	return writeGitignore(dir, "bootstrap/cache/", "storage/framework/")
}

// Symfony writes a Symfony app.
func Symfony(dir string) error {
	if err := WriteFile(filepath.Join(dir, "symfony.lock"), "{}\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "var/cache", "var/log"); err != nil {
		return err
	}
	return writeGitignore(dir, "var/")
}

// CMake writes a CMake project.
func CMake(dir string) error {
	if err := WriteFile(filepath.Join(dir, "CMakeLists.txt"), "cmake_minimum_required(VERSION 3.20)\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "build", "cmake-build-debug", "CMakeFiles", "_build"); err != nil {
		return err
	}
	return writeGitignore(dir, "build/", "_build/")
}

// Meson writes a Meson project.
func Meson(dir string) error {
	if err := WriteFile(filepath.Join(dir, "meson.build"), "project('app')\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "build", "builddir"); err != nil {
		return err
	}
	return writeGitignore(dir, "build/", "builddir/")
}

// Bazel writes a Bazel workspace with output symlinks.
func Bazel(dir string) error {
	if err := WriteFile(filepath.Join(dir, "MODULE.bazel"), `module(name = "app")`+"\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "bazel-bin", "bazel-out"); err != nil {
		return err
	}
	return writeGitignore(dir, "bazel-*")
}

// Zig writes a Zig project.
func Zig(dir string) error {
	if err := WriteFile(filepath.Join(dir, "build.zig"), "const std = @import(\"std\");\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "zig-out", "zig-cache", ".zig-cache"); err != nil {
		return err
	}
	return writeGitignore(dir, "zig-out/", "zig-cache/", ".zig-cache/")
}

// Elixir writes a Mix project.
func Elixir(dir string) error {
	if err := WriteFile(filepath.Join(dir, "mix.exs"), "defmodule App.MixProject do\nend\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "_build", "deps"); err != nil {
		return err
	}
	return writeGitignore(dir, "_build/", "deps/")
}

// Haskell writes a Cabal project.
func Haskell(dir string) error {
	if err := WriteFile(filepath.Join(dir, "app.cabal"), "name: app\nversion: 0.1.0\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "dist-newstyle", ".stack-work"); err != nil {
		return err
	}
	return writeGitignore(dir, "dist-newstyle/", ".stack-work/")
}

// Nim writes a Nimble project.
func Nim(dir string) error {
	if err := WriteFile(filepath.Join(dir, "app.nimble"), "version = \"0.1.0\"\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "nimcache"); err != nil {
		return err
	}
	return writeGitignore(dir, "nimcache/")
}

// Hugo writes a Hugo site.
func Hugo(dir string) error {
	if err := WriteFile(filepath.Join(dir, "hugo.toml"), "baseURL = 'https://example.org/'\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "public", "resources/_gen"); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, ".hugo_build.lock"), ""); err != nil {
		return err
	}
	return writeGitignore(dir, "public/", "resources/")
}

// Jekyll writes a Jekyll site.
func Jekyll(dir string) error {
	if err := WriteFile(filepath.Join(dir, "_config.yml"), "title: site\n"); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, "Gemfile"), `gem "jekyll"`+"\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "_site", ".jekyll-cache", ".sass-cache", "vendor/bundle", ".bundle", "tmp/cache"); err != nil {
		return err
	}
	return writeGitignore(dir, "_site/", ".jekyll-cache/")
}

// Zola writes a Zola site.
func Zola(dir string) error {
	if err := WriteFile(filepath.Join(dir, "config.toml"), "base_url = \"https://example.org\"\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "content", "public"); err != nil {
		return err
	}
	return writeGitignore(dir, "public/")
}

// Eleventy writes an Eleventy site.
func Eleventy(dir string) error {
	if err := WriteFile(filepath.Join(dir, "package.json"), `{"name":"11ty"}`); err != nil {
		return err
	}
	if err := WriteFile(filepath.Join(dir, ".eleventy.js"), `module.exports = {}\n`); err != nil {
		return err
	}
	if err := mkdirs(dir, "node_modules", "_site"); err != nil {
		return err
	}
	return writeGitignore(dir, "node_modules/", "_site/")
}

// MkDocs writes an MkDocs site.
func MkDocs(dir string) error {
	if err := WriteFile(filepath.Join(dir, "mkdocs.yml"), "site_name: docs\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "site"); err != nil {
		return err
	}
	return writeGitignore(dir, "site/")
}

// Unity writes a Unity project.
func Unity(dir string) error {
	if err := WriteFile(filepath.Join(dir, "ProjectSettings", "ProjectVersion.txt"), "m_EditorVersion: 2022.3\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "Library", "Temp", "Obj", "Build", "Logs", "UserSettings"); err != nil {
		return err
	}
	return writeGitignore(dir, "Library/", "Temp/", "Obj/", "Build/", "Logs/")
}

// Godot writes a Godot project.
func Godot(dir string) error {
	if err := WriteFile(filepath.Join(dir, "project.godot"), "; Engine configuration file\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, ".godot", ".import"); err != nil {
		return err
	}
	return writeGitignore(dir, ".godot/", ".import/")
}

// Unreal writes an Unreal project.
func Unreal(dir string) error {
	if err := WriteFile(filepath.Join(dir, "MyGame.uproject"), `{"FileVersion":3}`); err != nil {
		return err
	}
	if err := mkdirs(dir, "Binaries", "Intermediate", "DerivedDataCache", "Saved"); err != nil {
		return err
	}
	return writeGitignore(dir, "Binaries/", "Intermediate/", "Saved/")
}

// Terraform writes a Terraform project.
func Terraform(dir string) error {
	if err := WriteFile(filepath.Join(dir, "main.tf"), `terraform {}`+"\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, ".terraform"); err != nil {
		return err
	}
	return writeGitignore(dir, ".terraform/")
}

// Pulumi writes a Pulumi project.
func Pulumi(dir string) error {
	if err := WriteFile(filepath.Join(dir, "Pulumi.yaml"), "name: app\nruntime: go\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, ".pulumi"); err != nil {
		return err
	}
	return writeGitignore(dir, ".pulumi/")
}

// Latex writes a LaTeX document tree.
func Latex(dir string) error {
	if err := WriteFile(filepath.Join(dir, "main.tex"), `\documentclass{article}\begin{document}\end{document}`+"\n"); err != nil {
		return err
	}
	for _, f := range []string{"main.aux", "main.log", "main.out", "main.toc"} {
		if err := WriteFile(filepath.Join(dir, f), ""); err != nil {
			return err
		}
	}
	if err := mkdirs(dir, "_minted-main"); err != nil {
		return err
	}
	return writeGitignore(dir, "*.aux", "*.log", "*.out", "*.toc", "_minted-*")
}

// RustWorkspace writes a Cargo workspace with a member crate, both with target/.
func RustWorkspace(dir string) error {
	if err := WriteFile(filepath.Join(dir, "Cargo.toml"), "[workspace]\nmembers = [\"crates/svc\"]\n"); err != nil {
		return err
	}
	if err := mkdirs(dir, "target"); err != nil {
		return err
	}
	member := filepath.Join(dir, "crates", "svc")
	if err := WriteFile(filepath.Join(member, "Cargo.toml"), "[package]\nname = \"svc\"\nversion = \"0.1.0\"\n"); err != nil {
		return err
	}
	if err := mkdirs(member, "target"); err != nil {
		return err
	}
	return writeGitignore(dir, "target/")
}
