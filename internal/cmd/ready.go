package cmd

import (
	"fmt"

	"github.com/ryanlewis/yat/internal/item"
)

// ReadyCmd shows items where all dependencies are done and the item itself is not started.
type ReadyCmd struct {
	All bool `help:"Include in-progress items." short:"a"`
}

type readyJSON struct {
	ID       string `json:"id"`
	Priority string `json:"priority"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Title    string `json:"title"`
}

// Run executes the ready command.
func (r *ReadyCmd) Run(rc *RunContext) error {
	var items []*item.Item
	if r.All {
		items = rc.Graph.ReadyAll()
	} else {
		items = rc.Graph.Ready()
	}

	if rc.JSON {
		out := make([]readyJSON, len(items))
		for i, item := range items {
			out[i] = readyJSON{
				ID:       item.ID,
				Priority: string(item.Priority),
				Type:     item.Type,
				Status:   string(item.Status),
				Title:    item.Title,
			}
		}

		return rc.writeJSON(out)
	}

	if len(items) == 0 {
		rc.printf("No items are ready.\n")

		return nil
	}

	s := rc.styles()

	rc.printf("%s", s.GroupHeader("ready"))

	var maxID, maxPri, maxType int
	for _, it := range items {
		maxID = max(maxID, len(it.ID))
		maxPri = max(maxPri, len(string(it.Priority)))
		maxType = max(maxType, len(it.Type))
	}

	for _, it := range items {
		fmt.Fprintf(rc.Stdout, "  %-*s  %s  %-*s  %s\n",
			maxID, it.ID,
			padRight(s.Priority(it.Priority), len(string(it.Priority)), maxPri),
			maxType, it.Type,
			it.Title)
	}

	return nil
}
