package cmd

import (
	"fmt"
	"strings"

	"github.com/ryanlewis/yat/internal/item"
)

// CompleteCmd sets an item's status to done and shows newly unblocked items.
type CompleteCmd struct {
	ID string `arg:"" help:"Item ID to complete."`
}

type completeJSON struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Unblocked    []unblockedRef `json:"unblocked"`
	StillBlocked []blockedRef   `json:"still_blocked"`
}

type unblockedRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type blockedRef struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	WaitingOn []string `json:"waiting_on"`
}

// Run executes the complete command.
func (c *CompleteCmd) Run(rc *RunContext) error {
	if rc.ReadOnly {
		return fmt.Errorf("project is read-only (readonly: true in .yat.yaml)")
	}

	it, ok := rc.Graph.Item(c.ID)
	if !ok {
		return fmt.Errorf("item %q not found", c.ID)
	}

	if rc.Statuses.IsDone(string(it.Status)) {
		return fmt.Errorf("item %s is already done", c.ID)
	}

	// Simulate completion in memory so UnblockedBy/StillBlocked see the new state
	prevStatus := it.Status
	it.Status = item.Status(rc.Statuses.DefaultDone())
	unblocked := rc.Graph.UnblockedBy(c.ID)
	stillBlocked := rc.Graph.StillBlocked(c.ID)

	target := item.Status(rc.Statuses.DefaultDone())

	if err := item.SetStatusWithOptions(it.FilePath, target, rc.mutateOptions()); err != nil {
		it.Status = prevStatus

		return err
	}

	if rc.JSON {
		out := completeJSON{
			ID:           it.ID,
			Title:        it.Title,
			Unblocked:    []unblockedRef{},
			StillBlocked: []blockedRef{},
		}

		for _, u := range unblocked {
			out.Unblocked = append(out.Unblocked, unblockedRef{ID: u.ID, Title: u.Title})
		}

		for _, b := range stillBlocked {
			out.StillBlocked = append(out.StillBlocked, blockedRef{
				ID:        b.ID,
				Title:     b.Title,
				WaitingOn: rc.Graph.WaitingOn(b.ID),
			})
		}

		return rc.writeJSON(out)
	}

	rc.printf("Completed %s: %s\n", it.ID, it.Title)

	if len(unblocked) > 0 {
		rc.printf("\nNow unblocked:\n")

		for _, u := range unblocked {
			rc.printf("  %s  %s\n", u.ID, u.Title)
		}
	}

	if len(stillBlocked) > 0 {
		rc.printf("\nStill blocked (partial deps resolved):\n")

		for _, b := range stillBlocked {
			waiting := rc.Graph.WaitingOn(b.ID)
			rc.printf("  %s  %s (waiting on: %s)\n", b.ID, b.Title, strings.Join(waiting, ", "))
		}
	}

	return nil
}
