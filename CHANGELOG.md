# Changelog

## [0.1.0] — 2026-08-09

First release of `reclaim`: scan validated projects for regenerable artifacts, plan reclaimable targets, and delete (or trash) them safely.

### Highlights

- Recursive scan with depth/concurrency limits, nested-project attribution, and hard safety denylist
- Declarative YAML framework specs (embedded) plus Tier-2 Go detectors (e.g. Rust workspaces)
- Git-aware Skipped section (tracked / not ignored / dirty); overrides only via `.reclaim.yaml`
- Control files (`.reclaim.yaml`) with keep/delete precedence, strict mode, and inheritance
- Human plan output with sizing; confirmation prompt; `--json` for automation
- Executor: permanent delete by default, `--to-trash` on darwin/linux/windows, journal + partial-failure exit `4`
- Debugger surface: `explain`, `init`, `detectors list|show|test`
- Flags: `--aggressive`, `--no-config`, `--follow-symlinks`, `--cross-device`, `--use-git-binary`, framework filters, and more (see [spec.md §9](spec.md))

### Explicitly deferred

- `--restore`, `reclaim global` / global caches, network-FS special casing ([spec.md §15](spec.md))

### Manual smoke checklist

Run against disposable fixtures only:

1. `reclaim plan` on a disposable fixture — lists candidates, changes nothing
2. `reclaim scan -y` on a temp fixture — permanently removes reclaimable targets
3. `reclaim scan -y --to-trash` once on the developer’s OS
4. `make build && ./bin/reclaim version` — prints `0.1.0`
