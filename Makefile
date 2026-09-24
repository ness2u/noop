# noop — the gate reads `check`, and each prerequisite is one recorded step
# (ipsa gate, lane 1's shape, 2026-09-23). `make check` is what "validated"
# means for this repo: vet, then the tests under the race detector.
.PHONY: check build test run image publish

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

# The publish half of an RC (`ipsa gate --rc` checks rc:published): the stamped image
# pushed to the estate's registry (HTTP, so --tls-verify=false). Never :latest.
publish: image
	podman push --tls-verify=false registry.nessh:30500/loch-nessh/noop:$(VERSION)
	@echo "published registry.nessh:30500/loch-nessh/noop:$(VERSION)"
