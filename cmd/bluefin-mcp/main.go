package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Fprintf(os.Stderr, "bluefin-mcp %s\n", version)
		os.Exit(0)
	}
	fmt.Fprintln(os.Stderr, "bluefin-mcp starting...")
}
