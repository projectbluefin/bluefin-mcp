package system_test

import (
"context"
"os"
"testing"

"github.com/projectbluefin/bluefin-mcp/internal/cli"
"github.com/projectbluefin/bluefin-mcp/internal/system"
)

func TestListRecipes_ParsesGoldenFile(t *testing.T) {
data, err := os.ReadFile("../../testdata/ujust-list.txt")
if err != nil {
t.Fatalf("missing testdata: %v", err)
}
mock := cli.NewMockExecutor()
mock.SetResponse("ujust", []string{"--list"}, data, nil)

recipes, err := system.ListRecipes(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if len(recipes) == 0 {
t.Fatal("expected at least one recipe, got none")
}

// Verify known recipes present
names := make(map[string]bool)
for _, r := range recipes {
names[r.Name] = true
}
for _, expected := range []string{"update", "changelogs", "check-local-overrides"} {
if !names[expected] {
t.Errorf("expected recipe %q not found in output", expected)
}
}
}

func TestListRecipes_EmptyOutput_ReturnsEmptySlice(t *testing.T) {
mock := cli.NewMockExecutor()
mock.SetResponse("ujust", []string{"--list"}, []byte(""), nil)

recipes, err := system.ListRecipes(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if len(recipes) != 0 {
t.Errorf("expected empty slice, got %d recipes", len(recipes))
}
}

func TestListRecipes_UjustNotInstalled_ReturnsError(t *testing.T) {
mock := cli.NewMockExecutor()
mock.SetResponse("ujust", []string{"--list"}, nil, cli.ErrNotInstalled)

_, err := system.ListRecipes(context.Background(), mock)
if err == nil {
t.Fatal("expected error when ujust not installed")
}
}

// TestListRecipes_RecipeHasDescription verifies descriptions are parsed from the # comment.
func TestListRecipes_RecipeHasDescription(t *testing.T) {
data, _ := os.ReadFile("../../testdata/ujust-list.txt")
mock := cli.NewMockExecutor()
mock.SetResponse("ujust", []string{"--list"}, data, nil)

recipes, err := system.ListRecipes(context.Background(), mock)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}

for _, r := range recipes {
if r.Name == "update" {
if r.Description == "" {
t.Error("expected description for 'update' recipe, got empty string")
}
return
}
}
t.Error("recipe 'update' not found")
}

// TestNoExecuteRecipe documents the design decision: execute_recipe was removed.
// The server is fully read-only. If someone adds execute functionality,
// this test should be updated to reflect that decision explicitly.
func TestNoExecuteRecipe(t *testing.T) {
t.Log("execute_recipe intentionally absent — server is read-only")
}
