package system_test

import (
"context"
"os"
"testing"

"github.com/projectbluefin/bluefin-mcp/internal/cli"
"github.com/projectbluefin/bluefin-mcp/internal/system"
)

func TestParseBootcStatus_BaseVariant(t *testing.T) {
data, err := os.ReadFile("../../testdata/bootc-status.json")
if err != nil {
t.Fatalf("missing testdata: %v", err)
}
mock := cli.NewMockExecutor()
mock.SetResponse("bootc", []string{"status", "--json"}, data, nil)

status, err := system.GetSystemStatus(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if status.Variant != "base" {
t.Errorf("expected variant 'base', got %q", status.Variant)
}
if status.ImageRef != "ghcr.io/ublue-os/bluefin:stable" {
t.Errorf("unexpected image ref: %q", status.ImageRef)
}
if status.Booted == "" {
t.Error("booted digest should not be empty")
}
if !status.RollbackAvailable {
t.Error("rollback should be available when rollback field present")
}
}

func TestParseBootcStatus_DXVariant(t *testing.T) {
data, _ := os.ReadFile("../../testdata/bootc-status-dx.json")
mock := cli.NewMockExecutor()
mock.SetResponse("bootc", []string{"status", "--json"}, data, nil)

status, err := system.GetSystemStatus(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if status.Variant != "dx" {
t.Errorf("expected variant 'dx', got %q", status.Variant)
}
}

func TestParseBootcStatus_NvidiaVariant(t *testing.T) {
data, _ := os.ReadFile("../../testdata/bootc-status-nvidia.json")
mock := cli.NewMockExecutor()
mock.SetResponse("bootc", []string{"status", "--json"}, data, nil)

status, err := system.GetSystemStatus(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if status.Variant != "nvidia" {
t.Errorf("expected variant 'nvidia', got %q", status.Variant)
}
}

func TestGetSystemStatus_FallsBackToRpmOstree(t *testing.T) {
mock := cli.NewMockExecutor()
mock.SetResponse("bootc", []string{"status", "--json"}, nil, cli.ErrNotInstalled)
mock.SetResponse("rpm-ostree", []string{"status", "--json"}, []byte(`{"deployments":[{"booted":true,"checksum":"abc","origin":"ghcr.io/ublue-os/bluefin:stable"}]}`), nil)

status, err := system.GetSystemStatus(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if status.ImageRef == "" {
t.Error("should parse image ref from rpm-ostree fallback")
}
}

func TestGetVariantInfo_AllVariants(t *testing.T) {
cases := []struct {
imageRef string
want     string
}{
{"ghcr.io/ublue-os/bluefin:stable", "base"},
{"ghcr.io/ublue-os/bluefin-dx:stable", "dx"},
{"ghcr.io/ublue-os/bluefin-nvidia:stable", "nvidia"},
{"ghcr.io/ublue-os/aurora:stable", "aurora"},
{"ghcr.io/ublue-os/aurora-dx:stable", "aurora-dx"},
}
for _, tc := range cases {
t.Run(tc.want, func(t *testing.T) {
got := system.DetectVariant(tc.imageRef)
if got != tc.want {
t.Errorf("DetectVariant(%q) = %q, want %q", tc.imageRef, got, tc.want)
}
})
}
}

// TestGetSystemStatus_StagedUpdate verifies staged image is surfaced when present.
func TestGetSystemStatus_StagedUpdate(t *testing.T) {
stagedJSON := []byte(`{
"status": {
"booted": {
"image": {
"image": {"image": "ghcr.io/ublue-os/bluefin:stable", "transport": "registry"},
"imageDigest": "sha256:booteddigest"
}
},
"staged": {
"image": {
"image": {"image": "ghcr.io/ublue-os/bluefin:stable", "transport": "registry"},
"imageDigest": "sha256:stageddigest"
}
},
"rollback": null
}
}`)
mock := cli.NewMockExecutor()
mock.SetResponse("bootc", []string{"status", "--json"}, stagedJSON, nil)

status, err := system.GetSystemStatus(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if status.Staged == "" {
t.Error("expected staged digest to be non-empty when staged update present")
}
if status.RollbackAvailable {
t.Error("rollback should not be available when rollback is null")
}
}
