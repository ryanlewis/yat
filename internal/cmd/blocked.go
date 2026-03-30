package cmd

import (
	"fmt"
	"strings"
)

// BlockedCmd shows items that can't be started yet and what they're waiting on.
type BlockedCmd struct{}

type blockedItemJSON struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	WaitingOn []string `json:"waiting_on"`
}

// Run executes the blocked command.
func (b *BlockedCmd) Run(rc *RunContext) error {
	items := rc.Graph.Blocked()

	if rc.JSON {
		out := make([]blockedItemJSON, len(items))
		for i, item := range items {
			out[i] = blockedItemJSON{
				ID:        item.ID,
				Title:     item.Title,
				WaitingOn: rc.Graph.WaitingOn(item.ID),
			}
		}

		return rc.writeJSON(out)
	}

	if len(items) == 0 {
		rc.printf("No items are blocked.\n")

		return nil
	}

	s := rc.styles()

	for _, item := range items {
		waiting := rc.Graph.WaitingOn(item.ID)

		rc.printf("  %s  %s\n", s.Bold(item.ID), item.Title)

		fmt.Fprintf(rc.Stdout, "          %s %s\n\n",
			s.Dim("waiting on:"), strings.Join(waiting, ", "))
	}

	return nil
}
