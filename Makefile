# noop — the gate reads `check`, and each prerequisite is one recorded step
# (ipsa gate, lane 1's shape, 2026-09-23). `make check` is what "validated"
# means for this repo: vet, then the tests under the race detector.
.PHONY: check build test run image

check: build test

build:
	go vet ./...
	go build -o noop .

test:
	go test -race -v ./...

run: build
	PORT=8080 ENABLE_CHAOS=true ./noop

# A stamped image; never :latest (VERSION lands in noop_build_info and /version).
VERSION ?= $(shell date -u +%Y.%j.%H%M%S)
image:
	podman build --build-arg VERSION=$(VERSION) -t registry.nessh:30500/loch-nessh/noop:$(VERSION) .
