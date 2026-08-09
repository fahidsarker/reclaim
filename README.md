# reclaim

A Go CLI that walks a directory tree, finds regenerable build artifacts and dependency caches belonging to real projects, and reclaims the disk space they occupy.

See [spec.md](spec.md) for behaviour and [build-plan.md](build-plan.md) for the incremental implementation plan.

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

Other useful commands: `reclaim explain <path>`, `reclaim init`, `reclaim detectors list|show|test`.

## Safety model

- Only acts inside **validated projects** (real manifests). Orphan artifacts without a parseable manifest are skipped.
- **Hard denylist** (not overridable): filesystem roots, `$HOME` / common user dirs without `--i-know-what-im-doing`, VCS dirs, symlink escape from the scan root, sensitive basenames (`.env*`, keys, sqlite, terraform state, …).
- **Git rules**: tracked or non-ignored paths are never deleted; they appear in Skipped with a reason. Override only via a committed `.reclaim.yaml` (`delete:` or `require_git_ignored: false`).
- Deletion is **permanent** by default. Use `--to-trash` for the OS trash/recycle bin.
- `reclaim plan` never mutates. `reclaim scan` prints the full plan before prompting.

## Out of scope (v0.1)

Deferred intentionally (see [spec.md §15](spec.md)):

- No `--restore` / post-delete reinstall
- No `reclaim global` for tool caches (`~/.cargo/registry`, npm cacache, Xcode DerivedData, …)
- Network / remote filesystems are not a special case — they may simply be slow

## Development

```sh
make test
make smoke-exec    # cross-compile trash shims for linux/darwin/windows
make fuzz          # short local fuzz of config parser + patterns
go run ./cmd/reclaim version
```
