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
	if rc.ReadOnly {
		return fmt.Errorf("project is read-only (readonly: true in .yat.yaml)")
	}

	// Another yat command may have changed the item or its dependencies since
	// rc was loaded. Hold the lock while reloading, checking and writing.
	unlock, err := rc.lockAndReload()
	if err != nil {
		return err
	}
	defer unlock()

	it, ok := rc.Graph.Item(s.ID)
	if !ok {
		return fmt.Errorf("item %q not found", s.ID)
	}

	waiting := rc.Graph.WaitingOn(s.ID)
	if len(waiting) > 0 {
		return fmt.Errorf("cannot start %s: blocked by %s", s.ID, strings.Join(waiting, ", "))
	}

	if rc.Statuses.IsDone(string(it.Status)) {
		return fmt.Errorf("item %s is already done", s.ID)
	}

	if rc.Statuses.IsActive(string(it.Status)) {
		if rc.JSON {
			return rc.writeJSON(startJSON{ID: it.ID, Title: it.Title, Body: it.Body})
		}

		rc.printf("%s is already in progress.\n", s.ID)

		return nil
	}

	target := item.Status(rc.Statuses.DefaultActive())

	opts, err := rc.mutateOptions(it)
	if err != nil {
		return err
	}

	if err := item.SetStatusWithOptions(it.FilePath, target, opts); err != nil {
		return err
	}

	if rc.JSON {
		return rc.writeJSON(startJSON{
			ID:    it.ID,
			Title: it.Title,
			Body:  it.Body,
		})
	}

	rc.printf("Started %s: %s\n", it.ID, oneLine(it.Title))

	if it.Body != "" {
		rc.printf("\n%s", it.Body)
	}

	return nil
}
