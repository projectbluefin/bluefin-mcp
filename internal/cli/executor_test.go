package cli_test

import (
"context"
"testing"
"time"

"github.com/projectbluefin/bluefin-mcp/internal/cli"
)

func TestMockExecutor_ReturnsConfiguredOutput(t *testing.T) {
mock := cli.NewMockExecutor()
mock.SetResponse("ujust", []string{"--list"}, []byte("Available recipes:\n    update\n"), nil)

out, err := mock.Run(context.Background(), "ujust", []string{"--list"})
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if string(out) != "Available recipes:\n    update\n" {
t.Errorf("got %q, want configured output", string(out))
}
}

func TestMockExecutor_UnregisteredCommand_ReturnsError(t *testing.T) {
mock := cli.NewMockExecutor()
_, err := mock.Run(context.Background(), "nonexistent", nil)
if err == nil {
t.Fatal("expected error for unregistered command, got nil")
}
}

func TestMockExecutor_RecordsCalls(t *testing.T) {
mock := cli.NewMockExecutor()
mock.SetResponse("bootc", []string{"status", "--json"}, []byte("{}"), nil)
mock.Run(context.Background(), "bootc", []string{"status", "--json"})
mock.Run(context.Background(), "bootc", []string{"status", "--json"})

calls := mock.CallsFor("bootc")
if len(calls) != 2 {
t.Errorf("expected 2 calls, got %d", len(calls))
}
}

func TestMockExecutor_ContextCancellation(t *testing.T) {
mock := cli.NewMockExecutor()
mock.SetDelay("sleep", []string{"10"}, 5*time.Second)

ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
defer cancel()

_, err := mock.Run(ctx, "sleep", []string{"10"})
if err == nil {
t.Fatal("expected context cancellation error, got nil")
}
}
