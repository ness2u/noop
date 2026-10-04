## 2026-10-04 — request metrics are labelled by route, not raw path (lane 1b; the TPM's ask)

Internet scanners hit noop's public listener with paths like `/0.php`, `/123viva.php` and `/3PJcpMFsD8B.php`. `noop_http_requests_total`, `noop_http_response_bytes_total` and `noop_http_request_duration_seconds` labelled each with its raw `path`, so every scanner path minted new series in the metrics store, without bound.
- **Now:** the label is `route`: the pattern the mux matched (`mux.Handler(r)`). Anything only the `/` catch-all took (any path but `/` itself), or no route at all, is `unmatched`, as my-server does. The label NAME changes too (`path` → `route`, my-server's name), so the old series age out with the store's retention. Nothing outside noop queried `path` (no dashboard spec, no ansible file).
- **Kept:** the request log line still carries the raw path (`consult verdict` joins the window's logged paths).
- **Red first:** `TestScannerPathsDoNotMintSeries` failed on the old code (the raw paths in `/metrics`, no `route`). It asserts deltas, because the store is process-wide. The older metrics test's `/latency` is a public-listener miss, so it now reads `unmatched`. `go vet` and `gofmt` are clean; `go test -race` ok.

## 2026-09-29 — chaos moves to a private loopback listener; the public one never carries it (lane 1b)

noop's public listener is the one a Service and an Ingress name. With `ENABLE_CHAOS=true`, `/crash`, `/memory-leak`, `/spin-cpu` and `/latency` were served on it, so turning chaos on for a demo would have opened them to anyone who could reach the service. `/version` also said `"chaos"` to every caller.
- **Now:** chaos on starts a second listener on `127.0.0.1:${CHAOS_PORT:-8081}`, carrying only the injections. The public mux never registers them, whatever the flag says. Chaos off opens no second port. Loopback, not every interface: in a pod, only the pod itself and a port-forward can reach it, and the caller's own cluster permission governs the port-forward. No other pod can reach it by IP, whatever the network policy. If the chaos listener fails to bind, noop exits 1: chaos asked for and not served is a failed start.
- `/version` is now `{version, go, uptime_seconds}`. `noop_chaos_enabled` stays on `/metrics`, since a verdict reads what was injected there.
- **Red:** 34f16a6. With chaos on, the public mux routed all four chaos paths, and `/version` carried `"chaos":true`. The check uses the mux's own routing, so no crash is armed in the test binary.
- **Green:** `make check` (vet, build, `go test -race`, 13 tests). The binary was run once each way. On: `127.0.0.1:18081` and `*:18080` listening; public `/latency` and `/crash` → `nothing`; private `/latency` → a slow response; the private port by the host's own address → no answer. Off: only `*:18080`.
- **Not done here:** turning chaos on in a deployment, which is a spec change on the owner's word.

## 2026-09-28 — demo step 8 proven live: a bad candidate rolls back by stamp (ness ran both deploys; verified by ipsa-agent-4b)

- **Baseline, 02:26Z** (`.ipsa/runs/20260928T022606Z`): `ipsa enact deploy` put noop-service (`ness2u-xyz-live`, ness-cloud4 arm64) onto `2026.270.214444` = `release-candidate/noop/23a1113`. Every step settled. The rollout took 16 s, and the 90 s watch came back clean: healthz 200×3, 0 restarts. This replaced `master_latest`.
- **Bad candidate, 02:28Z** (the fixture worktree `~/.cache/ipsa-fixture/noop`, `.ipsa/runs/20260928T022805Z`): the image was `2026.270.214707-fixture-bad`, which exits 30 s after start.
  - The pod went Ready. Its healthz still read 200×3.
  - The watch failed at 97.8 s on **restarts=2** (`watch_failed`).
  - `rolled_back=true` by the stamp read before the apply: `set image … noop:2026.270.214444`, image only, since the diff beyond the image was false. `rollback_verify` was ok.
- **Read back afterwards:** the deployment names `2026.270.214444`, and its pod is 1/1 with 0 restarts.
- **For the demo:** the failure is caught by the restart counter, not by health. A candidate that crashes on a timer stays healthy between restarts. That's why the watch has two counters.

## 2026-09-27 — images carry linux/arm64 again: noop runs on ness-cloud4, which is arm64 (ipsa-agent-4b, lane 1; the TPM's go)

Found while putting noop onto stamped images for the demo. The only candidate, `release-candidate/noop/8081305` = `2026.268.213753`, is **amd64 only** (its config blob: `architecture: amd64`). noop's nodeSelector (`workload.ness2u.xyz/control-plane`) places it on **ness-cloud4, arm64**, where that image cannot exec its binary. What runs there today, `master_latest`, is a two-platform index from the old tekton `buildah-multiarch` pipeline; the Makefile `image` target that replaced it (d560c6e/8081305) dropped arm64. Rolling the RC would have crash-looped on stage.
- `test-image` (new; the deploy contract runs it between `image` and `publish`): every platform in `PLATFORMS` must RUN the image. Each variant starts with `PORT=off`, so it exits right after its startup line, and that line must name the stamp and the arch the binary executes as. The startup log now carries `os`/`arch` from `runtime`, so a variant podman quietly swapped for the host's can't pass. arm64 runs under qemu-user on linux3.
- Red first: today's single-platform `make image` → `ok linux/amd64`, `FAIL linux/arm64`, exit 2.
- Green: the Dockerfile's build stage runs on `$BUILDPLATFORM` and cross-compiles per `$TARGETOS/$TARGETARCH` (CGO off), so nothing compiles under emulation. The runtime stage only copies. `make image` builds one manifest for `linux/amd64 linux/arm64` (32 s), and `test-image` passes on both. `publish` pushes the whole manifest (`podman manifest push --all`).
- The 2026.268.213753 tag stays (history; its annotation is honest about what ran). It must never be rolled to cloud4. The next candidate comes from this commit through `ipsa gate --rc`.

## 2026-09-26 — a `watch` target (ipsa-agent-4b, lanes 1+2): since ipsa e103d3c0 a deploy with no watch counters is refused before anything runs, so this is what lets noop roll. Two counters in shtoned 50e51c02's shape, each able to say `unreadable`: `healthz_failing` (/healthz non-200 over 3 reads of the Service's live ClusterIP; `NOOP_URL` overrides) and `restarts` (pods running the image being deployed, `$IPSA_IMAGE`, compared without the registry host, so `ness-linux3.nessh:30500` and `registry.nessh:30500` name the same image; the pre-RC pod carries 9 restarts of history that a roll must not be judged on). Checked live: served image 0 failing reads but 9 restarts (fails, correctly, on that pod); an image no pod runs → unreadable; unreachable URL → 3 failing; wrong namespace → both unreadable.

## 2026-09-23 — the gate's first customer: a Makefile the gate reads, and the tests as the fix
- Lane 1 demonstrated the commit-gate's first real refusal on this repo at 99f0c23 (`ipsa gate .` →
  REFUSED `no_gate`, nothing written, the refusal printing the Makefile to declare). This commit is the fix
  that turns it green: `Makefile` with `check: build test` (`go vet ./...` + `go build`, then `go test
  -race -v ./...`), a `run` target with chaos on, and an `image` target that stamps `VERSION` and never
  tags `:latest`. `noop_test.go`: nine tests over `newMux()` behind `observe()` on an httptest server that
  carries the same per-connection pacing as a pod (`trackConnections`, extracted from `main` for exactly
  that) — every endpoint's status and body, the correlation id echoed in both spellings or minted once
  per request, `/count` under 200 concurrent requests with `-race`, exact byte counts and pacing on
  download/throughput, `/mirror` hiding the authorization header, `/metrics` carrying the per-path series
  and the chaos series and parsing line by line, `/version` as JSON with the stamp, a handler panic as a
  logged and counted 500, and the chaos routes absent when chaos is off (the root route is a catch-all,
  so an unregistered `/latency` answers `nothing`, never a slow response).

## 2026-09-22 — observability for the demo (the ipsa-sre lane, week of 09-22)
- `observe.go`: one JSON log line per request (`log/slog`, stdout; `cid`, method, path, query,
  status, bytes, `duration_ms`, remote, user agent), a recovered handler panic logged as a 500
  and counted, and `/metrics` in Prometheus exposition — requests/bytes by method+path+status,
  a duration histogram by path, in-flight, the `/count` value, build info, uptime, the Go
  runtime's heap/goroutines/GC, and one series per chaos injection (`noop_chaos_*`: leak bytes
  and active leaks, cpu spinners, crash armed, latency induced) so a verdict can be read
  against what was injected. `/version`. Standard library only.
- `noop.go`: routes on a `ServeMux` behind the middleware; the `/count` counter is atomic (it
  was a plain int64 written from every goroutine); `fmt.Printf` side prints became structured
  log lines; the correlation id is minted once per request and honours both header spellings.
- `Dockerfile`: `COPY *.go` instead of one named file (a new source file must not vanish from
  the image), `ARG VERSION` stamped into `main.version`, `CGO_ENABLED=0`.
- `k8s/deployment.yaml`: pull ref → `registry.nessh:30500` (one registry, on server2; the
  linux3 names are mirror aliases — todo.md, answered by ansible-root-04); liveness/readiness
  probes on `/healthz`; `prometheus.io/*` scrape annotations; `LOG_LEVEL`.
- **Not done on purpose:** the test suite. Lane 1 pilots the commit-gate on this repo and needs
  its zero tests for the gate's first real refusal; the tests land after that refusal is
  demonstrated, as the fix that turns it green. Verified instead by `go vet`, a build, and a
  local run with chaos on (every endpoint exercised; log and `/metrics` read back).

## 2026-09-03 — ipsa loop runs
- sdlc: add a /healthz endpoint — settled (2 step(s), validate green)

# Audit — noop

## 2026-08-17: SOP Sync (ledger-wide audit)
- `.mimir/` gitignored; seeded this audit log. Project is stable/complete for
  current needs (chaos/dummy server with /download and /throughput endpoints).

## Earlier (from git history)
- Per-connection rolling throughput history (10 req / 10s), ASCII-encodable
  download/throughput outputs with exact byte counts, pacing fixes.
- Tekton pipeline moved to gitlab-runner-labeled nodes; internal registry push path.
