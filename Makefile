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
# The platforms every image carries: ness-cloud4 (noop's nodeSelector) is arm64, the rest amd64.
PLATFORMS := linux/amd64 linux/arm64

image:
	@mkdir -p target
	$(eval VERSION ?= $(shell date -u +%Y.%j.%H%M%S))
	printf '%s' "$(VERSION)" > $(STAMP_FILE)
	@! podman manifest exists $(IMAGE_REPO):$(VERSION) || podman manifest rm $(IMAGE_REPO):$(VERSION) >/dev/null
	podman build --platform $(shell echo $(PLATFORMS) | tr ' ' ',') --build-arg VERSION=$(VERSION) --manifest $(IMAGE_REPO):$(VERSION) .
	@echo "built $(IMAGE_REPO):$(VERSION) (stamp in $(STAMP_FILE))"

# The publish half of an RC (`ipsa gate --rc` checks rc:published against the registry's
# manifest for the stamp): pushes exactly the stamp `image` wrote. HTTP registry, so
# --tls-verify=false. Refuses, loudly, when no stamp file exists.
publish:
	@test -s $(STAMP_FILE) || { echo "no stamp: run 'make image' first ($(STAMP_FILE) missing)"; exit 1; }
	podman manifest push --all --tls-verify=false $(IMAGE_REPO):$$(cat $(STAMP_FILE)) docker://$(IMAGE_REPO):$$(cat $(STAMP_FILE))
	@echo "published $(IMAGE_REPO):$$(cat $(STAMP_FILE))"

# Every platform the cluster can schedule noop onto must RUN the image: ness-cloud4 (the
# control-plane nodeSelector) is arm64, the build box amd64 — a 2026.268 RC built amd64-only would
# have crash-looped on cloud4. Each variant is run with PORT=off, so the listener fails right after
# the startup line and nothing is left running; the line must name this stamp AND the platform's
# arch as the binary sees it (runtime.GOARCH), so a variant podman silently swapped for the host's
# cannot pass. arm64 runs under qemu-user here. No pipes: the output is matched with `case`.

.PHONY: test-image
test-image:
	@test -s $(STAMP_FILE) || { echo "no stamp: run 'make image' first ($(STAMP_FILE) missing)"; exit 1; }
	@s=$$(cat $(STAMP_FILE)); fail=0; \
	for p in $(PLATFORMS); do \
	  arch=$${p#*/}; \
	  out=$$(podman run --rm --pull=never --platform $$p -e PORT=off $(IMAGE_REPO):$$s 2>&1); \
	  case "$$out" in \
	    *'"noop starting"'*'"version":"'"$$s"'"'*'"arch":"'"$$arch"'"'*) echo "ok $$p: runs, version $$s, arch $$arch" ;; \
	    *) echo "FAIL $$p: no startup line naming version $$s and arch $$arch; got: $$out"; fail=$$((fail+1)) ;; \
	  esac; \
	done; \
	[ $$fail -eq 0 ] && echo "PASS $(IMAGE_REPO):$$s on $(PLATFORMS)"

# The watch (ipsa enact deploy reads `make -s watch` by convention; shtoned 50e51c02's shape).
# Each counter prints `counter=<name> count=<n>` or `counter=<name> unreadable=<why>` — never a
# silent zero — and the LAST line is how many counters failed. Since e103d3c0 a deploy with no
# watch is refused, so this target is what lets noop roll at all.
#   healthz_failing  /healthz non-200 over 3 reads of the Service (its ClusterIP, read live: this
#                    runs on a cluster node; NOOP_URL overrides)
#   restarts         container restarts of the pods running the image being deployed ($IPSA_IMAGE,
#                    exported by the routine) — not the old pods' history (the pre-RC pod carries 9)
.PHONY: watch
NOOP_NS  ?= ness2u-xyz-live
NOOP_URL ?=
watch:
	@fail=0; \
	url="$(NOOP_URL)"; \
	if [ -z "$$url" ]; then ip=$$(kubectl -n $(NOOP_NS) get svc noop-service -o jsonpath='{.spec.clusterIP}' 2>/dev/null); [ -n "$$ip" ] && url="http://$$ip:8080"; fi; \
	if [ -z "$$url" ]; then echo "counter=healthz_failing unreadable=no Service noop-service in $(NOOP_NS)"; fail=$$((fail+1)); \
	else bad=0; seen=""; for i in 1 2 3; do c=$$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$$url/healthz"); seen="$$seen $$c"; [ "$$c" = "200" ] || bad=$$((bad+1)); done; \
	  echo "counter=healthz_failing count=$$bad (reads:$$seen from $$url/healthz)"; [ "$$bad" -eq 0 ] || fail=$$((fail+1)); fi; \
	r=$$(kubectl -n $(NOOP_NS) get pods -l app=noop-service -o jsonpath='{range .items[*]}{.spec.containers[0].image}|{.metadata.name}={.status.containerStatuses[0].restartCount}{" "}{end}' 2>/dev/null); \
	if [ -z "$$r" ]; then echo "counter=restarts unreadable=kubectl listed no noop-service pods in $(NOOP_NS)"; fail=$$((fail+1)); \
	else sel=$$(printf '%s\n' $$r | awk -F'|' -v img="$$IPSA_IMAGE" 'function repo(s) { sub(/^[^\/]*\//, "", s); return s } img == "" || repo($$1) == repo(img) {print $$2}'); \
	  if [ -z "$$sel" ]; then echo "counter=restarts unreadable=no pod runs $$IPSA_IMAGE yet ($$r)"; fail=$$((fail+1)); \
	  else n=$$(printf '%s\n' $$sel | awk -F= 'NF==2 {s+=$$2} END {print s+0}'); echo "counter=restarts count=$$n ($$(echo $$sel))"; [ "$$n" -eq 0 ] || fail=$$((fail+1)); fi; fi; \
	echo $$fail; [ $$fail -eq 0 ]
