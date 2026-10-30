package item

import (
	"os"
	"path/filepath"
	"testing"
)

// linkedID is the item that symlinkedDir writes.
const linkedID = "A-1"

// symlinkedDir makes real/a.md holding item A-1 and a symlink link -> real,
// and returns the path of the symlink, without a trailing separator.
func symlinkedDir(t *testing.T) string {
	t.Helper()

	base := t.TempDir()
	target := filepath.Join(base, "real")

	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(target, "a.md"), "---\nid: "+linkedID+"\n---\n")

	link := filepath.Join(base, "link")
	if err := os.Symlink("real", link); err != nil {
		t.Fatal(err)
	}

	return link
}

func TestLoadAll_SymlinkedRootIsFollowed(t *testing.T) {
	link := symlinkedDir(t)

	items, _, err := LoadAll(link)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if len(items) != 1 || items[0].ID != linkedID {
		t.Fatalf("items = %v, want one item %s", items, linkedID)
	}

	// Paths stay under the directory as given, so writes go through the link.
	if want := filepath.Join(link, "a.md"); items[0].FilePath != want {
		t.Errorf("FilePath = %q, want %q", items[0].FilePath, want)
	}
}

func TestLoadAll_SymlinkedRootWithTrailingSeparator(t *testing.T) {
	link := symlinkedDir(t)

	items, _, err := LoadAll(link + string(filepath.Separator))
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if len(items) != 1 || items[0].FilePath != filepath.Join(link, "a.md") {
		t.Fatalf("items = %v, want one item at %s", items, filepath.Join(link, "a.md"))
	}
}

func TestLoadAll_SymlinkedRootUnreadableSubdirIsSkipped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read any directory")
	}

	link := symlinkedDir(t)

	locked := filepath.Join(link, "locked")
	if err := os.Mkdir(locked, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) })

	items, warnings, err := loadWithWarnings(t, link)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if len(items) != 1 || items[0].ID != linkedID {
		t.Fatalf("expected only %s to load, got %v", linkedID, items)
	}

	want := "warning: skipping " + locked + ": permission denied\n"
	if warnings != want {
		t.Errorf("warnings = %q, want %q", warnings, want)
	}
}

func TestLoadAll_UnreadableSymlinkedRootFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read any directory")
	}

	link := symlinkedDir(t)
	target := filepath.Join(filepath.Dir(link), "real")

	if err := os.Chmod(target, 0); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chmod(target, 0o750) })

	if _, _, err := loadWithWarnings(t, link); err == nil {
		t.Fatal("LoadAll of an unreadable root returned no error")
	}
}
