# Single entry point for everything: `just` lists the recipes.
# CI and the pre-commit hooks call these same recipes.

set shell := ["bash", "-euo", "pipefail", "-c"]

# Build the tools below with the Go version from go.mod, not whatever is installed.
export GOTOOLCHAIN := "go" + `sed -n 's/^go //p' go.mod`

# Linters run through `go run pkg@version`, so nothing to install and the
# versions are pinned here, in one place. (Also needed on PATH: shellcheck.)
golangci_lint := "github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8"
govulncheck   := "golang.org/x/vuln/cmd/govulncheck@v1.1.4"
shfmt         := "mvdan.cc/sh/v3/cmd/shfmt@v3.10.0"
actionlint    := "github.com/rhysd/actionlint/cmd/actionlint@v1.7.7"

version := env("VERSION", `git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo dev`)
image   := env("IMAGE", "fdroid-selfhost:local")
shell_files := "scripts/*.sh e2e/*.sh"

default:
    @just --list

# Install the git hooks
setup:
    pre-commit install

# --- Develop ---------------------------------------------------------------

# Build the server binary into bin/
build:
    go build -trimpath -ldflags "-X main.version={{ version }}" -o bin/fdroid-selfhost ./cmd/fdroid-selfhost

# Build the server's Docker image
docker-build:
    docker build --build-arg VERSION={{ version }} -t {{ image }} .

# Run the server in Docker (needs a .env, see .env.example)
up:
    mkdir -p data
    docker compose up --build

# Rewrite files into the canonical format
fmt:
    gofmt -w .
    go run {{ shfmt }} -w {{ shell_files }}

# --- Check -----------------------------------------------------------------

# All linters
lint: lint-go lint-shell lint-docker lint-actions

lint-go:
    go vet ./...
    test -z "$(gofmt -l .)" || { gofmt -l .; echo "run: just fmt"; exit 1; }
    go run {{ golangci_lint }} run

lint-shell:
    shellcheck {{ shell_files }}
    go run {{ shfmt }} -d {{ shell_files }}

# Uses hadolint from PATH, or its Docker image
lint-docker:
    #!/usr/bin/env bash
    set -euo pipefail
    if command -v hadolint >/dev/null; then
        hadolint Dockerfile
    else
        docker run --rm -i -v "$PWD/.hadolint.yaml:/.config/hadolint.yaml:ro" hadolint/hadolint < Dockerfile
    fi

lint-actions:
    go run {{ actionlint }}

# Unit tests
test: test-go test-scripts

test-go:
    go test -race -cover ./...

test-scripts:
    scripts/test-version.sh

# Known vulnerabilities in dependencies and the standard library
vuln:
    go run {{ govulncheck }} ./...

# End-to-end test of the real Docker image
e2e: docker-build
    e2e/run.sh {{ image }}

# Everything CI checks; run before pushing
ci: lint test vuln e2e

clean:
    rm -rf bin
