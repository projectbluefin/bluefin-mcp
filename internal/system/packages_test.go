package system_test

import (
"context"
"os"
"testing"

"github.com/projectbluefin/bluefin-mcp/internal/cli"
"github.com/projectbluefin/bluefin-mcp/internal/system"
)

func TestGetFlatpakList_ParsesOutput(t *testing.T) {
data, err := os.ReadFile("../../testdata/flatpak-list.txt")
if err != nil {
t.Fatalf("missing testdata: %v", err)
}
mock := cli.NewMockExecutor()
mock.SetResponse("flatpak", []string{"list", "--columns=application,version"}, data, nil)

apps, err := system.GetFlatpakList(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if len(apps) == 0 {
t.Fatal("expected at least one app")
}
// Verify a well-known Bluefin flatpak is present (sourced from system-flatpaks.Brewfile).
// Version is "1.0.0" — a placeholder because the Brewfile does not pin versions;
// the refresh script sets this explicitly so tests remain deterministic.
found := false
for _, a := range apps {
if a.AppID == "org.mozilla.firefox" {
found = true
if a.Version != "1.0.0" {
t.Errorf("expected version 1.0.0 (placeholder from source Brewfile), got %q", a.Version)
}
}
}
if !found {
t.Error("expected org.mozilla.firefox in flatpak list")
}
}

func TestGetFlatpakList_AllAppsPresent(t *testing.T) {
data, _ := os.ReadFile("../../testdata/flatpak-list.txt")
mock := cli.NewMockExecutor()
mock.SetResponse("flatpak", []string{"list", "--columns=application,version"}, data, nil)

apps, err := system.GetFlatpakList(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
// Testdata is auto-generated from system-flatpaks.Brewfile in projectbluefin/common.
// Require at least the historical minimum; the exact count varies as upstream adds/removes apps.
const minExpectedApps = 10
if len(apps) < minExpectedApps {
t.Errorf("expected at least %d apps from system-flatpaks.Brewfile, got %d", minExpectedApps, len(apps))
}
}

func TestGetBrewPackages_ParsesOutput(t *testing.T) {
data, err := os.ReadFile("../../testdata/brew-list.txt")
if err != nil {
t.Fatalf("missing testdata: %v", err)
}
mock := cli.NewMockExecutor()
mock.SetResponse("brew", []string{"list", "--versions"}, data, nil)
mock.SetResponse("brew", []string{"outdated", "--json"}, []byte("{}"), nil)

pkgs, err := system.GetBrewPackages(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if len(pkgs) == 0 {
t.Fatal("expected at least one package")
}
}

func TestGetBrewPackages_KnownPackageVersion(t *testing.T) {
data, _ := os.ReadFile("../../testdata/brew-list.txt")
mock := cli.NewMockExecutor()
mock.SetResponse("brew", []string{"list", "--versions"}, data, nil)
mock.SetResponse("brew", []string{"outdated", "--json"}, []byte("{}"), nil)

pkgs, err := system.GetBrewPackages(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}

found := false
for _, p := range pkgs {
if p.Name == "ripgrep" {
found = true
if p.Version != "14.0.3" {
t.Errorf("expected ripgrep version 14.0.3, got %q", p.Version)
}
}
}
if !found {
t.Error("expected ripgrep in brew package list")
}
}

func TestGetBrewPackages_NotInstalled_DegradeGracefully(t *testing.T) {
mock := cli.NewMockExecutor()
mock.SetResponse("brew", []string{"list", "--versions"}, nil, cli.ErrNotInstalled)

result, err := system.GetBrewPackages(context.Background(), mock)
if err != nil {
t.Fatalf("expected graceful degrade, got: %v", err)
}
if len(result) != 0 {
t.Error("expected empty result when brew not installed")
}
}

// TestNoRootPackageManager documents the design law: no dnf/rpm/apt.
// Root FS is read-only. Only flatpak, brew, ujust are valid package managers.
func TestNoRootPackageManager(t *testing.T) {
t.Log("design law: no dnf/rpm/apt — root FS is read-only, use flatpak/brew/ujust")
}
