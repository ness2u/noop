FROM golang:1.25-alpine as BuildStage
RUN apk --no-cache add curl bash git 

ADD go.mod /go/src/noop/
ADD noop.go /go/src/noop/
WORKDIR /go/src/noop/
RUN go build -o noop

FROM alpine:3.23
WORKDIR /
COPY --from=BuildStage /go/src/noop/noop /noop
ENTRYPOINT ["/noop"]