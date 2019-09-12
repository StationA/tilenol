VERSION := `git describe --tags 2>/dev/null || echo "untagged"`
COMMITISH := `git describe --always 2>/dev/null`

deps:
	dep ensure

format:
	go fmt ./...

build: deps
	go build -o target/tilenol -ldflags="-X main.Version=${VERSION} -X main.Commitish=${COMMITISH}" ./cmd/...

lint: format
	golint github.com/stationa/tilenol

test: build
	go test -v -coverprofile=/tmp/tilenol.coverage.out ./...

coverage: test
	go tool cover -func=/tmp/tilenol.coverage.out

install: test
	go install -ldflags="-X main.Version=${VERSION} -X main.Commitish=${COMMITISH}" ./cmd/...

target:
	mkdir -p target

release: test target
	CGO_ENABLED=0 go build -a -o target/tilenol -ldflags="-X main.Version=${VERSION} -X main.Commitish=${COMMITISH}" ./cmd/...

clean:
	rm -rf target

.PHONY: build test install release clean
