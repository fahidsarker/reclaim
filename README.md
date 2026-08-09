# reclaim

A Go CLI that walks a directory tree, finds regenerable build artifacts and dependency caches belonging to real projects, and reclaims the disk space they occupy.

Behaviour detail: [spec.md](spec.md). Implementation history: [build-plan.md](build-plan.md). Releases: [CHANGELOG.md](CHANGELOG.md).

## Install

```sh
go install github.com/fahid/reclaim/cmd/reclaim@v0.1.0
```

Or build from a checkout (embeds the release version via ldflags):

```sh
make build
./bin/reclaim version   # 0.1.0
```

`go run` without ldflags prints `0.1.0-dev`.

## Quickstart

```sh
reclaim plan .          # scan and print candidates; never deletes
reclaim scan .          # print plan, then confirm before deleting
reclaim scan -y .       # skip confirmation (permanent delete by default)
reclaim scan -y --to-trash .
```

Narrow a run:

```sh
reclaim plan -f rust -f go ~/code
reclaim plan --exclude '**/vendor/**' .
reclaim scan --aggressive -y --to-trash .   # include expensive targets (Pods, venv, …)
```

## How it works

```
Root
  │ walk (scan)
  ▼
Directory ──► ControlFile (.reclaim.yaml)
         └──► Detector match ──► Project ──► Targets
                                              │
                         Rules (safety, git, config)
                                              ▼
                                           Plan ──► confirm ──► Executor
```

| Package | Role |
|---|---|
| `internal/scan` | Recursive walk, depth/concurrency, symlink and device bounds |
| `internal/detect` | Project + target discovery (YAML specs + Go detectors) |
| `internal/config` | `.reclaim.yaml` and global config parse / precedence |
| `internal/rules` | Hard safety denylist and git skip rules |
| `internal/plan` | Decisions, sizing, explain |
| `internal/ui` | Human plan, JSON, confirmation prompt |
| `internal/exec` | Permanent delete or OS trash, journal, partial failures |

**Detectors** come in two tiers:

1. **Tier 1 — YAML specs** embedded from [`internal/detect/specs/`](internal/detect/specs/). Drop a file there (or into `~/.config/reclaim/specs/`) — no Go changes required for most frameworks.
2. **Tier 2 — Go detectors** in [`internal/detect/builtin/`](internal/detect/builtin/) for logic YAML cannot express (e.g. Rust workspaces, Bazel symlink trees).

Full algorithm and types: [spec.md §3–4](spec.md).

## Commands

| Command | Purpose |
|---|---|
| `reclaim [path]` | Alias for `scan` |
| `reclaim scan [path]` | Scan, print plan, prompt, then delete |
| `reclaim plan [path]` | Scan and print only — never mutates (`scan --dry-run`) |
| `reclaim init` | Scaffold a commented `.reclaim.yaml` from what detectors find |
| `reclaim explain <path>` | Why a path is (or isn't) a deletion candidate |
| `reclaim detectors list\|show\|test` | Inspect and exercise detectors |
| `reclaim version` | Print version |

Default path is `.`.

## Flags

Shared by `scan` and `plan` (and the root alias).

### Scanning

| Flag | Default | Purpose |
|---|---|---|
| `--depth`, `-d` | `8` | Max directory depth below root |
| `--concurrency` | `min(8, NCPU)` | Walker parallelism |
| `--follow-symlinks` | `false` | Traverse symlinked dirs (cycle-guarded) |
| `--cross-device` | `false` | Cross filesystem boundaries |
| `--framework`, `-f` | all | Restrict to named detectors (repeatable) |
| `--exclude-framework` | none | Denylist detectors (repeatable) |
| `--exclude` | none | Skip paths matching a glob (repeatable) |
| `--i-know-what-im-doing` | `false` | Allow scanning `/` or `$HOME` |

### Execution

| Flag | Default | Purpose |
|---|---|---|
| `--dry-run`, `-n` | `false` | Print the plan and exit without deleting |
| `--yes`, `-y` | `false` | Skip the confirmation prompt |
| `--aggressive` | `false` | Include expensive targets (`Pods`, venvs, `.terraform`, …) |
| `--to-trash` | `false` | Send to OS trash instead of permanent delete |
| `--no-config` | `false` | Ignore all `.reclaim.yaml` and global config |

### Output

| Flag | Default | Purpose |
|---|---|---|
| `--json` | `false` | Machine-readable plan to stdout |
| `--no-size` | `false` | Skip size computation (faster) |
| `--quiet`, `-q` | `false` | Errors only |
| `--verbose`, `-v` | off | Per-decision reasoning (repeatable: `-vv`) |
| `--no-color` | auto | Disable colour; also honours `NO_COLOR` |
| `--use-git-binary` | `false` | Use `git check-ignore` instead of pure-Go ignore matching |

## Safety model

- Only acts inside **validated projects** (readable, parseable manifests). Orphan artifacts without a real project are skipped — there is no flag to include them.
- **Hard denylist** (not overridable): filesystem roots; `$HOME` / common user dirs without `--i-know-what-im-doing`; VCS dirs (`.git`, …); symlink escape from the scan root; sensitive basenames (`.env*`, keys, sqlite, terraform state, …).
- **Git rules**: tracked or non-ignored paths are never deleted; they appear under Skipped with a reason. Override only via a committed `.reclaim.yaml` (`delete:` or `require_git_ignored: false`).
- Weak / corrupt manifests never become deletion candidates (Skipped).
- Deletion is **permanent** by default. Use `--to-trash` for the OS trash/recycle bin.
- `reclaim plan` never mutates. `reclaim scan` always prints the full plan before prompting.
- Before each removal, the executor re-validates path identity (exists, same inode, still inside root, not a symlink escape).

Details: [spec.md §7](spec.md).

## Configuration

### `.reclaim.yaml`

Scaffold with `reclaim init`. Minimal example:

```yaml
version: 1
mode: merge          # merge (default) | strict
inherit: true

keep:
  - node_modules     # hard veto — wins over detectors and delete:

delete:
  - path: build/
    reason: "Custom CMake output"
    regenerate: "cmake --build build"
  - "tmp/**"         # shorthand string form

frameworks:
  only: [nodejs, nextjs]
  disable: [python]

require_git_ignored: true
ignore: false
```

**Precedence** (highest wins):

1. Hard safety denylist
2. `keep:` in the nearest `.reclaim.yaml`
3. `delete:` in the nearest `.reclaim.yaml`
4. Inherited ancestor `keep:` / `delete:`
5. Built-in detector targets
6. Global user config (`~/.config/reclaim/config.yaml`)

Patterns are gitignore-style, relative to the control file directory: `node_modules` (any depth), `/node_modules` (this level only), `build/` (dirs only), `**/*.log`, `!important/` (negation).

Full schema: [spec.md §5](spec.md).

### User detector specs

Drop extra YAML specs into `~/.config/reclaim/specs/` — loaded at runtime with no rebuild.

## Supported frameworks

~56 embedded YAML specs plus Tier-2 Go detectors (`rust`, `bazel`). List live names with `reclaim detectors list`. Targets marked † need `--aggressive` (or an explicit `delete:` entry).

| Ecosystem | Detectors (typical targets) |
|---|---|
| **JS / TS** | `nodejs` (`node_modules`); `nextjs` (`.next`, `out`); `nuxt`, `sveltekit`, `astro`, `vite`, `angular`, `gatsby`, `remix`, `turborepo`, `nx`, `parcel`, `electron`, `expo`, `react-native` († `ios/Pods`), `storybook`, `jest` |
| **Python** | `python` (`__pycache__`, caches, `build`/`dist`); `python-venv` († `.venv`/`venv`); `tox`, `poetry`, `uv` |
| **Rust** | `rust` (`target` — workspace-aware) |
| **Go** | `go` (`vendor` †, `bin`, `dist`) |
| **JVM** | `maven`, `gradle`, `android`, `sbt` |
| **Dart / Flutter** | `dart`, `flutter` († CocoaPods under iOS/macOS) |
| **Apple** | `swiftpm`, `cocoapods` († `Pods`), `carthage`, `xcode` |
| **.NET** | `dotnet` (`bin`, `obj`, `packages`) |
| **Ruby / PHP** | `bundler`, `rails`, `composer`, `laravel`, `symfony` |
| **C / systems** | `cmake`, `meson`, `bazel`, `zig`, `elixir`, `haskell`, `nim` |
| **Static sites** | `hugo`, `jekyll`, `zola`, `eleventy`, `mkdocs` |
| **Game / infra** | `unity` († `Library`), `godot`, `unreal`, `terraform` († `.terraform` — never state/lock), `pulumi`, `latex` |

Per-framework detection signals and full target lists: [spec.md §8](spec.md) and the YAML under [`internal/detect/specs/`](internal/detect/specs/).

## Output and exit codes

Human output groups reclaimable targets by project (largest first), then a **Skipped** section with reasons, then a confirmation prompt on `scan`. `--json` emits a stable plan schema on stdout; prompts and summaries go to stderr when JSON mode is on.

| Code | Meaning |
|---|---|
| `0` | Success, dry-run completed, or nothing found |
| `1` | Unexpected error |
| `2` | Invalid usage / bad config |
| `3` | User declined at the confirmation prompt |
| `4` | Completed with partial failures (some targets could not be removed) |

Mutating runs append an audit journal to `~/.local/state/reclaim/history.jsonl` (read UI deferred).

## Development

```sh
make test
make smoke-exec    # cross-compile trash shims for linux/darwin/windows
make fuzz          # short local fuzz of config parser + patterns
make build
go run ./cmd/reclaim version
```

CI runs tests (and short fuzz) on Linux/macOS/Windows via [`.github/workflows/ci.yml`](.github/workflows/ci.yml).
