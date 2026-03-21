package system

import (
	"context"
	"strings"

	"github.com/projectbluefin/bluefin-mcp/internal/cli"
)

// Recipe represents a ujust recipe with its name and optional description.
type Recipe struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListRecipes returns all available ujust recipes.
func ListRecipes(ctx context.Context, runner cli.CommandRunner) ([]Recipe, error) {
	out, err := runner.Run(ctx, "ujust", []string{"--list"})
	if err != nil {
		return nil, err
	}
	return parseUjustList(string(out)), nil
}

func parseUjustList(output string) []Recipe {
	var recipes []Recipe
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Available") {
			continue
		}
		// Format: "    recipe-name    # description"
		var name, desc string
		if idx := strings.Index(line, "#"); idx >= 0 {
			name = strings.TrimSpace(line[:idx])
			desc = strings.TrimSpace(line[idx+1:])
		} else {
			name = strings.TrimSpace(line)
		}
		if name == "" {
			continue
		}
		recipes = append(recipes, Recipe{Name: name, Description: desc})
	}
	return recipes
}
