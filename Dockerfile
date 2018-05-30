FROM golang:alpine AS build

WORKDIR /usr/local/bin
RUN wget -O dep https://github.com/golang/dep/releases/download/v0.4.1/dep-linux-amd64 && \
    chmod +x /usr/local/bin/dep

ADD . /go/src/github.com/jerluc/tilenol
WORKDIR /go/src/github.com/jerluc/tilenol
RUN ./build.sh

FROM alpine:3.7

COPY --from=build /go/src/github.com/jerluc/tilenol/target/tilenol /usr/bin/tilenol

EXPOSE 3000
EXPOSE 3001

ENTRYPOINT ["/usr/bin/tilenol"]
