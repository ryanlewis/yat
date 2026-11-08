package item

import (
	"slices"
	"strings"
	"testing"
)

func TestKeyOffsets_MatchNodeOffsetPerKey(t *testing.T) {
	fixed := []string{
		"id: a\nstatus: todo\n",
		"id: q\r\ntitle: \"a\r\nstatus: b # c\"\r\nstatus: todo\r\n",
		"id: q\ntitle: \"a\nstate: b # c\"\nstate: todo\n",
		"id: q\ntitle: 'multi\n\n  line'\nstatus: todo\n",
		"id: a\n{x: 1, y: 2}: v\nstatus: todo\n",
		"{id: a, status: todo, title: t}\n",
		"# lead comment\nid: a # trailing\n\n# between\nstatus: todo\n",
		"id: é\ntitle: \"日本語\"\nstatus: todo\n",
		"id: a\rstatus: todo\r",
		"id: a\u0085status: todo title: t ",
		"~: x\n5: y\nstatus: todo\n",
		"id: a\nnotes: |\n  status: fake\n  more\nstatus: todo\n",
		"id: a\nstatus:\n",
		"id: a\nstatus: todo",
	}

	for _, fm := range slices.Concat(fixed, generatedFrontmatters()) {
		pairs := topLevelPairs([]byte(fm))
		if pairs == nil {
			continue
		}

		got := keyOffsets([]byte(fm), pairs)
		if len(got) != len(pairs)/2 {
			t.Fatalf("%q: got %d offsets for %d keys", fm, len(got), len(pairs)/2)
		}

		for i := 0; i+1 < len(pairs); i += 2 {
			want, ok := nodeOffset([]byte(fm), pairs[i].Line, pairs[i].Column)
			if !ok {
				want = -1
			}

			if got[i/2] != want {
				t.Errorf("%q key %d (%s): offset %d, nodeOffset %d", fm, i/2, pairs[i].Value, got[i/2], want)
			}
		}
	}
}

// generatedFrontmatters returns many-key frontmatter with mixed line endings
// and quoted multi-line values.
func generatedFrontmatters() []string {
	out := make([]string, 0, 3)

	for _, nl := range []string{"\n", "\r\n", "\r"} {
		var b strings.Builder

		for i := range 20 {
			b.WriteString("k" + string(rune('a'+i)) + ": \"v" + nl + "continued\"" + nl)
			b.WriteString("# c" + nl + "s" + string(rune('a'+i)) + ": é # x" + nl)
		}

		out = append(out, b.String())
	}

	return out
}
