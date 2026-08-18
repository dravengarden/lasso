# Lasso product task runner
set shell := ["bash", "-euo", "pipefail", "-c"]

default:
    @just --list

build:
    go build -ldflags "-X github.com/dravengarden/lasso/internal/cli.Version=0.1.0 -X github.com/dravengarden/lasso/internal/cli.BuildRevision=$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)" -o bin/ ./cmd/...

alias install := build

fmt:
    gofmt -w cmd internal

format-check:
    test -z "$(gofmt -l cmd internal)"

lint:
    go vet ./...
    bash scripts/check-skills.sh

test:
    go test ./...

docs-index-check:
    bash modules/docs-index/scripts/check-docs-index.sh

verify: format-check lint test docs-index-check
    @echo "verify ok"

# Documentation site (Astro Starlight)
website-install:
    cd website && npm install

website-dev:
    cd website && npm run dev

website-build:
    cd website && npm run build

website-preview:
    cd website && npm run preview

doctor:
    command -v go
    command -v git
    test -x bin/lasso || just build
    ./bin/lasso version
