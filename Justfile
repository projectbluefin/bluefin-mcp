default: build

build:
    go build -ldflags "-X main.buildDate=$(date -u +%Y-%m-%d)" -o bin/bluefin-mcp ./cmd/bluefin-mcp

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
