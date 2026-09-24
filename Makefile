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
#
# The deploy contract's shape (ipsa gate --rc; deploy-routine.md §5): `image` and `publish`
# run as SEPARATE make invocations, so the stamp is decided ONCE, by `image`, and written to
# STAMP_FILE; `publish` reads it from there, never from the clock — or two invocations would
# mint two stamps and publish one that was never built. The gate exports IPSA_STAMP (the
# manifest's current tag, `master_latest` today); minting ignores it on purpose.
STAMP_FILE := target/image-stamp
IMAGE_REPO := registry.nessh:30500/loch-nessh/noop

image:
	@mkdir -p target
	$(eval VERSION ?= $(shell date -u +%Y.%j.%H%M%S))
	printf '%s' "$(VERSION)" > $(STAMP_FILE)
	podman build --build-arg VERSION=$(VERSION) -t $(IMAGE_REPO):$(VERSION) .
	@echo "built $(IMAGE_REPO):$(VERSION) (stamp in $(STAMP_FILE))"

# The publish half of an RC (`ipsa gate --rc` checks rc:published against the registry's
# manifest for the stamp): pushes exactly the stamp `image` wrote. HTTP registry, so
# --tls-verify=false. Refuses, loudly, when no stamp file exists.
publish:
	@test -s $(STAMP_FILE) || { echo "no stamp: run 'make image' first ($(STAMP_FILE) missing)"; exit 1; }
	podman push --tls-verify=false $(IMAGE_REPO):$$(cat $(STAMP_FILE))
	@echo "published $(IMAGE_REPO):$$(cat $(STAMP_FILE))"
