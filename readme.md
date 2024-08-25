# simple noop server

## build
```
go build
# or
docker build -t noop:latest .
```

## run
```
PORT=8080 ./noop
# or
docker run --rm -ti -p 8080:8080 noop:latest
```

## what it does
- `/` does nothing, but its something.
- `/count` counts... so does `/counter`
- `/mirror` shows request headers in the response.
- `/latency?ms=<latency-ms>` to induce a slow response.
- `/status?code=<code>` to control response status code.
- `/memory-leak?rate=<bytes-per-leak>&rate=<ms-between-leaks>` to induce a controlled, yet unrecoverable memory leak.
- `/spin-cpu?count=<num-of-spin-routines>&delay=<ms-before-start>&time=<duration-ms-of-spin>` to spin the cpu in various ways.	
- `/crash?delay=<delay-ms>` to cause a server panic.
