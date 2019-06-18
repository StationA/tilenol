FROM golang:alpine AS build

RUN apk update && apk add build-base

ADD . /go/src/github.com/stationa/tilenol
WORKDIR /go/src/github.com/stationa/tilenol
RUN wget https://github.com/golang/dep/releases/download/v0.5.1/dep-linux-amd64 -O $GOPATH/bin/dep
RUN chmod +x $GOPATH/bin/dep
RUN make release

FROM alpine:3.7

COPY --from=build /go/src/github.com/stationa/tilenol/target/tilenol /usr/bin/tilenol

EXPOSE 3000
EXPOSE 3001

ENTRYPOINT ["/usr/bin/tilenol"]
