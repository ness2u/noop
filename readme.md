# simple noop server

## build
```
go build
docker build -t ness2u/noop .
```

## run
```
PORT=8080 ./noop
# or
docker run --rm -ti -p 8080:8080 ness2u/noop
```

## what it does
- `/` does nothing.
- `/count` counts.
- `/mirror` shows request headers.
- `/slow?ms=1000` is just as slow as you want.
- `/status?code=<code>` to control response status code.
