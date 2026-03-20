package cmd

import (
	"fmt"
	"strings"
)

// StatusCmd shows an overview of all items.
type StatusCmd struct{}

type statusJSON struct {
	TotalItems   int                    `json:"total_items"`
	TotalPoints  int                    `json:"total_points"`
	ByType       map[string]int         `json:"by_type"`
	ByStatus     map[string]statusGroup `json:"by_status"`
	ReadyNow     []string               `json:"ready_now"`
	DeepestLayer int                    `json:"deepest_layer"`
	DeepestItems []string               `json:"deepest_items"`
}

type statusGroup struct {
	Count  int `json:"count"`
	Points int `json:"points"`
}

// Run executes the status command.
func (s *StatusCmd) Run(rc *RunContext) error {
	byType := make(map[string]int)
	var totalPoints int

	type groupAccum struct {
		count  int
		points int
	}

	groups := map[string]*groupAccum{
		"done":    {},
		"active":  {},
		"initial": {},
	}

	for _, it := range rc.Items {
		byType[it.Type]++
		totalPoints += it.Points

		st := string(it.Status)
		switch {
		case rc.Statuses.IsDone(st):
			groups["done"].count++
			groups["done"].points += it.Points
		case rc.Statuses.IsActive(st):
			groups["active"].count++
			groups["active"].points += it.Points
		case rc.Statuses.IsInitial(st):
			groups["initial"].count++
			groups["initial"].points += it.Points
		default:
			return fmt.Errorf("unexpected status %q for item %s", it.Status, it.ID)
		}
	}

	ready := rc.Graph.Ready()
	readyIDs := make([]string, len(ready))

	for i, it := range ready {
		readyIDs[i] = it.ID
	}

	deepestLayer, deepestItems := rc.Graph.DeepestLayer()

	if rc.JSON {
		byStatus := make(map[string]statusGroup, len(groups))
		for name, g := range groups {
			byStatus[name] = statusGroup{Count: g.count, Points: g.points}
		}

		return rc.writeJSON(statusJSON{
			TotalItems:   len(rc.Items),
			TotalPoints:  totalPoints,
			ByType:       byType,
			ByStatus:     byStatus,
			ReadyNow:     readyIDs,
			DeepestLayer: deepestLayer,
			DeepestItems: deepestItems,
		})
	}

	// Summary line
	total := len(rc.Items)
	typeParts := formatTypeCounts(byType)
	rc.printf("%d items: %s\n", total, strings.Join(typeParts, ", "))
	rc.printf("Total points: %d\n\n", totalPoints)

	// Status breakdown — use group labels with representative status names
	w := rc.newTabWriter()
	for _, entry := range []struct {
		label string
		key   string
	}{
		{"done", "done"},
		{"active", "active"},
		{"initial", "initial"},
	} {
		g := groups[entry.key]
		fmt.Fprintf(w, "  %s:\t%d\t(%d pts)\n", entry.label, g.count, g.points)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	rc.printf("\n")

	if len(readyIDs) > 0 {
		rc.printf("Ready now: %s\n", strings.Join(readyIDs, ", "))
	}

	if len(deepestItems) > 0 {
		rc.printf("Deepest layer: %d (%s)\n", deepestLayer, strings.Join(deepestItems, ", "))
	}

	return nil
}

func pluralizeType(t string) string {
	lower := strings.ToLower(t)
	if len(lower) >= 2 && lower[len(lower)-1] == 'y' {
		// Only consonant+y gets -ies (e.g. story→stories).
		// Vowel+y just gets -s (e.g. deploy→deploys).
		beforeY := lower[len(lower)-2]
		if beforeY != 'a' && beforeY != 'e' && beforeY != 'i' && beforeY != 'o' && beforeY != 'u' {
			return lower[:len(lower)-1] + "ies"
		}
	}

	return lower + "s"
}

func formatTypeCounts(byType map[string]int) []string {
	order := []string{"Spike", "Task", "Story"}
	var parts []string

	for _, t := range order {
		if c, ok := byType[t]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", c, pluralizeType(t)))
		}
	}

	// Include any types not in the standard order
	for t, c := range byType {
		found := false

		for _, o := range order {
			if t == o {
				found = true

				break
			}
		}

		if !found {
			parts = append(parts, fmt.Sprintf("%d %s", c, pluralizeType(t)))
		}
	}

	return parts
}
