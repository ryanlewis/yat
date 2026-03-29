package cmd

import (
	"fmt"
	"sort"
	"strings"
)

// GraphCmd renders the dependency graph as text.
type GraphCmd struct{}

type graphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type graphJSON struct {
	Layers [][]string  `json:"layers"`
	Edges  []graphEdge `json:"edges"`
}

// Run executes the graph command.
func (g *GraphCmd) Run(rc *RunContext) error {
	layers := rc.Graph.TopologicalLayers()

	if rc.JSON {
		graphEdges := rc.Graph.Edges()
		edges := make([]graphEdge, len(graphEdges))

		for i, e := range graphEdges {
			edges[i] = graphEdge{From: e.From, To: e.To}
		}

		return rc.writeJSON(graphJSON{
			Layers: layers,
			Edges:  edges,
		})
	}

	if len(layers) == 0 {
		rc.printf("No items found.\n")

		return nil
	}

	rc.printf("▶️ ready  🔄 active  🚫 blocked  ✅ done\n\n")

	return g.renderText(rc, layers)
}

func (g *GraphCmd) renderText(rc *RunContext, layers [][]string) error {
	maxIDLen := graphMaxIDLen(layers)

	for i, layer := range layers {
		if i > 0 {
			rc.printf("\n")
		}

		rc.printf("Layer %d\n", i)

		for _, id := range layer {
			it, ok := rc.Graph.Item(id)
			if !ok {
				return fmt.Errorf("internal error: graph layer references unknown item %q", id)
			}

			emoji := graphStatusEmoji(string(it.Status), rc.Graph.AllDepsDone(id), rc)
			deps := rc.Graph.DependentsOf(id)
			sort.Strings(deps)

			if len(deps) > 0 {
				fmt.Fprintf(rc.Stdout, "  %s %-*s  → %s\n", emoji, maxIDLen, id, strings.Join(deps, ", "))
			} else {
				fmt.Fprintf(rc.Stdout, "  %s %s\n", emoji, id)
			}
		}
	}

	return nil
}

func graphMaxIDLen(layers [][]string) int {
	maxLen := 0
	for _, layer := range layers {
		for _, id := range layer {
			if len(id) > maxLen {
				maxLen = len(id)
			}
		}
	}

	return maxLen
}

func graphStatusEmoji(status string, allDepsDone bool, rc *RunContext) string {
	switch {
	case rc.Statuses.IsDone(status):
		return "✅"
	case rc.Statuses.IsActive(status):
		return "🔄"
	case allDepsDone:
		return "▶️"
	default:
		return "🚫"
	}
}
