FROM golang:1.21-alpine

RUN apk --no-cache add curl bash git

ADD go.mod /go/src/noop/
ADD noop.go /go/src/noop/
WORKDIR /go/src/noop/
RUN pwd
RUN ls -al
RUN go install 
#./noop@latest 

CMD noop

