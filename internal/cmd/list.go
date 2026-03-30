package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ryanlewis/yat/internal/item"
)

// ListCmd shows all items grouped by status.
type ListCmd struct {
	Phase string `help:"Filter by phase. Use \"active\" for the current active phase." short:"p"`
}

type listItemJSON struct {
	ID        string   `json:"id"`
	Priority  string   `json:"priority"`
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	Title     string   `json:"title"`
	Group     string   `json:"group"`
	WaitingOn []string `json:"waiting_on,omitempty"`
}

type groupedItem struct {
	item      *item.Item
	waitingOn []string
}

type itemGroup struct {
	label string
	items []groupedItem
}

// Run executes the list command.
func (l *ListCmd) Run(rc *RunContext) error {
	items := rc.Items

	if l.Phase != "" {
		phase := l.Phase
		if phase == "active" {
			phase = rc.Graph.ActivePhase()
			if phase == "" {
				if rc.JSON {
					return rc.writeJSON([]listItemJSON{})
				}

				rc.printf("No active phase.\n")

				return nil
			}
		}

		items = filterByPhase(items, phase)
	}

	if len(items) == 0 {
		if rc.JSON {
			return rc.writeJSON([]listItemJSON{})
		}

		rc.printf("No items found.\n")

		return nil
	}

	groups := l.groupItems(rc, items)

	if rc.JSON {
		return l.runJSON(rc, groups)
	}

	return l.runText(rc, groups)
}

func filterByPhase(items []*item.Item, phase string) []*item.Item {
	var out []*item.Item

	for _, it := range items {
		if it.Phase == phase {
			out = append(out, it)
		}
	}

	return out
}

func (l *ListCmd) groupItems(rc *RunContext, items []*item.Item) []itemGroup {
	var active, ready, blocked, done []groupedItem

	for _, it := range items {
		st := string(it.Status)

		switch {
		case rc.Statuses.IsDone(st):
			done = append(done, groupedItem{item: it})
		case rc.Statuses.IsActive(st):
			active = append(active, groupedItem{item: it})
		default:
			if rc.Graph.AllDepsDone(it.ID) {
				ready = append(ready, groupedItem{item: it})
			} else {
				blocked = append(blocked, groupedItem{item: it, waitingOn: rc.Graph.WaitingOn(it.ID)})
			}
		}
	}

	sortByPriority(active)
	sortByPriority(ready)
	sortByPriority(blocked)
	sortByPriority(done)

	return []itemGroup{
		{"active", active},
		{"ready", ready},
		{"blocked", blocked},
		{"done", done},
	}
}

func sortByPriority(items []groupedItem) {
	sort.Slice(items, func(a, b int) bool {
		pa := item.PriorityRank(items[a].item.Priority)
		pb := item.PriorityRank(items[b].item.Priority)
		if pa != pb {
			return pa < pb
		}

		return items[a].item.ID < items[b].item.ID
	})
}

func (l *ListCmd) runText(rc *RunContext, groups []itemGroup) error {
	first := true

	for _, g := range groups {
		if len(g.items) == 0 {
			continue
		}

		if !first {
			rc.printf("\n")
		}

		first = false

		rc.printf("%s\n", strings.ToUpper(g.label))

		w := rc.newTabWriter()

		for _, gi := range g.items {
			if len(gi.waitingOn) > 0 {
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\twaiting on: %s\n",
					gi.item.ID, gi.item.Priority, gi.item.Type, gi.item.Title,
					strings.Join(gi.waitingOn, ", "))
			} else {
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
					gi.item.ID, gi.item.Priority, gi.item.Type, gi.item.Title)
			}
		}

		if err := w.Flush(); err != nil {
			return err
		}
	}

	return nil
}

func (l *ListCmd) runJSON(rc *RunContext, groups []itemGroup) error {
	var out []listItemJSON

	for _, g := range groups {
		for _, gi := range g.items {
			out = append(out, listItemJSON{
				ID:        gi.item.ID,
				Priority:  string(gi.item.Priority),
				Type:      gi.item.Type,
				Status:    string(gi.item.Status),
				Title:     gi.item.Title,
				Group:     g.label,
				WaitingOn: gi.waitingOn,
			})
		}
	}

	return rc.writeJSON(out)
}
