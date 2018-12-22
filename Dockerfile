FROM golang:alpine AS build

RUN apk update && apk add make

ADD . /go/src/github.com/jerluc/tilenol
WORKDIR /go/src/github.com/jerluc/tilenol
RUN make release

FROM alpine:3.7

COPY --from=build /go/src/github.com/jerluc/tilenol/target/tilenol /usr/bin/tilenol

EXPOSE 3000
EXPOSE 3001

ENTRYPOINT ["/usr/bin/tilenol"]
