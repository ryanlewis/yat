package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_SymlinkedItemsDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeItem(t, filepath.Join(dir, "real"), "a.md", "---\nid: A-1\ntitle: \"Linked item\"\nstatus: todo\n---\n")

	if err := os.Symlink("real", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, dir, "--dir", "link", "list")
	if code != 0 {
		t.Fatalf("list: exit code = %d, stderr:\n%s", code, stderr)
	}

	if !strings.Contains(stdout, "A-1") {
		t.Fatalf("list via a symlinked dir does not show A-1:\n%s", stdout)
	}

	if _, stderr, code := run(t, dir, "--dir", "link", "start", "A-1"); code != 0 {
		t.Fatalf("start: exit code = %d, stderr:\n%s", code, stderr)
	}

	data, err := os.ReadFile(filepath.Join(dir, "real", "a.md"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "status: in-progress") {
		t.Errorf("start did not update the file behind the link:\n%s", data)
	}
}
