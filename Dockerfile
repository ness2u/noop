FROM golang:alpine

RUN apk --no-cache add curl bash git

ADD noop.go /go/src/noop/
RUN go install noop

CMD noop

