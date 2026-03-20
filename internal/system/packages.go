package system

import (
	"context"
	"errors"
	"strings"

	"github.com/projectbluefin/bluefin-mcp/internal/cli"
)

// FlatpakApp represents an installed Flatpak application.
type FlatpakApp struct {
	AppID   string
	Version string
}

// BrewPackage represents an installed Homebrew package.
type BrewPackage struct {
	Name    string
	Version string
}

// GetFlatpakList returns all installed Flatpak applications.
func GetFlatpakList(ctx context.Context, runner cli.CommandRunner) ([]FlatpakApp, error) {
	out, err := runner.Run(ctx, "flatpak", []string{"list", "--columns=application,version"})
	if err != nil {
		return nil, err
	}
	return parseFlatpakList(string(out)), nil
}

func parseFlatpakList(output string) []FlatpakApp {
	var apps []FlatpakApp
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		apps = append(apps, FlatpakApp{AppID: fields[0], Version: fields[1]})
	}
	return apps
}

// GetBrewPackages returns all installed Homebrew packages. Returns nil, nil if brew is not installed.
func GetBrewPackages(ctx context.Context, runner cli.CommandRunner) ([]BrewPackage, error) {
	out, err := runner.Run(ctx, "brew", []string{"list", "--versions"})
	if err != nil {
		if errors.Is(err, cli.ErrNotInstalled) {
			return nil, nil // graceful degrade
		}
		return nil, err
	}
	return parseBrewList(string(out)), nil
}

func parseBrewList(output string) []BrewPackage {
	var pkgs []BrewPackage
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pkgs = append(pkgs, BrewPackage{Name: fields[0], Version: fields[1]})
	}
	return pkgs
}
