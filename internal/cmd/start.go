package cmd

import (
	"fmt"
	"strings"

	"github.com/ryanlewis/yat/internal/item"
)

// StartCmd sets an item's status to in-progress.
type StartCmd struct {
	ID string `arg:"" help:"Item ID to start."`
}

type startJSON struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Run executes the start command.
func (s *StartCmd) Run(rc *RunContext) error {
	it, ok := rc.Graph.Item(s.ID)
	if !ok {
		return fmt.Errorf("item %q not found", s.ID)
	}

	waiting := rc.Graph.WaitingOn(s.ID)
	if len(waiting) > 0 {
		return fmt.Errorf("cannot start %s: blocked by %s", s.ID, strings.Join(waiting, ", "))
	}

	if it.Status == item.StatusDone {
		return fmt.Errorf("item %s is already done", s.ID)
	}

	if it.Status == item.StatusInProgress {
		if rc.JSON {
			return rc.writeJSON(startJSON{ID: it.ID, Title: it.Title, Body: it.Body})
		}

		rc.printf("%s is already in progress.\n", s.ID)

		return nil
	}

	if err := item.SetStatus(it.FilePath, item.StatusInProgress); err != nil {
		return err
	}

	if rc.JSON {
		return rc.writeJSON(startJSON{
			ID:    it.ID,
			Title: it.Title,
			Body:  it.Body,
		})
	}

	rc.printf("Started %s: %s\n", it.ID, it.Title)

	if it.Body != "" {
		rc.printf("\n%s", it.Body)
	}

	return nil
}
