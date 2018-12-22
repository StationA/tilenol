VERSION := `git describe --tags 2>/dev/null || echo "untagged"`
COMMITISH := `git describe --always 2>/dev/null`

tools:
	@go install github.com/golang/dep/cmd/dep

deps: tools
	dep ensure

build:
	@go build -o target/tilenol -ldflags="-X main.Version=${VERSION} -X main.Commitish=${COMMITISH}" ./cmd/...

install: build
	@go install -ldflags="-X main.Version=${VERSION} -X main.Commitish=${COMMITISH}" ./cmd/...

target:
	mkdir -p target

release: build target
	@CGO_ENABLED=0 go build -a -o target/tilenol -ldflags="-X main.Version=${VERSION} -X main.Commitish=${COMMITISH}" ./cmd/...

clean:
	@rm -rf target

.PHONY: tools build install release clean
