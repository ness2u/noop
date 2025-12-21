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

## what it does
- `/` does nothing, but its something.
- `/liveness` and `/healthcheck` also do nothing.
- `/count` counts... so does `/counter`
- `/mirror` shows request headers in the response.
- `/status?code=<code>` to control response status code.

### chaos (enabled via `ENABLE_CHAOS=true`)
- `/latency?ms=<latency-ms>` to induce a slow response.
- `/memory-leak?rate=<bytes-per-leak>&rate=<ms-between-leaks>` to induce a controlled, yet unrecoverable memory leak.
- `/spin-cpu?count=<num-of-spin-routines>&delay=<ms-before-start>&time=<duration-ms-of-spin>` to spin the cpu in various ways.	
- `/crash?delay=<delay-ms>` to cause a server panic.