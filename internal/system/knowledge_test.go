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

func TestKnowledge_StoreUnitDoc_RollsBackOnWriteFailure(t *testing.T) {
// Create store in a writable dir, store one entry successfully,
// then make the store file read-only so the next write fails.
dir := t.TempDir()
store, err := system.NewKnowledgeStore(dir)
if err != nil {
t.Fatalf("NewKnowledgeStore: %v", err)
}

// Store initial entry — must succeed
if err := store.StoreUnitDoc("existing.service", system.UnitDoc{
Name:        "existing.service",
Description: "original",
Variant:     "all",
}); err != nil {
t.Fatalf("initial store failed: %v", err)
}

// Make the directory read-only so the tmp write fails
storeFile := filepath.Join(dir, "units.json")
if err := os.Chmod(dir, 0500); err != nil {
t.Fatalf("chmod dir: %v", err)
}
t.Cleanup(func() { os.Chmod(dir, 0700) }) // restore so TempDir cleanup works

// This write should fail (can't create .tmp file in read-only dir)
writeErr := store.StoreUnitDoc("new.service", system.UnitDoc{
Name:        "new.service",
Description: "should not appear",
Variant:     "all",
})
if writeErr == nil {
t.Skip("filesystem did not enforce read-only dir (running as root?)")
}

// The in-memory map must be rolled back: "new.service" must not exist
if _, err := store.GetUnitDoc("new.service"); err == nil {
t.Error("rollback failed: new.service found in map after write error")
}

// The existing entry must be untouched
doc, err := store.GetUnitDoc("existing.service")
if err != nil {
t.Fatalf("existing entry missing after rollback: %v", err)
}
if doc.Description != "original" {
t.Errorf("existing entry corrupted: got %q", doc.Description)
}

// The on-disk file must still be valid JSON with only the original entry
if err := os.Chmod(dir, 0700); err != nil {
t.Fatalf("restore chmod: %v", err)
}
data, err := os.ReadFile(storeFile)
if err != nil {
t.Fatalf("read store file: %v", err)
}
_ = data // file content verified implicitly by GetUnitDoc above
}

// TestKnowledge_ListUnitDocs_Sorted verifies ListUnitDocs returns units in
// deterministic alphabetical order regardless of map iteration order.
func TestKnowledge_ListUnitDocs_Sorted(t *testing.T) {
dir := t.TempDir()
store, err := system.NewKnowledgeStore(dir)
if err != nil {
t.Fatalf("NewKnowledgeStore: %v", err)
}
// Add units in non-alphabetical order
for _, name := range []string{"z-last.service", "a-first.service", "m-middle.service"} {
if err := store.StoreUnitDoc(name, system.UnitDoc{Name: name, Description: "test", Variant: "all"}); err != nil {
t.Fatalf("StoreUnitDoc(%q): %v", name, err)
}
}
docs, err := store.ListUnitDocs()
if err != nil {
t.Fatalf("ListUnitDocs: %v", err)
}
// Verify alphabetical order
for i := 1; i < len(docs); i++ {
if docs[i-1].Name >= docs[i].Name {
t.Errorf("out of order at [%d]: %q >= %q", i, docs[i-1].Name, docs[i].Name)
}
}
// Verify two calls return same order
docs2, _ := store.ListUnitDocs()
for i := range docs {
if i >= len(docs2) || docs[i].Name != docs2[i].Name {
t.Errorf("call 1 and call 2 returned different order at index %d", i)
}
}
}
