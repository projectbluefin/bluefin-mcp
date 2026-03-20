default: build

build:
    go build -o bin/bluefin-mcp ./cmd/bluefin-mcp

install: build
    cp bin/bluefin-mcp ~/.local/bin/bluefin-mcp

test:
    go test -race ./...

lint:
    go vet ./...
    staticcheck ./...

verify-binary:
    ~/.local/bin/bluefin-mcp --version

clean:
    rm -rf bin/
