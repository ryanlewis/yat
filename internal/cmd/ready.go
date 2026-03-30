package cmd

import "fmt"

// ReadyCmd shows items where all dependencies are done and the item itself is not done.
type ReadyCmd struct{}

type readyJSON struct {
	ID       string `json:"id"`
	Priority string `json:"priority"`
	Type     string `json:"type"`
	Title    string `json:"title"`
}

// Run executes the ready command.
func (r *ReadyCmd) Run(rc *RunContext) error {
	items := rc.Graph.Ready()

	if rc.JSON {
		out := make([]readyJSON, len(items))
		for i, item := range items {
			out[i] = readyJSON{
				ID:       item.ID,
				Priority: string(item.Priority),
				Type:     item.Type,
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

	w := rc.newTabWriter()

	for _, item := range items {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
			item.ID, s.Priority(item.Priority), item.Type, item.Title)
	}

	return w.Flush()
}
