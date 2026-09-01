# netscope is Linux-only: every .go file carries //go:build linux, so a bare
# `go build`/`go test` on a non-Linux host (e.g. macOS) sees zero files and
# silently no-ops. Every target below runs inside a golang container instead,
# so these commands behave identically on macOS, Linux, and CI.

GO_VERSION := 1.23
DOCKER_RUN := docker run --rm -v "$(CURDIR)":/src -w /src golang:$(GO_VERSION)

.DEFAULT_GOAL := check

.PHONY: all build vet test race check lint tidy clean help

help:
	@echo "Targets:"
	@echo "  build  - go build ./... (CGO_ENABLED=0, the release build gate)"
	@echo "  vet    - go vet ./..."
	@echo "  test   - go test -v ./..."
	@echo "  race   - go test -race -v ./..."
	@echo "  check  - build + vet + race (default target; the full local gate)"
	@echo "  lint   - golangci-lint run (best-effort; no repo config committed yet)"
	@echo "  tidy   - go mod tidy"
	@echo "  clean  - remove local build/test artifacts"

build:
	$(DOCKER_RUN) sh -c "CGO_ENABLED=0 go build ./..."

vet:
	$(DOCKER_RUN) sh -c "go vet ./..."

test:
	$(DOCKER_RUN) sh -c "go test -v ./..."

race:
	$(DOCKER_RUN) sh -c "go test -race -v ./..."

check: build vet race

lint:
	docker run --rm -v "$(CURDIR)":/src -w /src golangci/golangci-lint:latest golangci-lint run ./...

tidy:
	$(DOCKER_RUN) sh -c "go mod tidy"

clean:
	rm -f *.test *.out coverage.* profile.cov
