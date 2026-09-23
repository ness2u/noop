# simple noop server

## build
```bash
go build
# or
podman build -t noop:latest .
```

## run
```bash
PORT=8080 ./noop
# or with chaos enabled
PORT=8080 ENABLE_CHAOS=true ./noop

# or with podman
podman run --rm -ti -p 8080:8080 noop:latest
# or with podman and chaos
podman run --rm -ti -p 8080:8080 -e ENABLE_CHAOS=true noop:latest
```

## what it emits about itself
- One JSON log line per request on stdout (`cid`, method, path, query, status, bytes,
  `duration_ms`, remote, user agent). `LOG_LEVEL=debug|info|warn|error` (default `info`).
- `x-correlation-id` on every response: the caller's `X-Correlation-Id` (or the older
  `x-correlationId`) echoed, else one minted per request.
- `/metrics` — Prometheus exposition, standard library only: requests, bytes and a duration
  histogram per path; in-flight; recovered handler panics; the `/count` value; build info and
  uptime; Go heap/goroutines/GC; and `noop_chaos_*` for every injection (leak bytes and active
  leaks, cpu spinners, crash armed, latency induced) so what was injected is readable next to
  what it did.
- `/version` — `{"version","go","chaos","uptime_seconds"}`. Stamp the version at build time:
  `go build -ldflags "-X main.version=<stamp>"` or `podman build --build-arg VERSION=<stamp>`.

## what it does
- `/` does nothing, but its something.
- `/liveness` and `/healthcheck` also do nothing.
- `/count` counts... so does `/counter`
- `/mirror` shows request headers in the response.
- `/status?code=<code>` to control response status code.
- `/download?size=<bytes>` streams text bytes (ASCII lorem pattern) with exact length.
- `/throughput?bps=<bytes-per-second>&size=<bytes>` streams ASCII text paced to a per-connection target Bps; repeated calls on the same TCP connection update the target Bps.

### chaos (enabled via `ENABLE_CHAOS=true`)
- `/latency?ms=<latency-ms>` to induce a slow response.
- `/memory-leak?rate=<bytes-per-leak>&rate=<ms-between-leaks>` to induce a controlled, yet unrecoverable memory leak.
- `/spin-cpu?count=<num-of-spin-routines>&delay=<ms-before-start>&time=<duration-ms-of-spin>` to spin the cpu in various ways.	
- `/crash?delay=<delay-ms>` to cause a server panic.
