#!/bin/sh

VERSION=`git describe --tags 2>/dev/null || echo "untagged"`
COMMITISH=`git describe --always 2>/dev/null`

go build -o ./target/tilenol -ldflags="-X main.Version=${VERSION} -X main.Commitish=${COMMITISH}"
