package cmd

import "fmt"

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

	for i, layer := range layers {
		rc.printf("Layer %d:", i)

		for _, id := range layer {
			item, ok := rc.Graph.Item(id)
			if !ok {
				return fmt.Errorf("internal error: graph layer references unknown item %q", id)
			}
			status := string(item.Status)

			fmt.Fprintf(rc.Stdout, "  %s [%s]", id, status)
		}

		rc.printf("\n")

		// Show edges from this layer to dependents
		for _, id := range layer {
			for _, depID := range rc.Graph.DependentsOf(id) {
				fmt.Fprintf(rc.Stdout, "  %s -> %s\n", id, depID)
			}
		}
	}

	return nil
}
