package item

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file path")
	}

	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata")
}

func TestLoadAll(t *testing.T) {
	dir := testdataDir(t)

	items, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// Should find SP-001, TK-001, TK-002, TK-003, ST-001, ST-002
	// SPEC.md should be skipped (no id field)
	if len(items) != 6 {
		ids := make([]string, len(items))
		for i, item := range items {
			ids[i] = item.ID
		}
		t.Fatalf("expected 6 items, got %d: %v", len(items), ids)
	}
}

func TestLoadAllIndex(t *testing.T) {
	dir := testdataDir(t)

	items, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx, err := Index(items)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if _, ok := idx["TK-001"]; !ok {
		t.Error("TK-001 not found in index")
	}

	if _, ok := idx["SP-001"]; !ok {
		t.Error("SP-001 not found in index")
	}

	// SPEC.md should not be in the index
	for id := range idx {
		if id == "" {
			t.Error("empty ID found in index")
		}
	}
}
