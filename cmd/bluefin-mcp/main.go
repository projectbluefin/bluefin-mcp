package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mark3labs/mcp-go/server"
	"github.com/projectbluefin/bluefin-mcp/internal/cli"
	"github.com/projectbluefin/bluefin-mcp/internal/system"
	"github.com/projectbluefin/bluefin-mcp/internal/tools"
)

var version = "dev"

func main() {
	// Save the real stdout fd BEFORE redirecting os.Stdout. The MCP protocol
	// uses fd 1 for JSON-RPC responses; we must keep a handle to it even after
	// we point Go's os.Stdout variable at stderr to prevent stray fmt.Print*
	// calls from corrupting the protocol stream.
	realStdout := os.NewFile(1, "/dev/stdout")

	// Redirect Go's os.Stdout so any stray fmt.Print* calls go to stderr
	// instead of corrupting the JSON-RPC protocol stream.
	os.Stdout = os.Stderr

	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Fprintf(os.Stderr, "bluefin-mcp %s\n", version)
		os.Exit(0)
	}

	dataDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "bluefin-mcp")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating data dir: %v\n", err)
		os.Exit(1)
	}

	store, err := system.NewKnowledgeStore(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading knowledge store: %v\n", err)
		os.Exit(1)
	}

	runner := cli.NewRealExecutor()
	s := server.NewMCPServer("bluefin-mcp", version)
	tools.Register(s, runner, store)

	// Use NewStdioServer + Listen (not ServeStdio) so we can pass realStdout
	// explicitly. ServeStdio calls s.Listen(ctx, os.Stdin, os.Stdout) internally,
	// but os.Stdout has been redirected above — all responses would go to stderr.
	// Mirror ServeStdio's signal handling so SIGTERM/SIGINT still trigger a
	// clean shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		cancel()
	}()

	stdio := server.NewStdioServer(s)
	if err := stdio.Listen(ctx, os.Stdin, realStdout); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
