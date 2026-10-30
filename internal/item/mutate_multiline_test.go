package item

import (
	"os"
	"path/filepath"
	"testing"
)

// A line inside another key's multi-line quoted value can start with
// "status:". Only the real status key may be rewritten.
func TestSetStatus_LeavesStatusLineInsideQuotedValue(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{
			name: "double quoted title",
			in:   "---\nid: q\ntitle: \"Fix login\nstatus: broken #2 on prod\"\nstatus: todo\n---\nbody\n",
			want: "---\nid: q\ntitle: \"Fix login\nstatus: broken #2 on prod\"\nstatus: done\n---\nbody\n",
		},
		{
			name: "single quoted note",
			in:   "---\nid: q\nnote: 'x\nstatus: y'\nstatus: todo\n---\nbody\n",
			want: "---\nid: q\nnote: 'x\nstatus: y'\nstatus: done\n---\nbody\n",
		},
		{
			name: "status key before the quoted value",
			in:   "---\nid: q\nstatus: todo\ntitle: \"a\nstatus: b # c\"\n---\n",
			want: "---\nid: q\nstatus: done\ntitle: \"a\nstatus: b # c\"\n---\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "q.md")
			if err := os.WriteFile(path, []byte(tt.in), 0o644); err != nil {
				t.Fatal(err)
			}

			if err := SetStatus(path, StatusDone); err != nil {
				t.Fatalf("SetStatus: %v", err)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			if string(got) != tt.want {
				t.Errorf("file =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}
