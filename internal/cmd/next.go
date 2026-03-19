package cmd

import "fmt"

// NextCmd shows the single highest-priority ready item with its full content.
type NextCmd struct{}

type nextJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Type     string `json:"type"`
	Points   int    `json:"points"`
	Phase    string `json:"phase"`
	Body     string `json:"body"`
}

// Run executes the next command.
func (n *NextCmd) Run(rc *RunContext) error {
	items := rc.Graph.Ready()

	if len(items) == 0 {
		if rc.JSON {
			return rc.writeJSON(nil)
		}

		rc.printf("No items are ready.\n")

		return nil
	}

	item := items[0]

	if rc.JSON {
		return rc.writeJSON(nextJSON{
			ID:       item.ID,
			Title:    item.Title,
			Priority: string(item.Priority),
			Type:     item.Type,
			Points:   item.Points,
			Phase:    item.Phase,
			Body:     item.Body,
		})
	}

	rc.printf("%s: %s\n", item.ID, item.Title)

	fmt.Fprintf(rc.Stdout, "Priority: %s  |  Type: %s  |  Points: %d  |  Phase: %s\n",
		item.Priority, item.Type, item.Points, item.Phase)

	if item.Body != "" {
		rc.printf("\n%s", item.Body)
	}

	return nil
}
