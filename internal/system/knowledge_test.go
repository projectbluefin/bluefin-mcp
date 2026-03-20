package system_test

import (
"os"
"path/filepath"
"sync"
"testing"

"github.com/projectbluefin/bluefin-mcp/internal/system"
)

func TestKnowledge_SeedLoaded(t *testing.T) {
dir := t.TempDir()
store, err := system.NewKnowledgeStore(dir)
if err != nil {
t.Fatalf("unexpected error: %v", err)
}

// Pre-populated unit must be present without any store_unit_docs call
doc, err := store.GetUnitDoc("flatpak-nuke-fedora.service")
if err != nil {
t.Fatalf("expected pre-seeded doc, got error: %v", err)
}
if doc.Description == "" {
t.Error("pre-seeded description should not be empty")
}
if doc.Variant == "" {
t.Error("pre-seeded variant should not be empty")
}
}

func TestKnowledge_StoreAndRetrieve(t *testing.T) {
dir := t.TempDir()
store, _ := system.NewKnowledgeStore(dir)

err := store.StoreUnitDoc("test.service", system.UnitDoc{
Name:        "test.service",
Description: "A test service",
Variant:     "all",
})
if err != nil {
t.Fatalf("store failed: %v", err)
}

doc, err := store.GetUnitDoc("test.service")
if err != nil {
t.Fatalf("retrieve failed: %v", err)
}
if doc.Description != "A test service" {
t.Errorf("got %q, want 'A test service'", doc.Description)
}
}

func TestKnowledge_ListAll(t *testing.T) {
dir := t.TempDir()
store, _ := system.NewKnowledgeStore(dir)

docs, err := store.ListUnitDocs()
if err != nil {
t.Fatalf("list failed: %v", err)
}
// Should have all seeded units (seed has 10)
if len(docs) < 10 {
t.Errorf("expected at least 10 seeded units, got %d", len(docs))
}
}

func TestKnowledge_GetNonExistent_ReturnsError(t *testing.T) {
dir := t.TempDir()
store, _ := system.NewKnowledgeStore(dir)

_, err := store.GetUnitDoc("does-not-exist.service")
if err == nil {
t.Fatal("expected error when retrieving non-existent unit doc")
}
}

func TestKnowledge_InvalidUnitName(t *testing.T) {
dir := t.TempDir()
store, _ := system.NewKnowledgeStore(dir)

err := store.StoreUnitDoc("../evil-path", system.UnitDoc{Description: "bad"})
if err == nil {
t.Fatal("expected error for invalid unit name, got nil")
}
}

func TestKnowledge_InvalidUnitName_AbsolutePath(t *testing.T) {
dir := t.TempDir()
store, _ := system.NewKnowledgeStore(dir)

err := store.StoreUnitDoc("/etc/passwd", system.UnitDoc{Description: "bad"})
if err == nil {
t.Fatal("expected error for absolute path unit name, got nil")
}
}

func TestKnowledge_ConcurrentWrites_NoCorruption(t *testing.T) {
dir := t.TempDir()
store, _ := system.NewKnowledgeStore(dir)

var wg sync.WaitGroup
for i := 0; i < 20; i++ {
wg.Add(1)
go func(n int) {
defer wg.Done()
name := "test-concurrent-" + string(rune('a'+n)) + ".service"
store.StoreUnitDoc(name, system.UnitDoc{
Name:        name,
Description: "concurrent write test",
Variant:     "all",
})
}(i)
}
wg.Wait()

// Store file must still be valid JSON (not corrupted)
data, err := os.ReadFile(filepath.Join(dir, "units.json"))
if err != nil {
t.Fatalf("store file missing: %v", err)
}
if len(data) == 0 {
t.Error("store file empty after concurrent writes")
}
}

func TestKnowledge_StoreOverwritesExisting(t *testing.T) {
dir := t.TempDir()
store, _ := system.NewKnowledgeStore(dir)

store.StoreUnitDoc("overwrite.service", system.UnitDoc{
Name:        "overwrite.service",
Description: "original description",
Variant:     "all",
})
store.StoreUnitDoc("overwrite.service", system.UnitDoc{
Name:        "overwrite.service",
Description: "updated description",
Variant:     "dx",
})

doc, err := store.GetUnitDoc("overwrite.service")
if err != nil {
t.Fatalf("retrieve failed: %v", err)
}
if doc.Description != "updated description" {
t.Errorf("expected updated description, got %q", doc.Description)
}
if doc.Variant != "dx" {
t.Errorf("expected variant 'dx', got %q", doc.Variant)
}
}

func TestKnowledge_SeedContainsKnownDXUnit(t *testing.T) {
dir := t.TempDir()
store, _ := system.NewKnowledgeStore(dir)

doc, err := store.GetUnitDoc("bluefin-dx-groups.service")
if err != nil {
t.Fatalf("expected dx unit in seed: %v", err)
}
if doc.Variant != "dx" {
t.Errorf("expected variant 'dx' for bluefin-dx-groups.service, got %q", doc.Variant)
}
}
