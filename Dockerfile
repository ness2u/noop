FROM golang:1.21-alpine as BuildStage
RUN apk --no-cache add curl bash git 

ADD go.mod /go/src/noop/
ADD noop.go /go/src/noop/
WORKDIR /go/src/noop/
RUN pwd
RUN ls -al
RUN go build && pwd && ls -al

FROM alpine:latest
WORKDIR /
COPY --from=BuildStage /go/src/noop/noop /noop
ENTRYPOINT ["/noop"]

