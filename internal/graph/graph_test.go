package graph

import (
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func makeItems() []*item.Item {
	return []*item.Item{
		{ID: "SP-001", Title: "Spike", Type: "Spike", Priority: item.PriorityCritical, Status: item.StatusDraft, Dependencies: nil},
		{ID: "TK-001", Title: "Scaffolding", Type: "Task", Priority: item.PriorityCritical, Status: item.StatusDraft, Dependencies: nil},
		{ID: "TK-002", Title: "Abstraction", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusDraft, Dependencies: []string{"TK-001", "SP-001"}},
		{ID: "TK-003", Title: "Error handling", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusDraft, Dependencies: []string{"TK-001"}},
		{ID: "ST-001", Title: "Command parsing", Type: "Story", Priority: item.PriorityCritical, Status: item.StatusDraft, Dependencies: []string{"TK-001", "TK-003"}},
		{ID: "ST-002", Title: "Profile parsing", Type: "Story", Priority: item.PriorityHigh, Status: item.StatusDraft, Dependencies: []string{"TK-003", "TK-002"}},
	}
}

func mustBuild(t *testing.T, items []*item.Item) *Graph {
	t.Helper()

	g, err := Build(items)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	return g
}

func TestReady_AllDraft(t *testing.T) {
	g := mustBuild(t, makeItems())
	ready := g.Ready()

	// Only items with no deps should be ready: SP-001 and TK-001
	if len(ready) != 2 {
		ids := make([]string, len(ready))
		for i, r := range ready {
			ids[i] = r.ID
		}
		t.Fatalf("expected 2 ready items, got %d: %v", len(ready), ids)
	}

	// Both are Critical, so sorted by ID
	if ready[0].ID != "SP-001" || ready[1].ID != "TK-001" {
		t.Errorf("ready = [%s, %s], want [SP-001, TK-001]", ready[0].ID, ready[1].ID)
	}
}

func TestReady_AfterComplete(t *testing.T) {
	items := makeItems()
	// Mark TK-001 as done
	items[1].Status = item.StatusDone
	g := mustBuild(t, items)

	ready := g.Ready()

	// SP-001 (no deps, draft), TK-003 (dep TK-001 done)
	ids := make([]string, len(ready))
	for i, r := range ready {
		ids[i] = r.ID
	}

	if len(ready) != 2 {
		t.Fatalf("expected 2 ready, got %d: %v", len(ready), ids)
	}

	// SP-001 is Critical, TK-003 is High
	if ready[0].ID != "SP-001" {
		t.Errorf("first ready = %s, want SP-001", ready[0].ID)
	}
	if ready[1].ID != "TK-003" {
		t.Errorf("second ready = %s, want TK-003", ready[1].ID)
	}
}

func TestBlocked(t *testing.T) {
	g := mustBuild(t, makeItems())
	blocked := g.Blocked()

	// TK-002, TK-003, ST-001, ST-002 should be blocked
	if len(blocked) != 4 {
		ids := make([]string, len(blocked))
		for i, b := range blocked {
			ids[i] = b.ID
		}
		t.Fatalf("expected 4 blocked, got %d: %v", len(blocked), ids)
	}
}

func TestWaitingOn(t *testing.T) {
	g := mustBuild(t, makeItems())

	waiting := g.WaitingOn("ST-002")
	if len(waiting) != 2 {
		t.Fatalf("expected 2 waiting, got %d: %v", len(waiting), waiting)
	}
	if waiting[0] != "TK-002" || waiting[1] != "TK-003" {
		t.Errorf("waiting = %v, want [TK-002, TK-003]", waiting)
	}
}

func TestUnblockedBy(t *testing.T) {
	items := makeItems()
	// Mark TK-001 as done (simulating completing it)
	items[1].Status = item.StatusDone
	g := mustBuild(t, items)

	// Now simulate completing SP-001
	items[0].Status = item.StatusDone

	unblocked := g.UnblockedBy("SP-001")

	// TK-002 depends on TK-001 (done) and SP-001 (now done) -> should be unblocked
	if len(unblocked) != 1 || unblocked[0].ID != "TK-002" {
		ids := make([]string, len(unblocked))
		for i, u := range unblocked {
			ids[i] = u.ID
		}
		t.Errorf("unblocked = %v, want [TK-002]", ids)
	}
}

func TestStillBlocked(t *testing.T) {
	items := makeItems()
	g := mustBuild(t, items)

	// Simulate completing TK-001
	items[1].Status = item.StatusDone

	stillBlocked := g.StillBlocked("TK-001")

	// TK-002 depends on TK-001 (done) and SP-001 (still draft) -> still blocked
	// ST-001 depends on TK-001 (done) and TK-003 (draft) -> still blocked
	if len(stillBlocked) != 2 {
		ids := make([]string, len(stillBlocked))
		for i, b := range stillBlocked {
			ids[i] = b.ID
		}
		t.Fatalf("expected 2 still blocked, got %d: %v", len(stillBlocked), ids)
	}
}

func TestTopologicalLayers(t *testing.T) {
	g := mustBuild(t, makeItems())
	layers := g.TopologicalLayers()

	if len(layers) < 3 {
		t.Fatalf("expected at least 3 layers, got %d", len(layers))
	}

	// Layer 0: SP-001, TK-001 (no deps)
	layer0 := layers[0]
	if len(layer0) != 2 {
		t.Errorf("layer 0: expected 2 items, got %d: %v", len(layer0), layer0)
	}

	// Last layer should have ST-002 (depends on everything)
	lastLayer := layers[len(layers)-1]
	found := false
	for _, id := range lastLayer {
		if id == "ST-002" {
			found = true
		}
	}
	if !found {
		t.Errorf("ST-002 should be in deepest layer, last layer = %v", lastLayer)
	}
}

func TestDeepestLayer(t *testing.T) {
	g := mustBuild(t, makeItems())
	depth, items := g.DeepestLayer()

	if depth < 2 {
		t.Errorf("expected depth >= 2, got %d", depth)
	}

	if len(items) == 0 {
		t.Error("expected items in deepest layer")
	}
}

func TestBuild_UnknownDependency(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Dependencies: nil},
		{ID: "TK-002", Dependencies: []string{"TK-001", "NONEXISTENT"}},
	}

	_, err := Build(items)
	if err == nil {
		t.Fatal("expected error for unknown dependency, got nil")
	}
}

func TestBuild_CyclicDependency(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Dependencies: []string{"TK-002"}},
		{ID: "TK-002", Dependencies: []string{"TK-001"}},
	}

	_, err := Build(items)
	if err == nil {
		t.Fatal("expected error for cyclic dependency, got nil")
	}
}

func TestReady_CustomIsDone(t *testing.T) {
	// "shipped" is also a done status
	isDone := func(s item.Status) bool { return s == "done" || s == "shipped" }
	items := []*item.Item{
		{ID: "A", Status: "shipped", Dependencies: nil},
		{ID: "B", Status: "draft", Dependencies: []string{"A"}},
		{ID: "C", Status: "draft", Dependencies: []string{"B"}},
	}

	g, err := Build(items, isDone)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	ready := g.Ready()
	if len(ready) != 1 || ready[0].ID != "B" {
		ids := make([]string, len(ready))
		for i, r := range ready {
			ids[i] = r.ID
		}
		t.Errorf("ready = %v, want [B]", ids)
	}
}

func TestBlocked_CustomIsDone(t *testing.T) {
	isDone := func(s item.Status) bool { return s == "done" || s == "shipped" }
	items := []*item.Item{
		{ID: "A", Status: "shipped", Dependencies: nil},
		{ID: "B", Status: "draft", Dependencies: []string{"A"}},
		{ID: "C", Status: "draft", Dependencies: []string{"B"}},
	}

	g, err := Build(items, isDone)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	blocked := g.Blocked()
	if len(blocked) != 1 || blocked[0].ID != "C" {
		ids := make([]string, len(blocked))
		for i, b := range blocked {
			ids[i] = b.ID
		}
		t.Errorf("blocked = %v, want [C]", ids)
	}
}

func TestWaitingOn_CustomIsDone(t *testing.T) {
	isDone := func(s item.Status) bool { return s == "closed" }
	items := []*item.Item{
		{ID: "A", Status: "closed", Dependencies: nil},
		{ID: "B", Status: "open", Dependencies: nil},
		{ID: "C", Status: "open", Dependencies: []string{"A", "B"}},
	}

	g, err := Build(items, isDone)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	waiting := g.WaitingOn("C")
	if len(waiting) != 1 || waiting[0] != "B" {
		t.Errorf("waiting = %v, want [B]", waiting)
	}
}

func TestBuild_LongCycle(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Dependencies: []string{"C"}},
		{ID: "B", Dependencies: []string{"A"}},
		{ID: "C", Dependencies: []string{"B"}},
		{ID: "D", Dependencies: nil},
	}

	_, err := Build(items)
	if err == nil {
		t.Fatal("expected error for cyclic dependency, got nil")
	}
}
