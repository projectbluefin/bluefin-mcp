package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/server"
	"github.com/projectbluefin/bluefin-mcp/internal/cli"
	"github.com/projectbluefin/bluefin-mcp/internal/system"
	"github.com/projectbluefin/bluefin-mcp/internal/tools"
)

var version = "dev"

func main() {
	// CRITICAL: redirect stdout to stderr BEFORE any other output.
	// Any write to stdout corrupts the JSON-RPC MCP protocol.
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

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
