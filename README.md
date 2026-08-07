# reclaim

A Go CLI that walks a directory tree, finds regenerable build artifacts and dependency caches belonging to real projects, and reclaims the disk space they occupy.

See [spec.md](spec.md) for behaviour and [build-plan.md](build-plan.md) for the incremental implementation plan.

## Development

```sh
make test
go run ./cmd/reclaim version
```
