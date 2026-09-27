# One manifest, one image per platform (linux/amd64 + linux/arm64: `make image`). The build stage
# runs on the BUILD host's platform and cross-compiles for each target — CGO is off, so Go needs no
# emulation — and the runtime stage only copies, so no RUN ever executes under qemu.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine as BuildStage
RUN apk --no-cache add curl bash git

# Every .go file in the package, not one named file: a source file added
# later (observe.go, 2026-09-22) must not be able to build locally and
# vanish from the image.
ADD go.mod /go/src/noop/
COPY *.go /go/src/noop/
WORKDIR /go/src/noop/
# VERSION is stamped into the binary (noop_build_info, /version, the startup
# log line). Pass --build-arg VERSION=<stamp>; the default says it was not.
ARG VERSION=unstamped
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-X main.version=${VERSION}" -o noop

FROM alpine:3.23
WORKDIR /
COPY --from=BuildStage /go/src/noop/noop /noop
ENTRYPOINT ["/noop"]
