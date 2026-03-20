package system

import (
	"context"
	"errors"
	"strings"

	"github.com/projectbluefin/bluefin-mcp/internal/cli"
)

// DistroboxEntry represents a single Distrobox container.
type DistroboxEntry struct {
	ID     string
	Name   string
	Status string
	Image  string
}

// ListDistrobox returns all Distrobox containers. Returns nil, nil if distrobox is not installed.
func ListDistrobox(ctx context.Context, runner cli.CommandRunner) ([]DistroboxEntry, error) {
	out, err := runner.Run(ctx, "distrobox", []string{"list"})
	if err != nil {
		if errors.Is(err, cli.ErrNotInstalled) {
			return nil, nil // graceful degrade
		}
		return nil, err
	}
	return parseDistroboxList(string(out)), nil
}

func parseDistroboxList(output string) []DistroboxEntry {
	var entries []DistroboxEntry
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "ID") {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, DistroboxEntry{
			ID:     strings.TrimSpace(parts[0]),
			Name:   strings.TrimSpace(parts[1]),
			Status: strings.TrimSpace(parts[2]),
			Image:  strings.TrimSpace(parts[3]),
		})
	}
	return entries
}
