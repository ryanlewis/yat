package graph

import (
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

// Items that only depend on a cycle, or sit between two cycles, are not part
// of one and must not be named in the cycle error.
func TestBuild_CycleErrorNamesOnlyCycleMembers(t *testing.T) {
	tests := []struct {
		name  string
		items []*item.Item
		want  string
	}{
		{
			name: "downstream of a two-node cycle",
			items: []*item.Item{
				{ID: "a", Dependencies: []string{"b"}},
				{ID: "b", Dependencies: []string{"a"}},
				{ID: "z", Dependencies: []string{"a"}},
				{ID: "z2", Dependencies: []string{"z"}},
			},
			want: "dependency cycle detected involving: a, b",
		},
		{
			name: "self-loop with a dependent",
			items: []*item.Item{
				{ID: "s", Dependencies: []string{"s"}},
				{ID: "t", Dependencies: []string{"s"}},
			},
			want: "dependency cycle detected involving: s",
		},
		{
			name: "between two cycles",
			items: []*item.Item{
				{ID: "a", Dependencies: []string{"b"}},
				{ID: "b", Dependencies: []string{"a"}},
				{ID: "m", Dependencies: []string{"a"}},
				{ID: "c", Dependencies: []string{"d", "m"}},
				{ID: "d", Dependencies: []string{"c"}},
			},
			want: "dependency cycle detected involving: a, b, c, d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Build(tt.items)
			if err == nil {
				t.Fatal("expected a cycle error")
			}

			if err.Error() != tt.want {
				t.Errorf("error = %q, want %q", err, tt.want)
			}
		})
	}
}
