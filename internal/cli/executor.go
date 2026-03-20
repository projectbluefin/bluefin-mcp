package cli

import (
	"context"
	"errors"
)

// ErrNotInstalled is returned when a binary is not found in PATH.
var ErrNotInstalled = errors.New("binary not installed")

// CommandRunner executes external commands. Injected into system/ packages for testability.
type CommandRunner interface {
	Run(ctx context.Context, name string, args []string) ([]byte, error)
}
