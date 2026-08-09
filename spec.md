# `reclaim` — Technical Specification

A Go CLI that walks a directory tree, identifies regenerable build artifacts and dependency caches belonging to real projects, and reclaims the disk space they occupy.

**Version:** 0.1
**Status:** implemented (v0.1)

---

## 1. Goals & Non-Goals

### Goals

- Recursively scan a root directory and find **regenerable** artifacts (`node_modules`, `target`, `.next`, `build`, `__pycache__`, …).
- Only ever act inside a **validated project** — a directory with a real manifest, not just a stray `node_modules`.
- Respect an explicit `.reclaim.yaml` control file, which always outranks built-in framework rules.
- Refuse to delete anything git considers meaningful (tracked or not-ignored), and say so loudly rather than silently.
- Be **modular**: adding support for a new framework should be one new file, no edits to core code — and ideally no Go at all.
- Be safe by default and boring in its failure modes.

### Non-Goals

- Not a general-purpose "disk cleaner". No system caches, no browser profiles, no `/tmp`.
- Not a `du` replacement, though it reports sizes so you can see what each target is worth.
- No daemon, no watch mode, no scheduling (v1).
- No deletion of anything that is not reproducible by a documented command.

---

## 2. Decided Behaviours

These were settled up front and drive the architecture:

| Decision | Behaviour |
|---|---|
| **Deletion mechanism** | Permanent removal by default. `--to-trash` routes through the OS trash/recycle bin instead. |
| **Nested projects** | The scanner keeps descending after a match. A monorepo's `packages/*/node_modules` are each matched and attributed to their own nearest project. Traversal is pruned *into* candidate targets only. |
| **Git-tracked or non-ignored artifacts** | Never deleted. Listed in a **Skipped** section with a concrete reason, so the user can fix their `.gitignore` or override explicitly. |

---

## 3. Core Concepts

```
Root ──walk──> Directory
                  │
                  ├── ControlFile (.reclaim.yaml)      ← explicit user intent
                  ├── Detector match  ─────────────>   Project
                  │                                      │
                  └── (descend)                          └── []Target
                                                             │
                                              Rules (git, safety, config)
                                                             │
                                                             ▼
                                                          Plan ──> Executor
```

### 3.1 Project

A directory that a detector has positively identified, with a manifest proving it is a real project.

```go
type Project struct {
    Root       string            // absolute path
    Framework  string            // "nodejs", "flutter", "rust"
    Confidence Confidence        // Strong | Weak
    Manifest   string            // path to the file that proved it
    Metadata   map[string]string // e.g. packageManager=pnpm
    GitRepo    *GitRepo          // nil if not under version control
    Parent     *Project          // nearest enclosing project, nil at top
}
```

### 3.2 Target

A single deletion candidate produced by a detector.

```go
type Target struct {
    Path       string     // absolute
    RelPath    string     // relative to Project.Root
    Kind       TargetKind // KindDir | KindFile | KindGlob
    Reason     string     // "Next.js build output"
    Regenerate string     // "npm run build" — shown in output, never executed
    Safety     Safety     // SafetyNormal | SafetyRequiresFlag
    Size       int64      // bytes, filled by the reporting pass; -1 if --no-size
    ModTime    time.Time  // mtime of the target itself (one stat, not a walk)
}
```

`SafetyRequiresFlag` marks targets that are cheap to delete but expensive or slow to regenerate (`ios/Pods`, `.terraform`, Unity `Library`). They are excluded unless `--aggressive` is passed, or explicitly listed in a `.reclaim.yaml` `delete:` block.

### 3.3 Decision

Every target ends up with exactly one decision, and every decision carries a human-readable reason. This is the single most important invariant for user trust.

```go
type Verdict int
const (
    VerdictDelete Verdict = iota
    VerdictSkipped        // matched, but a rule vetoed it
    VerdictKept           // .reclaim.yaml keep: entry
)
```

---

## 4. Scanning Algorithm

```
scan(root, opts):
  queue ← [(root, depth=0, inheritedControl=nil)]

  while queue not empty:
    dir, depth, inherited ← pop(queue)

    if depth > opts.MaxDepth:            continue
    if isSymlink(dir) and !opts.FollowSymlinks: continue
    if basename(dir) in HardPruneSet:    continue   // .git, .hg, .svn

    control ← loadControlFile(dir) ?? inherited
    if control != nil and control.Mode == Strict:
        targets ← control.ExplicitTargets(dir)
    else:
        match ← registry.DetectBest(dir)
        if match != nil:
            project ← newProject(dir, match, parent: nearestProject(dir))
            targets ← match.Targets
            if control != nil:
                targets ← control.Apply(targets)   // keep vetoes, delete adds

    for t in targets:
        decisions ← append(decisions, evaluate(t, project, control, opts))

    for child in readdir(dir):
        if child is a candidate target:   continue   // never walk into node_modules
        if !child.IsDir():                continue
        push(queue, (child, depth+1, control if control.Inherit else nil))
```

### 4.1 Depth

`--depth` (default `8`) counts directory levels below the root. It applies to the whole walk, not just the unmatched portion — descending into a monorepo's `packages/foo` consumes depth. `--depth 0` scans only the root itself.

### 4.2 Pruning

Never descended into:

- Any directory that is itself a delete candidate. Finding `node_modules` at depth 3 means nothing below it is examined, even a nested `node_modules/some-pkg/node_modules`. It is removed with its parent.
- `.git`, `.hg`, `.svn`, `.jj`
- Symlinked directories (unless `--follow-symlinks`, which additionally enforces a visited-inode set to break cycles)
- Mount points differing from the root's device, unless `--cross-device`

### 4.3 Nested project attribution

Because the walk continues past a match, a target is attributed to the **nearest enclosing project**. In a pnpm workspace:

```
repo/                      → Project(nodejs, pnpm workspace)
├── node_modules           → Target owned by repo/
├── .turbo                 → Target owned by repo/
└── packages/
    ├── api/               → Project(nodejs), Parent = repo/
    │   ├── node_modules   → Target owned by packages/api/
    │   └── dist           → Target owned by packages/api/
    └── web/               → Project(nextjs), Parent = repo/
        ├── node_modules   → Target owned by packages/web/
        └── .next          → Target owned by packages/web/
```

Output groups by project, so the user sees the shape of the repo rather than a flat list of 40 paths.

### 4.4 Concurrency

- Directory walk: bounded worker pool, default `min(8, NumCPU)`, tunable via `--concurrency`.
- Size computation is the expensive phase. It runs as a separate worker pool *after* the plan is built, and is purely for reporting — no decision depends on it, so a failure to size a target degrades to "unknown" rather than dropping it. `--no-size` skips the phase entirely, making the scan bounded by directory count rather than file count.
- Detector `Detect()` calls must be pure and side-effect free so they can run in parallel.

---

## 5. `.reclaim.yaml` — Control File

### 5.1 Schema

```yaml
version: 1

# merge (default) — framework detectors still run, this file adjusts the result
# strict          — detectors are ignored here; only `delete:` below is a candidate
mode: merge

# Does this file govern subdirectories too?
inherit: true

# Hard veto. Wins over everything, including built-in specs and `delete:` below.
keep:
  - node_modules
  - "**/.cache"
  - vendor/

# Additional candidates beyond what detectors found.
delete:
  - path: build/
    reason: "Custom CMake output"
    regenerate: "cmake --build build"
  - "tmp/**"           # shorthand: string form, reason auto-filled

# Constrain which detectors may run in this subtree.
frameworks:
  only: [nodejs, nextjs]     # allowlist; omit for "all"
  disable: [python]          # denylist, applied after `only`

# Override the git rule for this subtree only.
require_git_ignored: false   # default: true

# Do not touch anything under this subtree at all.
ignore: false
```

### 5.2 Precedence

Highest wins:

1. **Hard safety denylist** (§7.1) — not overridable by any config
2. `keep:` in the nearest `.reclaim.yaml`
3. `delete:` in the nearest `.reclaim.yaml`
4. `keep:` / `delete:` in an inherited ancestor `.reclaim.yaml`
5. Built-in framework spec targets
6. Global user config (`~/.config/reclaim/config.yaml`)

A `keep:` entry at any level beats a `delete:` entry at any lower-precedence level. Concretely: `node_modules` under `keep:` is never deleted, no matter what the Node detector says.

### 5.3 Pattern semantics

Patterns are gitignore-style, matched relative to the `.reclaim.yaml` directory:

- `node_modules` — matches at any depth in the subtree
- `/node_modules` — matches only at the control file's own level
- `build/` — trailing slash: directories only
- `**/*.log` — doublestar glob
- `!important/` — negation, subtracts from a preceding pattern in the same list

Implemented with `github.com/bmatcuk/doublestar/v4` plus a thin gitignore-anchoring layer.

### 5.4 `reclaim init`

Scaffolds a commented `.reclaim.yaml` in the current directory, pre-populated with whatever the detectors currently find, everything commented out. Zero-risk starting point.

---

## 6. Detector System (the modularity story)

Two tiers. Most frameworks need only tier 1.

### 6.1 Tier 1 — Declarative YAML specs

Adding a framework means dropping a YAML file into `internal/detect/specs/`. It is compiled in via `go:embed`, requiring no code changes anywhere. Users can also drop specs into `~/.config/reclaim/specs/` and get them loaded at runtime with no rebuild.

```yaml
name: nextjs
description: Next.js applications
priority: 20            # higher wins when several detectors match one dir
extends: nodejs         # inherit nodejs targets (node_modules) and detection

detect:
  all:
    - file_exists: package.json
    - any:
        - file_exists: next.config.js
        - file_exists: next.config.mjs
        - file_exists: next.config.ts
        - json_path:
            file: package.json
            path: dependencies.next
        - json_path:
            file: package.json
            path: devDependencies.next

targets:
  - path: .next
    reason: Next.js build cache and output
    regenerate: next build
  - path: out
    reason: Next.js static export output
    regenerate: next export
    when:                          # conditional target
      file_contains:
        file: next.config.js
        pattern: "output.*export"

metadata:
  packageManager:
    from_file_exists:
      pnpm-lock.yaml: pnpm
      yarn.lock: yarn
      bun.lockb: bun
      package-lock.json: npm
```

**Available predicates:**

| Predicate | Purpose |
|---|---|
| `file_exists` / `dir_exists` | Path presence |
| `glob_exists` | e.g. `*.csproj`, `*.xcodeproj` |
| `file_contains` | Regex against file contents (size-capped at 1 MiB) |
| `json_path` | Key presence/value in JSON (`dependencies.next`) |
| `yaml_path` / `toml_path` | Same for YAML/TOML manifests |
| `any` / `all` / `not` | Boolean composition |

The schema is validated at load time; a malformed user spec warns and is skipped rather than aborting the run.

### 6.2 Tier 2 — Go detectors

For logic YAML can't express (parsing a lockfile to locate a workspace root, reading Gradle settings, resolving a Cargo workspace's shared `target/`).

```go
package detect

type Detector interface {
    Name() string
    Priority() int
    Detect(ctx *Context, dir string) (*Match, error)
}

// Optional: implement to refine targets after all projects are known.
type PostProcessor interface {
    PostProcess(projects []*Project) error
}

type Context struct {
    FS      fs.StatFS      // injectable for tests
    Cache   *DirCache      // memoised readdir/stat/file-read per directory
    Config  *config.Config
}
```

Registration is `init()`-based:

```go
// internal/detect/builtin/rust.go
func init() { detect.Register(&RustDetector{}) }
```

`internal/detect/builtin/builtin.go` blank-imports the package so a single import in `main` pulls in everything. Adding a Go detector = one new file + one line in that import list.

### 6.3 Resolution when several detectors match

All matching detectors run. Their targets are **unioned**, deduplicated by path. `priority` only decides the reported `Framework` label and which metadata wins. A Next.js app in a pnpm monorepo correctly yields `node_modules` (nodejs), `.next` (nextjs), and `.turbo` (turborepo) together.

`extends:` composes specs — `nextjs extends nodejs` means the Node detection predicates must also pass, and Node's targets are included.

### 6.4 `reclaim detectors`

```
reclaim detectors list              # all registered, with source (builtin/user)
reclaim detectors show nextjs       # resolved spec after `extends` expansion
reclaim detectors test ./some/dir   # which detectors match here and why
```

`detectors test` prints the predicate tree with pass/fail per node — the primary debugging tool for anyone writing a spec.

---

## 7. Safety Rules

### 7.1 Hard denylist (not overridable by any config)

The run aborts, or the individual target is dropped, if:

- Target resolves to `/`, `~`, `~/Desktop`, `~/Documents`, `~/Downloads`, or any XDG base directory
- Target is or contains `.git`, `.hg`, `.svn`, `.jj`
- Target path, after `filepath.EvalSymlinks`, escapes the scan root
- Target is a symlink (deleted as a link only if it is itself a listed target; never traversed)
- Target basename matches the state denylist: `terraform.tfstate*`, `*.sqlite`, `*.db`, `.env*`, `*.pem`, `*.key`, `id_rsa*`
- Root is `/`, `$HOME`, or a filesystem root, without `--i-know-what-im-doing`

### 7.2 Project validation

A directory is never a project on the strength of an artifact alone. `node_modules` with no readable, parseable `package.json` in the same directory is **orphaned** and is always skipped with that reason — there is no flag to include it. Likewise the manifest must parse: a corrupt `package.json` yields `Confidence: Weak`, and weak matches never become deletion candidates. Both appear in the Skipped section so the situation is visible rather than silent, and both can be handled deliberately by listing the path under `delete:` in a `.reclaim.yaml`.

### 7.3 Git rules

For each project, walk up to find `.git`. Then, per target:

| Condition | Verdict |
|---|---|
| No git repo anywhere above | **Delete** — allowed |
| Path is git-ignored | **Delete** — allowed |
| Path is tracked by git (`git ls-files`) | **Skipped** — "tracked by git" |
| Path exists, untracked, not ignored | **Skipped** — "not in .gitignore" |
| Repo has uncommitted changes inside the target | **Skipped** — "uncommitted changes" |

Skipped items appear in a dedicated output section with the reason and a hint:

```
Skipped (3)
  ~/code/api/node_modules       not in .gitignore    → add `node_modules` to .gitignore
  ~/code/web/dist               tracked by git       → intentional? use `keep:` to silence
  ~/code/lib/target             uncommitted changes  → commit or stash first
```

Ignore matching uses `github.com/go-git/go-git/v5/plumbing/format/gitignore` — no `git` binary required, and it correctly layers `.gitignore`, nested `.gitignore` files, `.git/info/exclude`, and the global excludes file. `--use-git-binary` switches to shelling out to batched `git check-ignore --stdin -z` for exotic configurations.

There is no command-line override for this. A non-ignored artifact can only be deleted by naming it explicitly in a `.reclaim.yaml` `delete:` block, or by setting `require_git_ignored: false` for that subtree. Both are deliberate, reviewable, and committed alongside the project rather than typed once in a shell.

### 7.4 Deletion-time re-validation

Between plan and execute, the filesystem may change. Immediately before removing each target the executor re-checks: path still exists, is still the same inode, is still inside the root, is still not a symlink. Any mismatch aborts that target and is reported.

---

## 8. Built-in Framework Specs

Detection signal → what gets removed. All targets are subject to §7 rules.

### JavaScript / TypeScript

| Framework | Detection | Targets | Notes |
|---|---|---|---|
| `nodejs` | `package.json` (parses) | `node_modules` | Base spec for the rest |
| `nextjs` | `next.config.*` or `next` dep | `.next`, `out` | `out` only if static export configured |
| `nuxt` | `nuxt.config.*` or `nuxt` dep | `.nuxt`, `.output` | |
| `sveltekit` | `svelte.config.js` + `@sveltejs/kit` | `.svelte-kit` | |
| `astro` | `astro.config.*` | `.astro`, `dist` | |
| `vite` | `vite.config.*` | `dist`, `node_modules/.vite` | |
| `angular` | `angular.json` | `.angular/cache`, `dist` | |
| `gatsby` | `gatsby-config.*` | `.cache`, `public` | `public` is generated in Gatsby |
| `remix` | `remix.config.*` or `@remix-run/*` | `build`, `.cache` | |
| `turborepo` | `turbo.json` | `.turbo` | |
| `nx` | `nx.json` | `.nx/cache`, `dist` | |
| `parcel` | `parcel` dep | `.parcel-cache`, `dist` | |
| `electron` | `electron` dep or `electron-builder.yml` | `dist`, `out`, `release` | |
| `expo` | `app.json` w/ `expo` key | `.expo`, `.expo-shared` | |
| `react-native` | `react-native` dep + `metro.config.*` | `android/build`, `android/.gradle`, `ios/Pods`†, `ios/build` | † `SafetyRequiresFlag` |
| `storybook` | `.storybook/` | `storybook-static` | |
| `jest`/`vitest` | config file present | `coverage` | |

### Python

| Framework | Detection | Targets |
|---|---|---|
| `python` | `pyproject.toml`, `setup.py`, `setup.cfg`, or `requirements.txt` | `__pycache__` (recursive), `.pytest_cache`, `.mypy_cache`, `.ruff_cache`, `.ipynb_checkpoints`, `*.egg-info`, `build`, `dist` |
| `python-venv` | `pyvenv.cfg` in `.venv`/`venv`/`env` | `.venv`, `venv`, `env` † |
| `tox` | `tox.ini` | `.tox` |
| `poetry` | `[tool.poetry]` in `pyproject.toml` | metadata only |
| `uv` | `uv.lock` | metadata only |

† venvs are `SafetyRequiresFlag` — slow to rebuild and often contain locally-installed editable packages.

### Rust

| Detection | Targets | Notes |
|---|---|---|
| `Cargo.toml` (parses, has `[package]` or `[workspace]`) | `target` | Workspace-aware Go detector: a member crate's `target` is skipped in favour of the workspace root's. Usually the single largest win on a dev machine. |

### Go

| Detection | Targets | Notes |
|---|---|---|
| `go.mod` | `vendor` †, `bin`, `dist` | Build cache is global (`go clean -cache`), out of scope. `vendor` is `SafetyRequiresFlag` since some builds require it offline. |

### JVM

| Framework | Detection | Targets |
|---|---|---|
| `maven` | `pom.xml` | `target` |
| `gradle` | `build.gradle[.kts]` or `settings.gradle[.kts]` | `build`, `.gradle` |
| `android` | `build.gradle` + `AndroidManifest.xml` | `build`, `.gradle`, `.cxx`, `app/build`, `captures` |
| `sbt` | `build.sbt` | `target`, `project/target` |

### Dart / Flutter

| Detection | Targets |
|---|---|
| `pubspec.yaml` with `flutter:` key | `build`, `.dart_tool`, `.flutter-plugins`, `.flutter-plugins-dependencies`, `ios/Pods`†, `ios/.symlinks`, `android/.gradle`, `macos/Pods`† |
| `pubspec.yaml` without `flutter:` (plain Dart) | `.dart_tool`, `build` |

### Apple

| Framework | Detection | Targets |
|---|---|---|
| `swiftpm` | `Package.swift` | `.build`, `.swiftpm` |
| `cocoapods` | `Podfile` | `Pods` † |
| `carthage` | `Cartfile` | `Carthage/Build` † |
| `xcode` | `*.xcodeproj` / `*.xcworkspace` | `build`, `DerivedData` (project-local only) |

### .NET

| Detection | Targets |
|---|---|
| `*.csproj`, `*.fsproj`, `*.vbproj`, `*.sln` | `bin`, `obj`, `packages` |

### Ruby / PHP

| Framework | Detection | Targets |
|---|---|---|
| `bundler` | `Gemfile` | `vendor/bundle`, `.bundle`, `tmp/cache` |
| `rails` | `config/application.rb` | `tmp/cache`, `log`, `public/assets`, `public/packs`, `storage/*` (excl. DB) |
| `composer` | `composer.json` | `vendor` |
| `laravel` | `artisan` file | `bootstrap/cache`, `storage/framework/{cache,sessions,views}` |
| `symfony` | `symfony.lock` or `config/bundles.php` | `var/cache`, `var/log` |

### C / C++ / Systems

| Framework | Detection | Targets |
|---|---|---|
| `cmake` | `CMakeLists.txt` | `build`, `cmake-build-*`, `CMakeFiles`, `_build` |
| `meson` | `meson.build` | `build`, `builddir` |
| `bazel` | `WORKSPACE` or `MODULE.bazel` | `bazel-*` (symlinks — unlinked, never traversed) |
| `zig` | `build.zig` | `zig-out`, `zig-cache`, `.zig-cache` |
| `elixir` | `mix.exs` | `_build`, `deps` |
| `haskell` | `*.cabal` or `stack.yaml` | `dist-newstyle`, `.stack-work` |
| `nim` | `*.nimble` | `nimcache` |

### Static site generators

| Framework | Detection | Targets |
|---|---|---|
| `hugo` | `hugo.toml` / `config.toml` w/ hugo keys | `public`, `resources/_gen`, `.hugo_build.lock` |
| `jekyll` | `_config.yml` + `Gemfile` w/ jekyll | `_site`, `.jekyll-cache`, `.sass-cache` |
| `zola` | `config.toml` + `content/` | `public` |
| `eleventy` | `.eleventy.js` | `_site` |
| `mkdocs` | `mkdocs.yml` | `site` |

### Game engines & infra

| Framework | Detection | Targets |
|---|---|---|
| `unity` | `ProjectSettings/ProjectVersion.txt` | `Library` †, `Temp`, `Obj`, `Build`, `Logs`, `UserSettings` |
| `godot` | `project.godot` | `.godot`, `.import` |
| `unreal` | `*.uproject` | `Binaries`, `Intermediate`, `DerivedDataCache`, `Saved` † |
| `terraform` | `*.tf` | `.terraform` † — **never** `*.tfstate*` or `.terraform.lock.hcl` |
| `pulumi` | `Pulumi.yaml` | `.pulumi` † |
| `docs/latex` | `*.tex` | `*.aux`, `*.log`, `*.out`, `*.toc`, `_minted-*` |

Total: ~50 specs at v1, of which roughly 45 are pure declarative YAML.

---

## 9. CLI Surface

### Commands

```
reclaim [path]              # alias for `reclaim scan`
reclaim scan [path]         # scan, show plan, prompt for confirmation
reclaim plan [path]         # scan and print only — never prompts (= scan --dry-run)
reclaim init                # scaffold a .reclaim.yaml here
reclaim explain <path>      # why is (or isn't) this path a candidate?
reclaim detectors list|show|test
reclaim version
```

### Flags

**Scanning**

| Flag | Default | Purpose |
|---|---|---|
| `--depth`, `-d` | `8` | Max directory depth below root |
| `--concurrency` | `min(8,NCPU)` | Walker parallelism |
| `--follow-symlinks` | `false` | Traverse symlinked dirs (cycle-guarded) |
| `--cross-device` | `false` | Cross filesystem boundaries |
| `--framework`, `-f` | all | Restrict to named detectors (repeatable) |
| `--exclude-framework` | none | Denylist detectors (repeatable) |
| `--exclude` | none | Skip paths matching a glob (repeatable) |

**Execution**

| Flag | Default | Purpose |
|---|---|---|
| `--dry-run`, `-n` | `false` | Print the plan, exit 0, change nothing |
| `--yes`, `-y` | `false` | Skip the confirmation prompt |
| `--aggressive` | `false` | Include `SafetyRequiresFlag` targets |
| `--to-trash` | `false` | Send to OS trash instead of permanent delete |
| `--no-config` | `false` | Ignore all `.reclaim.yaml` and global config |

**Output**

| Flag | Default | Purpose |
|---|---|---|
| `--json` | `false` | Machine-readable plan/result to stdout |
| `--no-size` | `false` | Skip size computation (much faster) |
| `--quiet`, `-q` | `false` | Errors only |
| `--verbose`, `-v` | `false` | Per-decision reasoning (repeatable: `-vv`) |
| `--no-color` | auto | Also honours `NO_COLOR` |

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success, or dry-run completed, or nothing found |
| `1` | Unexpected error |
| `2` | Invalid usage / bad config |
| `3` | User declined at the confirmation prompt |
| `4` | Completed with partial failures (some targets could not be removed) |

---

## 10. Output Design

Default human output, grouped by project, sorted by reclaimable size descending (by project path when `--no-size` is set):

```
Scanning ~/code (depth 8)… 1,284 dirs, 37 projects, 0.4s
Sizing 24 targets… done, 0.5s

~/code/rust-service                                     rust
  target                          1.8 GB   62d   cargo build
                                  ─────────
                                   1.8 GB

~/code/web-app                                          nextjs · pnpm
  node_modules                    412 MB   14d   npm install
  .next                            88 MB    2d   next build
  .turbo                           12 MB    2d   turbo run build
                                  ─────────
                                   512 MB

~/code/web-app/packages/api                             nodejs
  node_modules                    203 MB   14d   npm install
                                  ─────────
                                   203 MB

Skipped (2)
  ~/code/legacy/node_modules      not in .gitignore   → add to .gitignore, or list under delete: in .reclaim.yaml
  ~/code/tool/dist                tracked by git      → intentional? add to .reclaim.yaml keep:

─────────────────────────────────────────────────────────
 12 projects · 24 targets · 4.7 GB reclaimable
 Deletion is permanent. Use --to-trash to recover later.

Proceed? [y/N]
```

- The confirmation prompt is a single yes/no over the whole plan. To act on a subset, narrow the scan (`--framework`, `--exclude`, a deeper root) or pin the exception in a `.reclaim.yaml` — both of which are repeatable, unlike a one-off selection.
- Sizes are reporting only; no decision depends on them. They are computed after the plan is built, with a progress line, and the prompt waits for completion. `--no-size` drops the size columns and totals and skips straight to the prompt.
- The age column is the target directory's own mtime — a single `stat`, not a recursive walk. Informational only; nothing filters on it.
- The plan is printed **before** the prompt, always, in full. No pagination that could hide a line.
- `--json` emits a stable schema: `{version, root, scannedAt, projects[], skipped[], totals{projects,targets,bytes}}`.

### `reclaim explain`

```
$ reclaim explain ~/code/legacy/node_modules

~/code/legacy/node_modules
  Project:    ~/code/legacy (nodejs, strong — package.json parsed)
  Detector:   nodejs → target `node_modules`
  Regenerate: npm install
  Git repo:   ~/code/legacy
  Ignored:    no  ← blocking
  Tracked:    no
  Control:    ~/code/.reclaim.yaml (inherited, mode=merge) — no matching rule

  Verdict: SKIPPED — not in .gitignore
```

---

## 11. Execution

### Ordering & mechanics

1. Sort targets deepest-first so nested removals never orphan a parent handle.
2. Re-validate each target (§7.4).
3. Remove:
   - **Permanent:** `os.RemoveAll`. On Windows, retry with attribute-clearing on `ERROR_ACCESS_DENIED` for read-only files.
   - **Trash (`--to-trash`):** platform-native. macOS: `NSFileManager trashItemAtURL` via a small cgo/`osascript` shim. Linux: XDG Trash spec (`~/.local/share/Trash/{files,info}`) with a `.trashinfo` file; fall back to permanent-with-warning if the target is on a different filesystem than the trash dir. Windows: `SHFileOperation` with `FOF_ALLOWUNDO`.
4. Failures are collected, not fatal. The run continues and reports at the end; exit code `4`.
5. `SIGINT` mid-run: stop before the next target, print what was already removed, exit `4`. Never leave a half-removed target un-reported.

### Journal

Every mutating run appends to `~/.local/state/reclaim/history.jsonl`: timestamp, root, flags, and every target with its project, framework, size, and outcome. `reclaim history` (v1.1) reads it. This is the audit trail for "what did I just do", and cheap to add now.

---

## 12. Package Layout

```
cmd/reclaim/
  main.go
internal/
  cli/                  cobra commands, flag wiring
  config/
    control.go          .reclaim.yaml parse + precedence
    global.go           ~/.config/reclaim/config.yaml
    pattern.go          gitignore-style matcher
  scan/
    walker.go           concurrent bounded walk, pruning
    cache.go            per-dir readdir/stat/read memoisation
  detect/
    registry.go         Register(), DetectBest(), ordering
    detector.go         Detector, Match, Target, Context
    spec.go             declarative YAML schema + predicate evaluator
    specs/*.yaml        ~45 embedded framework specs
    builtin/            Go detectors: rust.go, gradle.go, bazel.go, …
  rules/
    git.go              ignore + tracked + dirty checks
    safety.go           hard denylist, path containment
    filter.go           age/confidence/framework filters
  plan/
    plan.go             Decision, grouping, totals
    size.go             concurrent size walker (reporting only)
  exec/
    delete.go           permanent removal
    trash_darwin.go
    trash_linux.go
    trash_windows.go
    journal.go
  ui/
    render.go           table output, colour
    prompt.go           yes/no confirmation
    json.go
pkg/reclaim/            stable public API for out-of-tree detectors
testdata/
  fixtures/             generated project trees per framework
```

### Dependencies

| Library | Use |
|---|---|
| `spf13/cobra` | CLI |
| `go-git/go-git/v5/plumbing/format/gitignore` | ignore matching without the git binary |
| `bmatcuk/doublestar/v4` | glob patterns |
| `goccy/go-yaml` | YAML with good error positions |
| `BurntSushi/toml` | `Cargo.toml`, `pyproject.toml` |
| `charmbracelet/lipgloss` | output styling |
| `dustin/go-humanize` | sizes and relative times |

No TUI framework, and no terminal-control dependency at all — output is a printed table and the only input is a single yes/no. `reclaim` stays pipeable, CI-friendly, and readable in a scrollback buffer.

---

## 13. Testing

- **Fixture generator**: a `testdata` builder that materialises a realistic tree per framework (manifest + artifact + `.gitignore` + optional `git init`). Every spec ships with one.
- **Table-driven detector tests**: for each spec, assert positive match on its fixture, negative match on a decoy (artifact present, manifest absent), and exact target set.
- **Golden-file plan tests**: run the scanner over a composite fixture repo, compare the rendered plan against a checked-in golden file. Catches ordering, grouping, and wording regressions.
- **Safety tests are non-negotiable and run first**: symlink escape, `.git` deletion, path traversal via `..` in a control file, root/`$HOME` guard, tracked-file skip.
- **Executor tests** run against a temp dir with a fake trash backend; the permanent-delete path is exercised only inside `t.TempDir()`.
- Fuzz the control-file parser and the pattern matcher.

---

## 14. Build Order

1. Walker + safety rules + `--dry-run` with a single hardcoded Node detector. Nothing can be deleted yet.
2. Declarative spec engine + predicate evaluator + the first ten YAML specs.
3. Git rules and the Skipped section.
4. `.reclaim.yaml` parsing and the full precedence chain.
5. Size computation, output rendering, grouping, confirmation prompt.
6. Executor: permanent, then trash, then journal.
7. The remaining ~35 specs, plus Go detectors for Rust workspaces and Gradle.
8. `explain`, `init`, `detectors test`, `--json`.

Step 1 through 5 is a genuinely useful `reclaim plan` binary before a single byte can be deleted, which is the right order for a tool like this.

---

## 15. Open Questions for Later

- Should `reclaim` learn a project's package manager and offer to *reinstall* after deletion (`--restore`)? Probably a separate tool.
- Global caches (`~/.cargo/registry`, `~/.npm/_cacache`, `~/Library/Developer/Xcode/DerivedData`) are the other half of the disk-space problem but have completely different safety properties. Separate `reclaim global` command, v2.
- Remote/network filesystems: currently just slow. Detect and warn?
