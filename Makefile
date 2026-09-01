# netscope supports Linux (via netlink) and BSD/macOS (via routing sockets).
# Docker targets test the Linux build gate inside a golang container.

GO_VERSION := 1.23
DOCKER_RUN := docker run --rm -v "$(CURDIR)":/src -w /src golang:$(GO_VERSION)

.DEFAULT_GOAL := check

.PHONY: all build vet test test-native race check lint tidy clean help

help:
	@echo "Targets:"
	@echo "  build       - go build ./... in Docker (CGO_ENABLED=0, release gate)"
	@echo "  vet         - go vet ./... in Docker"
	@echo "  test        - go test -v ./... in Docker"
	@echo "  test-native - go test -v ./... natively on host OS"
	@echo "  race        - go test -race -v ./... in Docker"
	@echo "  check       - build + vet + race (default target; full local gate)"
	@echo "  lint        - golangci-lint run (best-effort; no repo config committed yet)"
	@echo "  tidy        - go mod tidy in Docker"
	@echo "  clean       - remove local build/test artifacts"

build:
	$(DOCKER_RUN) sh -c "CGO_ENABLED=0 go build ./..."

vet:
	$(DOCKER_RUN) sh -c "go vet ./..."

test:
	$(DOCKER_RUN) sh -c "go test -v ./..."

test-native:
	go test -v ./...

race:
	$(DOCKER_RUN) sh -c "go test -race -v ./..."

check: build vet race

lint:
	docker run --rm -v "$(CURDIR)":/src -w /src golangci/golangci-lint:latest golangci-lint run ./...

tidy:
	$(DOCKER_RUN) sh -c "go mod tidy"

clean:
	rm -f *.test *.out coverage.* profile.cov
