package item

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")

	if err := CreateFile(path, []byte("one"), 0o600); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}

	if err := CreateFile(path, []byte("two"), 0o600); !errors.Is(err, ErrFileExists) {
		t.Errorf("second CreateFile = %v, want ErrFileExists", err)
	}

	if data, _ := os.ReadFile(path); string(data) != "one" {
		t.Errorf("file = %q, want it unchanged", data)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("want only new.md left, got %d entries", len(entries))
	}
}
