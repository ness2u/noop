# simple noop server

## build
`docker build -t ness2u/noop .`

## run
`docker run --rm -ti -p 9000:9000 ness2u/noop`

## what it does
- `/` does nothing.
- `/count` counts.
- `/mirror` shows request headers.
- `/slow` takes 1 second to respond.
- `/status?code=<code>` to control response status code.
