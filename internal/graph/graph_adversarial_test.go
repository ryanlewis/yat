package graph

import (
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

// --- Self-loops ---

func TestBuild_SelfLoop(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Dependencies: []string{"A"}},
	}
	_, err := Build(items)
	if err == nil {
		t.Fatal("expected error for self-loop")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected 'cycle' in error, got: %v", err)
	}
}

// --- Single item graph ---

func TestBuild_SingleItem(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
	}
	g, err := Build(items)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	ready := g.Ready()
	if len(ready) != 1 || ready[0].ID != "A" {
		t.Errorf("Ready() = %v, want [A]", ready)
	}

	blocked := g.Blocked()
	if len(blocked) != 0 {
		t.Errorf("Blocked() = %v, want []", blocked)
	}

	layers := g.TopologicalLayers()
	if len(layers) != 1 || len(layers[0]) != 1 || layers[0][0] != "A" {
		t.Errorf("TopologicalLayers() = %v, want [[A]]", layers)
	}
}

// --- Empty graph ---

func TestBuild_EmptyGraph(t *testing.T) {
	g, err := Build([]*item.Item{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	ready := g.Ready()
	if len(ready) != 0 {
		t.Errorf("Ready() = %v, want []", ready)
	}

	blocked := g.Blocked()
	if len(blocked) != 0 {
		t.Errorf("Blocked() = %v, want []", blocked)
	}

	layers := g.TopologicalLayers()
	if len(layers) != 0 {
		t.Errorf("TopologicalLayers() = %v, want []", layers)
	}

	depth, items := g.DeepestLayer()
	if depth != 0 || items != nil {
		t.Errorf("DeepestLayer() = (%d, %v), want (0, nil)", depth, items)
	}
}

// --- All items done ---

func TestReady_AllDone(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusDone},
		{ID: "B", Status: item.StatusDone, Dependencies: []string{"A"}},
	}
	g := mustBuild(t, items)
	ready := g.Ready()
	if len(ready) != 0 {
		t.Errorf("Ready() should be empty when all done, got %d items", len(ready))
	}
}

// --- All items in-progress ---

func TestReady_AllInProgress(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusInProgress},
		{ID: "B", Status: item.StatusInProgress, Dependencies: []string{"A"}},
	}
	g := mustBuild(t, items)

	// A is in-progress with no deps → ready (status != done and all deps done)
	// B is in-progress but dep A is not done → blocked
	ready := g.Ready()
	if len(ready) != 1 || ready[0].ID != "A" {
		ids := make([]string, len(ready))
		for i, r := range ready {
			ids[i] = r.ID
		}
		t.Errorf("Ready() = %v, want [A]", ids)
	}
}

// --- Long linear chain ---

func TestBuild_LongLinearChain(t *testing.T) {
	const chainLen = 50
	items := make([]*item.Item, chainLen)
	for i := range chainLen {
		it := &item.Item{
			ID:     string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Status: item.StatusTodo,
		}
		if i > 0 {
			it.Dependencies = []string{items[i-1].ID}
		}
		items[i] = it
	}

	g, err := Build(items)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	layers := g.TopologicalLayers()
	if len(layers) != chainLen {
		t.Errorf("expected %d layers for linear chain, got %d", chainLen, len(layers))
	}

	// Only first item should be ready
	ready := g.Ready()
	if len(ready) != 1 || ready[0].ID != items[0].ID {
		t.Errorf("only first item should be ready, got %d items", len(ready))
	}
}

// --- Wide graph (many roots) ---

func TestBuild_WideGraph(t *testing.T) {
	const width = 100
	items := make([]*item.Item, width)
	for i := range width {
		items[i] = &item.Item{
			ID:     "ITEM-" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Status: item.StatusTodo,
		}
	}

	g, err := Build(items)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// All items are roots — single layer
	layers := g.TopologicalLayers()
	if len(layers) != 1 {
		t.Errorf("expected 1 layer for wide graph, got %d", len(layers))
	}
	if len(layers[0]) != width {
		t.Errorf("layer 0 should have %d items, got %d", width, len(layers[0]))
	}

	// All items ready
	ready := g.Ready()
	if len(ready) != width {
		t.Errorf("all %d items should be ready, got %d", width, len(ready))
	}
}

// --- Diamond dependency ---

func TestBuild_DiamondDependency(t *testing.T) {
	//    A
	//   / \
	//  B   C
	//   \ /
	//    D
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
		{ID: "B", Status: item.StatusTodo, Dependencies: []string{"A"}},
		{ID: "C", Status: item.StatusTodo, Dependencies: []string{"A"}},
		{ID: "D", Status: item.StatusTodo, Dependencies: []string{"B", "C"}},
	}

	g := mustBuild(t, items)

	// Only A is ready
	ready := g.Ready()
	if len(ready) != 1 || ready[0].ID != "A" {
		t.Error("only A should be ready")
	}

	// Complete A → B and C become ready
	items[0].Status = item.StatusDone
	unblocked := g.UnblockedBy("A")
	if len(unblocked) != 2 {
		t.Errorf("completing A should unblock B and C, got %d", len(unblocked))
	}

	// D is still blocked (needs both B and C)
	stillBlocked := g.StillBlocked("A")
	if len(stillBlocked) != 0 {
		// D doesn't directly depend on A, so it shouldn't appear here
		// Actually D depends on B and C, not A. So StillBlocked("A") checks
		// dependents of A, which are B and C. Since both B and C have all OTHER
		// deps done (A is done), they are unblocked, not still blocked.
		t.Errorf("StillBlocked(A) = %d, want 0", len(stillBlocked))
	}

	// Now complete B — D is still blocked by C
	items[1].Status = item.StatusDone
	stillBlocked = g.StillBlocked("B")
	if len(stillBlocked) != 1 || stillBlocked[0].ID != "D" {
		t.Error("D should still be blocked after completing only B")
	}

	// Complete C — D becomes ready
	items[2].Status = item.StatusDone
	unblocked = g.UnblockedBy("C")
	if len(unblocked) != 1 || unblocked[0].ID != "D" {
		t.Error("D should be unblocked after completing C")
	}
}

// --- Multiple disjoint cycles ---

func TestBuild_MultipleDisjointCycles(t *testing.T) {
	items := []*item.Item{
		// Cycle 1: A <-> B
		{ID: "A", Dependencies: []string{"B"}},
		{ID: "B", Dependencies: []string{"A"}},
		// Cycle 2: C <-> D
		{ID: "C", Dependencies: []string{"D"}},
		{ID: "D", Dependencies: []string{"C"}},
		// Non-cyclic
		{ID: "E", Dependencies: nil},
	}

	_, err := Build(items)
	if err == nil {
		t.Fatal("expected error for cycles")
	}
	// All 4 cyclic items should be listed
	for _, id := range []string{"A", "B", "C", "D"} {
		if !strings.Contains(err.Error(), id) {
			t.Errorf("expected %s in cycle error, got: %v", id, err)
		}
	}
	// E should NOT be in the error
	if strings.Contains(err.Error(), "E") {
		t.Errorf("non-cyclic item E should not be in error, got: %v", err)
	}
}

// --- Long cycle ---

func TestBuild_LongCycleChain(t *testing.T) {
	// A -> B -> C -> D -> E -> A (5-node cycle)
	items := []*item.Item{
		{ID: "A", Dependencies: []string{"E"}},
		{ID: "B", Dependencies: []string{"A"}},
		{ID: "C", Dependencies: []string{"B"}},
		{ID: "D", Dependencies: []string{"C"}},
		{ID: "E", Dependencies: []string{"D"}},
	}

	_, err := Build(items)
	if err == nil {
		t.Fatal("expected error for 5-node cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected 'cycle' in error, got: %v", err)
	}
}

// --- Cycle with done items ---

func TestBuild_CycleDetectedRegardlessOfStatus(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusDone, Dependencies: []string{"B"}},
		{ID: "B", Status: item.StatusDone, Dependencies: []string{"A"}},
	}

	_, err := Build(items)
	if err == nil {
		t.Fatal("cycles should be detected even if all items are done")
	}
}

// --- WaitingOn edge cases ---

func TestWaitingOn_NoDeps(t *testing.T) {
	g := mustBuild(t, []*item.Item{
		{ID: "A", Status: item.StatusTodo},
	})
	waiting := g.WaitingOn("A")
	if len(waiting) != 0 {
		t.Errorf("WaitingOn should be empty for item with no deps, got %v", waiting)
	}
}

func TestWaitingOn_AllDepsDone(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusDone},
		{ID: "B", Status: item.StatusTodo, Dependencies: []string{"A"}},
	}
	g := mustBuild(t, items)
	waiting := g.WaitingOn("B")
	if len(waiting) != 0 {
		t.Errorf("WaitingOn should be empty when all deps done, got %v", waiting)
	}
}

func TestWaitingOn_UnknownItem(t *testing.T) {
	g := mustBuild(t, []*item.Item{
		{ID: "A", Status: item.StatusTodo},
	})
	waiting := g.WaitingOn("NONEXISTENT")
	if len(waiting) != 0 {
		t.Errorf("WaitingOn for unknown item should be empty, got %v", waiting)
	}
}

// --- DependentsOf edge cases ---

func TestDependentsOf_NoDependents(t *testing.T) {
	g := mustBuild(t, []*item.Item{
		{ID: "A", Status: item.StatusTodo},
	})
	deps := g.DependentsOf("A")
	if deps != nil {
		t.Errorf("DependentsOf should be nil for item with no dependents, got %v", deps)
	}
}

func TestDependentsOf_UnknownItem(t *testing.T) {
	g := mustBuild(t, []*item.Item{
		{ID: "A", Status: item.StatusTodo},
	})
	deps := g.DependentsOf("NONEXISTENT")
	if deps != nil {
		t.Errorf("DependentsOf for unknown item should be nil, got %v", deps)
	}
}

func TestDependentsOf_ReturnsCopy(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
		{ID: "B", Status: item.StatusTodo, Dependencies: []string{"A"}},
	}
	g := mustBuild(t, items)

	deps1 := g.DependentsOf("A")
	deps2 := g.DependentsOf("A")

	// Mutating one should not affect the other
	if len(deps1) > 0 {
		deps1[0] = "MUTATED"
	}
	if len(deps2) > 0 && deps2[0] == "MUTATED" {
		t.Error("DependentsOf returned same slice, not a copy")
	}
}

// --- Edges edge cases ---

func TestEdges_NoEdges(t *testing.T) {
	g := mustBuild(t, []*item.Item{
		{ID: "A", Status: item.StatusTodo},
		{ID: "B", Status: item.StatusTodo},
	})
	edges := g.Edges()
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

func TestEdges_Sorted(t *testing.T) {
	items := []*item.Item{
		{ID: "Z", Status: item.StatusTodo},
		{ID: "A", Status: item.StatusTodo, Dependencies: []string{"Z"}},
		{ID: "M", Status: item.StatusTodo, Dependencies: []string{"Z"}},
	}
	g := mustBuild(t, items)
	edges := g.Edges()

	// Edges should be sorted by From, then To
	for i := 1; i < len(edges); i++ {
		prev := edges[i-1]
		curr := edges[i]
		if prev.From > curr.From || (prev.From == curr.From && prev.To > curr.To) {
			t.Errorf("edges not sorted: %v > %v", prev, curr)
		}
	}
}

// --- Priority sorting edge cases ---

func TestReady_SortingWithMixedPriorities(t *testing.T) {
	items := []*item.Item{
		{ID: "D", Priority: item.PriorityLow, Status: item.StatusTodo},
		{ID: "C", Priority: item.PriorityMedium, Status: item.StatusTodo},
		{ID: "B", Priority: item.PriorityHigh, Status: item.StatusTodo},
		{ID: "A", Priority: item.PriorityCritical, Status: item.StatusTodo},
	}
	g := mustBuild(t, items)
	ready := g.Ready()

	expected := []string{"A", "B", "C", "D"}
	if len(ready) != 4 {
		t.Fatalf("expected 4 ready, got %d", len(ready))
	}
	for i, exp := range expected {
		if ready[i].ID != exp {
			t.Errorf("ready[%d] = %s, want %s", i, ready[i].ID, exp)
		}
	}
}

func TestReady_SortingWithEmptyPriority(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Priority: "", Status: item.StatusTodo},
		{ID: "B", Priority: item.PriorityCritical, Status: item.StatusTodo},
		{ID: "C", Priority: "", Status: item.StatusTodo},
	}
	g := mustBuild(t, items)
	ready := g.Ready()

	// B (Critical) should be first, then A and C (empty = last, sorted by ID)
	if len(ready) != 3 {
		t.Fatalf("expected 3 ready, got %d", len(ready))
	}
	if ready[0].ID != "B" {
		t.Errorf("first ready = %s, want B (Critical)", ready[0].ID)
	}
	if ready[1].ID != "A" {
		t.Errorf("second ready = %s, want A", ready[1].ID)
	}
	if ready[2].ID != "C" {
		t.Errorf("third ready = %s, want C", ready[2].ID)
	}
}

func TestReady_SortingStabilityWithSamePriority(t *testing.T) {
	items := []*item.Item{
		{ID: "Z", Priority: item.PriorityHigh, Status: item.StatusTodo},
		{ID: "A", Priority: item.PriorityHigh, Status: item.StatusTodo},
		{ID: "M", Priority: item.PriorityHigh, Status: item.StatusTodo},
	}
	g := mustBuild(t, items)

	// Run multiple times to check stability
	for range 10 {
		ready := g.Ready()
		if len(ready) != 3 {
			t.Fatalf("expected 3 ready, got %d", len(ready))
		}
		if ready[0].ID != "A" || ready[1].ID != "M" || ready[2].ID != "Z" {
			t.Errorf("unstable sort: got [%s, %s, %s], want [A, M, Z]",
				ready[0].ID, ready[1].ID, ready[2].ID)
		}
	}
}

// --- UnblockedBy / StillBlocked with done dependents ---

func TestUnblockedBy_SkipsDoneDependents(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
		{ID: "B", Status: item.StatusDone, Dependencies: []string{"A"}},
	}
	g := mustBuild(t, items)

	items[0].Status = item.StatusDone
	unblocked := g.UnblockedBy("A")
	// B is already done, should not appear in unblocked
	if len(unblocked) != 0 {
		t.Errorf("done dependents should be skipped, got %d", len(unblocked))
	}
}

// --- Item lookup ---

func TestItem_Found(t *testing.T) {
	items := []*item.Item{{ID: "A", Status: item.StatusTodo}}
	g := mustBuild(t, items)
	it, ok := g.Item("A")
	if !ok || it.ID != "A" {
		t.Error("Item(A) should be found")
	}
}

func TestItem_NotFound(t *testing.T) {
	g := mustBuild(t, []*item.Item{{ID: "A", Status: item.StatusTodo}})
	_, ok := g.Item("NONEXISTENT")
	if ok {
		t.Error("Item(NONEXISTENT) should not be found")
	}
}

// --- Duplicate dependencies in a single item ---

func TestBuild_DuplicateDependencies(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
		{ID: "B", Status: item.StatusTodo, Dependencies: []string{"A", "A", "A"}},
	}
	// Should build without error — duplicates are legal (if odd)
	g, err := Build(items)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// B should still show as blocked by A
	blocked := g.Blocked()
	if len(blocked) != 1 || blocked[0].ID != "B" {
		t.Error("B should be blocked")
	}

	// Edges should show the duplicates
	edges := g.Edges()
	count := 0
	for _, e := range edges {
		if e.From == "A" && e.To == "B" {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 duplicate edges A->B, got %d", count)
	}
}

// --- Topological layers determinism ---

func TestTopologicalLayers_Deterministic(t *testing.T) {
	items := makeItems()
	g := mustBuild(t, items)

	first := g.TopologicalLayers()
	for range 20 {
		layers := g.TopologicalLayers()
		if len(layers) != len(first) {
			t.Fatalf("layer count changed: %d vs %d", len(layers), len(first))
		}
		for i := range layers {
			if len(layers[i]) != len(first[i]) {
				t.Fatalf("layer %d size changed: %d vs %d", i, len(layers[i]), len(first[i]))
			}
			for j := range layers[i] {
				if layers[i][j] != first[i][j] {
					t.Errorf("layer[%d][%d] = %s, previously %s", i, j, layers[i][j], first[i][j])
				}
			}
		}
	}
}

// --- Complex unblock scenario ---

func TestUnblockedBy_ComplexFanOut(t *testing.T) {
	// A is depended on by B, C, D
	// B also depends on E (not done)
	// C and D depend only on A
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
		{ID: "B", Status: item.StatusTodo, Dependencies: []string{"A", "E"}},
		{ID: "C", Status: item.StatusTodo, Dependencies: []string{"A"}},
		{ID: "D", Status: item.StatusTodo, Dependencies: []string{"A"}},
		{ID: "E", Status: item.StatusTodo},
	}
	g := mustBuild(t, items)

	items[0].Status = item.StatusDone
	unblocked := g.UnblockedBy("A")
	stillBlocked := g.StillBlocked("A")

	// C and D should be unblocked (only dep was A)
	if len(unblocked) != 2 {
		ids := make([]string, len(unblocked))
		for i, u := range unblocked {
			ids[i] = u.ID
		}
		t.Errorf("unblocked = %v, want [C, D]", ids)
	}

	// B should still be blocked (also depends on E which is todo)
	if len(stillBlocked) != 1 || stillBlocked[0].ID != "B" {
		t.Errorf("stillBlocked should be [B], got %d items", len(stillBlocked))
	}
}
