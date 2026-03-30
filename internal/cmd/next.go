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

	s := rc.styles()

	rc.printf("%s", s.Header(item.ID, item.Title))

	fmt.Fprintf(rc.Stdout, "%s  |  Type: %s  |  Points: %d  |  Phase: %s\n",
		s.Dim("Priority: ")+s.Priority(item.Priority),
		item.Type, item.Points, item.Phase)

	if item.Body != "" {
		rc.printf("\n%s", s.RenderBody(item.Body))
	}

	return nil
}
