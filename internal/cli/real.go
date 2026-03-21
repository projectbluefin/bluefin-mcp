package cli

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// RealExecutor runs actual system commands.
type RealExecutor struct{}

func NewRealExecutor() *RealExecutor { return &RealExecutor{} }

func (r *RealExecutor) Run(ctx context.Context, name string, args []string) ([]byte, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, ErrNotInstalled)
	}

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 5 * time.Second // allow 5s for graceful exit after context cancel

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}
