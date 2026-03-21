package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

	"github.com/projectbluefin/bluefin-mcp/internal/seed"
)

// unitNameRe validates systemd unit names. Accepts names like test-concurrent-a.service.
var unitNameRe = regexp.MustCompile(`^[\w@][\w@\-\.\:]*\.(service|timer|socket|target|mount|path|slice)$`)

// UnitDoc holds semantic documentation for a Bluefin custom systemd unit.
type UnitDoc struct {
	Name        string `json:"name"`
	Variant     string `json:"variant"`
	Description string `json:"description"`
}

type store struct {
	Version string             `json:"version"`
	Units   map[string]UnitDoc `json:"units"`
}

// KnowledgeStore is a thread-safe persistent store for unit documentation.
type KnowledgeStore struct {
	mu   sync.Mutex
	path string
	data store
}

// NewKnowledgeStore creates (or loads) a KnowledgeStore from the given directory,
// pre-populated with the embedded seed units.
func NewKnowledgeStore(dir string) (*KnowledgeStore, error) {
	ks := &KnowledgeStore{
		path: filepath.Join(dir, "units.json"),
		data: store{Version: "1", Units: make(map[string]UnitDoc)},
	}

	// Load seed data
	var seedData store
	if err := json.Unmarshal(seed.Units, &seedData); err != nil {
		return nil, fmt.Errorf("corrupt embedded seed data: %w", err)
	}
	for k, v := range seedData.Units {
		ks.data.Units[k] = v
	}

	// Load existing store — user additions override seed
	if data, err := os.ReadFile(ks.path); err == nil {
		var existing store
		if json.Unmarshal(data, &existing) == nil {
			for k, v := range existing.Units {
				ks.data.Units[k] = v
			}
		}
	}

	return ks, nil
}

// GetUnitDoc returns the documentation for the named unit, or an error if not found.
func (ks *KnowledgeStore) GetUnitDoc(name string) (*UnitDoc, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	doc, ok := ks.data.Units[name]
	if !ok {
		return nil, fmt.Errorf("unit %q not found", name)
	}
	return &doc, nil
}

// StoreUnitDoc validates the unit name and persists the documentation atomically.
func (ks *KnowledgeStore) StoreUnitDoc(name string, doc UnitDoc) error {
	if !unitNameRe.MatchString(name) {
		return fmt.Errorf("invalid unit name: %q", name)
	}
	ks.mu.Lock()
	defer ks.mu.Unlock()
	old, hadOld := ks.data.Units[name]
	ks.data.Units[name] = doc
	if err := ks.writeLocked(); err != nil {
		// Rollback: restore previous in-memory state to keep map and disk consistent
		if hadOld {
			ks.data.Units[name] = old
		} else {
			delete(ks.data.Units, name)
		}
		return err
	}
	return nil
}

// ListUnitDocs returns all documented units sorted by name for deterministic output.
func (ks *KnowledgeStore) ListUnitDocs() ([]UnitDoc, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	docs := make([]UnitDoc, 0, len(ks.data.Units))
	for _, d := range ks.data.Units {
		docs = append(docs, d)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Name < docs[j].Name })
	return docs, nil
}

// writeLocked writes the store to disk atomically (must hold ks.mu).
func (ks *KnowledgeStore) writeLocked() error {
	data, err := json.MarshalIndent(ks.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := ks.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, ks.path)
}
