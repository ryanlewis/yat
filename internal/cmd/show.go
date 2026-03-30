package cmd

import "fmt"

// ShowCmd prints the full markdown content of an item.
type ShowCmd struct {
	ID string `arg:"" help:"Item ID to show."`
}

type showJSON struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Type         string   `json:"type"`
	Priority     string   `json:"priority"`
	Points       int      `json:"points"`
	Dependencies []string `json:"dependencies"`
	Status       string   `json:"status"`
	Phase        string   `json:"phase"`
	FilePath     string   `json:"file_path"`
	Body         string   `json:"body"`
}

// Run executes the show command.
func (s *ShowCmd) Run(rc *RunContext) error {
	item, ok := rc.Graph.Item(s.ID)
	if !ok {
		return fmt.Errorf("item %q not found", s.ID)
	}

	if rc.JSON {
		deps := item.Dependencies
		if deps == nil {
			deps = []string{}
		}

		return rc.writeJSON(showJSON{
			ID:           item.ID,
			Title:        item.Title,
			Type:         item.Type,
			Priority:     string(item.Priority),
			Points:       item.Points,
			Dependencies: deps,
			Status:       string(item.Status),
			Phase:        item.Phase,
			FilePath:     item.FilePath,
			Body:         item.Body,
		})
	}

	sty := rc.styles()

	rc.printf("%s", sty.Header(item.ID, item.Title))

	fmt.Fprintf(rc.Stdout, "%s  |  Type: %s  |  Points: %d  |  Phase: %s  |  %s\n",
		sty.Dim("Priority: ")+sty.Priority(item.Priority),
		item.Type, item.Points, item.Phase,
		sty.Status(item.Status))

	if len(item.Dependencies) > 0 {
		rc.printf("%s", sty.Dim("Dependencies:"))

		for _, dep := range item.Dependencies {
			fmt.Fprintf(rc.Stdout, " %s", dep)
		}

		rc.printf("\n")
	}

	if item.Body != "" {
		rc.printf("\n%s", sty.RenderBody(item.Body))
	}

	return nil
}
