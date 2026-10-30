package item

import "testing"

func TestParseWithOptions_InvalidStatusListsValidInOrder(t *testing.T) {
	data := []byte("---\nid: TK-001\ntitle: T\nstatus: bogus\n---\n")
	opts := ParseOptions{ValidStatuses: NewStatusSet([]string{"zeta", "alpha", "mid", "beta", "omega", "delta"})}

	want := `invalid status "bogus" in x.md: must be one of [alpha beta delta mid omega zeta]`

	_, err := ParseWithOptions(data, "x.md", opts)
	if err == nil {
		t.Fatal("expected error")
	}

	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}
