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

	// Both Critical, same critical depth; TK-001 unblocks TK-003, SP-001 unblocks nothing
	if ready[0].ID != "TK-001" || ready[1].ID != "SP-001" {
		t.Errorf("ready = [%s, %s], want [TK-001, SP-001]", ready[0].ID, ready[1].ID)
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

	g, err := Build(items, WithIsDone(isDone))
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

	g, err := Build(items, WithIsDone(isDone))
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

	g, err := Build(items, WithIsDone(isDone))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	waiting := g.WaitingOn("C")
	if len(waiting) != 1 || waiting[0] != "B" {
		t.Errorf("waiting = %v, want [B]", waiting)
	}
}

// --- ActivePhase ---

func TestActivePhase_NoPhases(t *testing.T) {
	g := mustBuild(t, []*item.Item{
		{ID: "A", Status: item.StatusDraft},
		{ID: "B", Status: item.StatusDraft},
	})
	if got := g.ActivePhase(); got != "" {
		t.Errorf("ActivePhase() = %q, want empty", got)
	}
}

func TestActivePhase_EarliestIncomplete(t *testing.T) {
	g := mustBuild(t, []*item.Item{
		{ID: "A", Phase: "1", Status: item.StatusDone},
		{ID: "B", Phase: "1", Status: item.StatusDone},
		{ID: "C", Phase: "2", Status: item.StatusDraft},
		{ID: "D", Phase: "3", Status: item.StatusDraft},
	})
	if got := g.ActivePhase(); got != "2" {
		t.Errorf("ActivePhase() = %q, want 2", got)
	}
}

func TestActivePhase_FirstPhaseStillActive(t *testing.T) {
	g := mustBuild(t, []*item.Item{
		{ID: "A", Phase: "1", Status: item.StatusDone},
		{ID: "B", Phase: "1", Status: item.StatusInProgress},
		{ID: "C", Phase: "2", Status: item.StatusDraft},
	})
	if got := g.ActivePhase(); got != "1" {
		t.Errorf("ActivePhase() = %q, want 1", got)
	}
}

func TestActivePhase_AllDone(t *testing.T) {
	g := mustBuild(t, []*item.Item{
		{ID: "A", Phase: "1", Status: item.StatusDone},
		{ID: "B", Phase: "2", Status: item.StatusDone},
	})
	if got := g.ActivePhase(); got != "" {
		t.Errorf("ActivePhase() = %q, want empty", got)
	}
}

// --- ActivePhase with explicit order ---

func TestActivePhase_ExplicitOrder(t *testing.T) {
	// "Pre-1" sorts after "1" lexicographically, but config says it comes first.
	g, err := Build([]*item.Item{
		{ID: "A", Phase: "Pre-1", Status: item.StatusDraft},
		{ID: "B", Phase: "1", Status: item.StatusDraft},
		{ID: "C", Phase: "2", Status: item.StatusDraft},
	}, WithPhaseOrder([]string{"Pre-1", "1", "2"}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := g.ActivePhase(); got != "Pre-1" {
		t.Errorf("ActivePhase() = %q, want Pre-1", got)
	}
}

func TestActivePhase_ExplicitOrderSkipsDone(t *testing.T) {
	g, err := Build([]*item.Item{
		{ID: "A", Phase: "Pre-1", Status: item.StatusDone},
		{ID: "B", Phase: "1", Status: item.StatusDraft},
		{ID: "C", Phase: "2", Status: item.StatusDraft},
	}, WithPhaseOrder([]string{"Pre-1", "1", "2"}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := g.ActivePhase(); got != "1" {
		t.Errorf("ActivePhase() = %q, want 1", got)
	}
}

func TestActivePhase_ExplicitOrderUnlistedPhaseIgnored(t *testing.T) {
	// Phase "mystery" is not in the config order — should not be returned.
	g, err := Build([]*item.Item{
		{ID: "A", Phase: "1", Status: item.StatusDone},
		{ID: "B", Phase: "mystery", Status: item.StatusDraft},
	}, WithPhaseOrder([]string{"1", "2"}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := g.ActivePhase(); got != "" {
		t.Errorf("ActivePhase() = %q, want empty (unlisted phase ignored)", got)
	}
}

func TestReady_ExplicitPhaseOrder(t *testing.T) {
	// Without config, lexicographic would pick "1" over "Pre-1".
	// With config, "Pre-1" is the active phase.
	g, err := Build([]*item.Item{
		{ID: "A", Phase: "Pre-1", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "B", Phase: "1", Priority: item.PriorityHigh, Status: item.StatusDraft},
	}, WithPhaseOrder([]string{"Pre-1", "1", "2"}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	ready := g.Ready()
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready, got %d", len(ready))
	}
	if ready[0].ID != "A" {
		t.Errorf("ready[0] = %s, want A (Pre-1 is active phase per config)", ready[0].ID)
	}
}

// --- Phase-based sorting ---

func TestReady_PhaseSorting(t *testing.T) {
	// A is in the active phase, B is in a later phase, C has no phase.
	// All same priority, no deps.
	g := mustBuild(t, []*item.Item{
		{ID: "A", Phase: "2", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "B", Phase: "1", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "C", Priority: item.PriorityHigh, Status: item.StatusDraft},
	})
	// Active phase = "1" (earliest incomplete)
	ready := g.Ready()
	if len(ready) != 3 {
		t.Fatalf("expected 3 ready, got %d", len(ready))
	}
	// B (phase 1 = active) first, C (no phase = neutral) second, A (phase 2 = wrong) last
	if ready[0].ID != "B" {
		t.Errorf("ready[0] = %s, want B (active phase)", ready[0].ID)
	}
	if ready[1].ID != "C" {
		t.Errorf("ready[1] = %s, want C (no phase)", ready[1].ID)
	}
	if ready[2].ID != "A" {
		t.Errorf("ready[2] = %s, want A (wrong phase)", ready[2].ID)
	}
}

func TestReady_PhaseCompletedAdvancesToNext(t *testing.T) {
	g := mustBuild(t, []*item.Item{
		{ID: "A", Phase: "1", Priority: item.PriorityHigh, Status: item.StatusDone},
		{ID: "B", Phase: "2", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "C", Phase: "3", Priority: item.PriorityHigh, Status: item.StatusDraft},
	})
	ready := g.Ready()
	// Active phase = "2", so B first, C last
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready, got %d", len(ready))
	}
	if ready[0].ID != "B" {
		t.Errorf("ready[0] = %s, want B (active phase 2)", ready[0].ID)
	}
	if ready[1].ID != "C" {
		t.Errorf("ready[1] = %s, want C (wrong phase 3)", ready[1].ID)
	}
}

// --- Critical path depth sorting ---

func TestReady_CriticalPathDepth(t *testing.T) {
	// A has a 3-deep chain below it, B has a 1-deep chain. Same priority.
	g := mustBuild(t, []*item.Item{
		{ID: "A", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "B", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "C", Status: item.StatusDraft, Dependencies: []string{"A"}},
		{ID: "D", Status: item.StatusDraft, Dependencies: []string{"C"}},
		{ID: "E", Status: item.StatusDraft, Dependencies: []string{"D"}},
		{ID: "F", Status: item.StatusDraft, Dependencies: []string{"B"}},
	})
	ready := g.Ready()
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready, got %d", len(ready))
	}
	// A has depth 3 (A→C→D→E), B has depth 1 (B→F) → A first
	if ready[0].ID != "A" {
		t.Errorf("ready[0] = %s, want A (deeper critical path)", ready[0].ID)
	}
	if ready[1].ID != "B" {
		t.Errorf("ready[1] = %s, want B (shallower critical path)", ready[1].ID)
	}
}

func TestReady_CriticalPathIgnoresDoneItems(t *testing.T) {
	// A has dependents C (done) and D (draft). B has dependent E (draft) and F (draft→E).
	g := mustBuild(t, []*item.Item{
		{ID: "A", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "B", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "C", Status: item.StatusDone, Dependencies: []string{"A"}},
		{ID: "D", Status: item.StatusDraft, Dependencies: []string{"A"}},
		{ID: "E", Status: item.StatusDraft, Dependencies: []string{"B"}},
		{ID: "F", Status: item.StatusDraft, Dependencies: []string{"E"}},
	})
	ready := g.Ready()
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready, got %d", len(ready))
	}
	// A: depth 1 (only D is non-done). B: depth 2 (B→E→F). B first.
	if ready[0].ID != "B" {
		t.Errorf("ready[0] = %s, want B (deeper non-done chain)", ready[0].ID)
	}
	if ready[1].ID != "A" {
		t.Errorf("ready[1] = %s, want A (shallower non-done chain)", ready[1].ID)
	}
}

// --- Unblock impact sorting ---

func TestReady_UnblockImpact(t *testing.T) {
	// A and B have same priority, same critical depth.
	// A unblocks 2 items immediately, B unblocks 0.
	g := mustBuild(t, []*item.Item{
		{ID: "A", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "B", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "C", Status: item.StatusDraft, Dependencies: []string{"A"}},
		{ID: "D", Status: item.StatusDraft, Dependencies: []string{"A"}},
		{ID: "E", Status: item.StatusDraft, Dependencies: []string{"B", "A"}},
	})
	ready := g.Ready()
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready, got %d", len(ready))
	}
	// A: depth 1, unblocks C and D (E still blocked by B). B: depth 1, unblocks 0.
	// A first by unblock impact.
	if ready[0].ID != "A" {
		t.Errorf("ready[0] = %s, want A (unblocks 2)", ready[0].ID)
	}
	if ready[1].ID != "B" {
		t.Errorf("ready[1] = %s, want B (unblocks 0)", ready[1].ID)
	}
}

// --- Priority still outranks structural metrics ---

func TestReady_PriorityBeatsDepth(t *testing.T) {
	// A is Low priority but has a deep chain. B is Critical with no chain.
	// Priority should win.
	g := mustBuild(t, []*item.Item{
		{ID: "A", Priority: item.PriorityLow, Status: item.StatusDraft},
		{ID: "B", Priority: item.PriorityCritical, Status: item.StatusDraft},
		{ID: "C", Status: item.StatusDraft, Dependencies: []string{"A"}},
		{ID: "D", Status: item.StatusDraft, Dependencies: []string{"C"}},
	})
	ready := g.Ready()
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready, got %d", len(ready))
	}
	if ready[0].ID != "B" {
		t.Errorf("ready[0] = %s, want B (Critical beats deeper Low)", ready[0].ID)
	}
}

// --- Combined multi-factor ---

func TestReady_CombinedPhaseAndPriority(t *testing.T) {
	// B is Critical but wrong phase. A is High but active phase. Phase wins.
	g := mustBuild(t, []*item.Item{
		{ID: "A", Phase: "1", Priority: item.PriorityHigh, Status: item.StatusDraft},
		{ID: "B", Phase: "2", Priority: item.PriorityCritical, Status: item.StatusDraft},
	})
	ready := g.Ready()
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready, got %d", len(ready))
	}
	if ready[0].ID != "A" {
		t.Errorf("ready[0] = %s, want A (active phase beats higher priority)", ready[0].ID)
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
