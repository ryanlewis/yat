// Package graph builds and queries a dependency graph from items.
package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ryanlewis/yat/internal/item"
)

// Graph holds the dependency relationships between items.
type Graph struct {
	items      map[string]*item.Item
	forward    map[string][]string // item -> its dependencies
	reverse    map[string][]string // item -> items that depend on it
	isDone     func(item.Status) bool
	phaseOrder []string // explicit phase ordering from config; nil = lexicographic
}

// Edge represents a dependency relationship.
type Edge struct {
	From string // dependency
	To   string // dependent
}

// Item returns the item with the given ID.
func (g *Graph) Item(id string) (*item.Item, bool) {
	i, ok := g.items[id]
	return i, ok
}

// DependentsOf returns the IDs of items that depend on the given item.
func (g *Graph) DependentsOf(id string) []string {
	deps := g.reverse[id]
	if deps == nil {
		return nil
	}

	out := make([]string, len(deps))
	copy(out, deps)

	return out
}

// Edges returns all dependency edges, sorted for stable output.
func (g *Graph) Edges() []Edge {
	var edges []Edge

	for id, deps := range g.forward {
		for _, dep := range deps {
			edges = append(edges, Edge{From: dep, To: id})
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}

		return edges[i].To < edges[j].To
	})

	return edges
}

// BuildOption configures optional behavior of Build.
type BuildOption func(*Graph)

// WithIsDone sets a custom function to determine if an item's status means "done".
func WithIsDone(fn func(item.Status) bool) BuildOption {
	return func(g *Graph) { g.isDone = fn }
}

// WithPhaseOrder sets an explicit phase ordering for ActivePhase.
// When set, the active phase is the first entry with incomplete items
// instead of the lexicographically earliest.
func WithPhaseOrder(phases []string) BuildOption {
	return func(g *Graph) { g.phaseOrder = phases }
}

// Build constructs a dependency graph from a slice of items.
// If isDone is nil, defaults to checking for item.StatusDone.
// Returns an error if any item references a dependency ID that does not exist.
func Build(items []*item.Item, opts ...BuildOption) (*Graph, error) {
	g := &Graph{
		items:   make(map[string]*item.Item, len(items)),
		forward: make(map[string][]string, len(items)),
		reverse: make(map[string][]string, len(items)),
		isDone:  func(s item.Status) bool { return s == item.StatusDone },
	}

	for _, opt := range opts {
		opt(g)
	}

	for _, i := range items {
		g.items[i.ID] = i
		g.forward[i.ID] = i.Dependencies
	}

	for id, deps := range g.forward {
		for _, dep := range deps {
			if _, ok := g.items[dep]; !ok {
				return nil, fmt.Errorf("item %s has unknown dependency %q", id, dep)
			}

			g.reverse[dep] = append(g.reverse[dep], id)
		}
	}

	if err := g.detectCycles(); err != nil {
		return nil, err
	}

	return g, nil
}

// detectCycles verifies all nodes appear in the topological sort.
// Nodes involved in cycles will have permanently non-zero in-degree.
func (g *Graph) detectCycles() error {
	layers := g.TopologicalLayers()

	var visited int
	for _, layer := range layers {
		visited += len(layer)
	}

	if visited == len(g.items) {
		return nil
	}

	inLayer := make(map[string]bool, visited)
	for _, layer := range layers {
		for _, id := range layer {
			inLayer[id] = true
		}
	}

	var cyclic []string
	for id := range g.items {
		if !inLayer[id] {
			cyclic = append(cyclic, id)
		}
	}

	sort.Strings(cyclic)

	return fmt.Errorf("dependency cycle detected involving: %s", strings.Join(cyclic, ", "))
}

// Ready returns items where status != done and all dependencies have status == done.
// Results are sorted by: phase match, priority, critical path depth (descending),
// unblock impact (descending), then ID.
func (g *Graph) Ready() []*item.Item {
	var ready []*item.Item

	for _, i := range g.items {
		if g.isDone(i.Status) {
			continue
		}

		if g.AllDepsDone(i.ID) {
			ready = append(ready, i)
		}
	}

	activePhase := g.ActivePhase()
	depths := g.criticalDepthMap()

	unblockCounts := make(map[string]int, len(ready))
	for _, i := range ready {
		unblockCounts[i.ID] = len(g.UnblockedBy(i.ID))
	}

	sort.Slice(ready, func(a, b int) bool {
		ia, ib := ready[a], ready[b]

		// 1. Phase match (active phase first, no phase neutral, wrong phase last)
		pa := phaseRank(ia.Phase, activePhase)
		pb := phaseRank(ib.Phase, activePhase)
		if pa != pb {
			return pa < pb
		}

		// 2. Priority
		pra := item.PriorityRank(ia.Priority)
		prb := item.PriorityRank(ib.Priority)
		if pra != prb {
			return pra < prb
		}

		// 3. Critical path depth (deeper chain = more important)
		da := depths[ia.ID]
		db := depths[ib.ID]
		if da != db {
			return da > db
		}

		// 4. Unblock impact (more immediate unblocks = more important)
		ua := unblockCounts[ia.ID]
		ub := unblockCounts[ib.ID]
		if ua != ub {
			return ua > ub
		}

		// 5. ID tiebreaker
		return ia.ID < ib.ID
	})

	return ready
}

// phaseRank returns a sort rank for an item's phase relative to the active phase.
// 0 = matches active phase, 1 = no phase set (neutral), 2 = wrong phase.
func phaseRank(itemPhase, activePhase string) int {
	if activePhase == "" {
		return 1
	}

	if itemPhase == activePhase {
		return 0
	}

	if itemPhase == "" {
		return 1
	}

	return 2
}

// Blocked returns items where status != done and at least one dependency is not done.
func (g *Graph) Blocked() []*item.Item {
	var blocked []*item.Item

	for _, i := range g.items {
		if g.isDone(i.Status) {
			continue
		}

		if !g.AllDepsDone(i.ID) {
			blocked = append(blocked, i)
		}
	}

	sort.Slice(blocked, func(a, b int) bool {
		return blocked[a].ID < blocked[b].ID
	})

	return blocked
}

// WaitingOn returns the IDs of dependencies that are not yet done for the given item.
func (g *Graph) WaitingOn(id string) []string {
	var waiting []string

	for _, dep := range g.forward[id] {
		i, ok := g.items[dep]
		if !ok || !g.isDone(i.Status) {
			waiting = append(waiting, dep)
		}
	}

	sort.Strings(waiting)

	return waiting
}

// UnblockedBy returns items that would become ready if the given item were completed.
// It checks each dependent: if all of that dependent's OTHER deps are done, it's newly unblocked.
func (g *Graph) UnblockedBy(id string) []*item.Item {
	return g.partitionDependents(id, true)
}

// StillBlocked returns items that are partially unblocked by completing the given item
// but still have other incomplete dependencies. These are items that depend on id
// but won't become ready when id is completed.
func (g *Graph) StillBlocked(id string) []*item.Item {
	return g.partitionDependents(id, false)
}

// partitionDependents returns dependents of id filtered by whether all their other
// deps are done (wantReady=true) or not (wantReady=false).
func (g *Graph) partitionDependents(id string, wantReady bool) []*item.Item {
	var result []*item.Item

	for _, depID := range g.reverse[id] {
		dep, ok := g.items[depID]
		if !ok || g.isDone(dep.Status) {
			continue
		}

		allOthersDone := true

		for _, otherDep := range g.forward[depID] {
			if otherDep == id {
				continue
			}

			other, exists := g.items[otherDep]
			if !exists || !g.isDone(other.Status) {
				allOthersDone = false

				break
			}
		}

		if allOthersDone == wantReady {
			result = append(result, dep)
		}
	}

	sort.Slice(result, func(a, b int) bool {
		return result[a].ID < result[b].ID
	})

	return result
}

// TopologicalLayers returns items grouped into layers using Kahn's algorithm.
// Layer 0 has no dependencies, layer 1 depends only on layer 0 items, etc.
func (g *Graph) TopologicalLayers() [][]string {
	inDegree, current := g.computeInDegree()

	layers := make([][]string, 0, len(g.items))

	for len(current) > 0 {
		layers = append(layers, current)

		var next []string

		for _, id := range current {
			for _, depID := range g.reverse[id] {
				inDegree[depID]--

				if inDegree[depID] == 0 {
					next = append(next, depID)
				}
			}
		}

		sort.Strings(next)
		current = next
	}

	return layers
}

// computeInDegree calculates the in-degree for each node and returns the initial
// set of nodes with no dependencies (roots).
func (g *Graph) computeInDegree() (inDegree map[string]int, roots []string) {
	inDegree = make(map[string]int, len(g.items))
	for id := range g.items {
		inDegree[id] = 0
	}

	for id, deps := range g.forward {
		if _, ok := g.items[id]; !ok {
			continue
		}

		inDegree[id] += len(deps)
	}

	for id, deg := range inDegree {
		if deg == 0 {
			roots = append(roots, id)
		}
	}

	sort.Strings(roots)

	return inDegree, roots
}

// ActivePhase returns the first phase that still has at least one non-done item.
// If a phase order was configured via WithPhaseOrder, it uses that order.
// Otherwise it falls back to lexicographic order.
// Returns "" if no items have a phase set or all phased items are done.
func (g *Graph) ActivePhase() string {
	incomplete := make(map[string]struct{})

	for _, it := range g.items {
		if it.Phase != "" && !g.isDone(it.Status) {
			incomplete[it.Phase] = struct{}{}
		}
	}

	if len(incomplete) == 0 {
		return ""
	}

	// Use explicit order if configured.
	if len(g.phaseOrder) > 0 {
		for _, p := range g.phaseOrder {
			if _, ok := incomplete[p]; ok {
				return p
			}
		}

		return ""
	}

	// Fall back to lexicographic order.
	phases := make([]string, 0, len(incomplete))
	for p := range incomplete {
		phases = append(phases, p)
	}

	sort.Strings(phases)

	return phases[0]
}

// DeepestLayer returns the last layer index and the items in it.
func (g *Graph) DeepestLayer() (depth int, items []string) {
	layers := g.TopologicalLayers()
	if len(layers) == 0 {
		return 0, nil
	}

	last := len(layers) - 1

	return last, layers[last]
}

// criticalDepthMap computes, for each non-done item, the longest chain of
// non-done transitive dependents below it. Leaf items (no non-done dependents)
// have depth 0.
func (g *Graph) criticalDepthMap() map[string]int {
	layers := g.TopologicalLayers()
	depth := make(map[string]int, len(g.items))

	for i := len(layers) - 1; i >= 0; i-- {
		for _, id := range layers[i] {
			if g.isDone(g.items[id].Status) {
				continue
			}

			maxChild := 0

			for _, depID := range g.reverse[id] {
				dep, ok := g.items[depID]
				if !ok || g.isDone(dep.Status) {
					continue
				}

				if depth[depID]+1 > maxChild {
					maxChild = depth[depID] + 1
				}
			}

			depth[id] = maxChild
		}
	}

	return depth
}

// AllDepsDone reports whether all dependencies of the given item are done.
func (g *Graph) AllDepsDone(id string) bool {
	for _, dep := range g.forward[id] {
		i, ok := g.items[dep]
		if !ok || !g.isDone(i.Status) {
			return false
		}
	}

	return true
}
