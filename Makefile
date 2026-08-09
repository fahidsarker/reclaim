.PHONY: test build fuzz smoke-exec

VERSION ?= 0.1.0
LDFLAGS = -X github.com/fahid/reclaim/internal/cli.Version=$(VERSION)

test:
	go test ./internal/rules ./internal/exec ./...

build:
	go build -ldflags "$(LDFLAGS)" -o bin/reclaim ./cmd/reclaim

fuzz:
	go test ./internal/config -run=^$$ -fuzz=FuzzParseControl -fuzztime=10s
	go test ./internal/config -run=^$$ -fuzz=FuzzMatchPattern -fuzztime=10s

smoke-exec:
	@mkdir -p bin/smoke
	GOOS=linux GOARCH=amd64 go test -c -o bin/smoke/exec_linux.test ./internal/exec
	GOOS=darwin GOARCH=amd64 go test -c -o bin/smoke/exec_darwin.test ./internal/exec
	GOOS=windows GOARCH=amd64 go test -c -o bin/smoke/exec_windows.test ./internal/exec
	@rm -rf bin/smoke
	@echo "smoke-exec: linux/darwin/windows ./internal/exec compiled OK"
