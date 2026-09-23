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
