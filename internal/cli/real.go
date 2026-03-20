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

	out, err := cmd.Output()
	if err != nil {
		// On context cancellation, send SIGTERM then SIGKILL to process group
		if ctx.Err() != nil && cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			time.Sleep(3 * time.Second)
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil, err
	}
	return out, nil
}
