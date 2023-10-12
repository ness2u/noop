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
- `/` does nothing.
- `/count` counts... so does `/counter`
- `/mirror` shows request headers.
- `/slow?ms=1000` is just as slow as you want.
- `/status?code=<code>` to control response status code.
