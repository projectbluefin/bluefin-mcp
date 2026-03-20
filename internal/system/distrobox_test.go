package system_test

import (
"context"
"os"
"testing"

"github.com/projectbluefin/bluefin-mcp/internal/cli"
"github.com/projectbluefin/bluefin-mcp/internal/system"
)

func TestListDistrobox_ParsesTable(t *testing.T) {
data, err := os.ReadFile("../../testdata/distrobox-list.txt")
if err != nil {
t.Fatalf("missing testdata: %v", err)
}
mock := cli.NewMockExecutor()
mock.SetResponse("distrobox", []string{"list"}, data, nil)

boxes, err := system.ListDistrobox(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if len(boxes) != 2 {
t.Errorf("expected 2 boxes, got %d", len(boxes))
}
if boxes[0].Name != "ubuntu-22.04" {
t.Errorf("expected first box name 'ubuntu-22.04', got %q", boxes[0].Name)
}
}

func TestListDistrobox_StatusPreserved(t *testing.T) {
data, _ := os.ReadFile("../../testdata/distrobox-list.txt")
mock := cli.NewMockExecutor()
mock.SetResponse("distrobox", []string{"list"}, data, nil)

boxes, err := system.ListDistrobox(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if boxes[0].Status != "Up" {
t.Errorf("expected first box status 'Up', got %q", boxes[0].Status)
}
if boxes[1].Status != "Created" {
t.Errorf("expected second box status 'Created', got %q", boxes[1].Status)
}
}

func TestListDistrobox_ImagePreserved(t *testing.T) {
data, _ := os.ReadFile("../../testdata/distrobox-list.txt")
mock := cli.NewMockExecutor()
mock.SetResponse("distrobox", []string{"list"}, data, nil)

boxes, err := system.ListDistrobox(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if boxes[0].Image != "docker.io/library/ubuntu:22.04" {
t.Errorf("unexpected image for ubuntu box: %q", boxes[0].Image)
}
}

func TestListDistrobox_NotInstalled_DegradeGracefully(t *testing.T) {
mock := cli.NewMockExecutor()
mock.SetResponse("distrobox", []string{"list"}, nil, cli.ErrNotInstalled)

result, err := system.ListDistrobox(context.Background(), mock)
// Should NOT return error — should return empty list with not-installed indicator
if err != nil {
t.Fatalf("expected graceful degrade, got error: %v", err)
}
if result != nil && len(result) != 0 {
t.Error("expected empty result when distrobox not installed")
}
}
