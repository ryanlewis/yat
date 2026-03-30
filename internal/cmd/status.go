package cmd

import (
	"fmt"
	"sort"
	"strings"
)

// StatusCmd shows an overview of all items.
type StatusCmd struct{}

type statusJSON struct {
	TotalItems   int                    `json:"total_items"`
	TotalPoints  int                    `json:"total_points"`
	ByType       map[string]int         `json:"by_type"`
	ByStatus     map[string]statusGroup `json:"by_status"`
	ActivePhase  string                 `json:"active_phase"`
	ByPhase      map[string]statusGroup `json:"by_phase"`
	ReadyNow     []string               `json:"ready_now"`
	DeepestLayer int                    `json:"deepest_layer"`
	DeepestItems []string               `json:"deepest_items"`
}

type statusGroup struct {
	Count  int `json:"count"`
	Points int `json:"points"`
}

type statusData struct {
	totalItems   int
	totalPoints  int
	byType       map[string]int
	groups       map[string]*groupAccum
	byPhase      map[string]*groupAccum
	activePhase  string
	readyIDs     []string
	deepestLayer int
	deepestItems []string
}

type groupAccum struct {
	count  int
	points int
}

// Run executes the status command.
func (s *StatusCmd) Run(rc *RunContext) error {
	data, err := s.collect(rc)
	if err != nil {
		return err
	}

	if rc.JSON {
		return s.writeJSON(rc, data)
	}

	return s.writeText(rc, data)
}

func (s *StatusCmd) collect(rc *RunContext) (*statusData, error) {
	byType := make(map[string]int)
	var totalPoints int

	groups := map[string]*groupAccum{
		"done":    {},
		"active":  {},
		"initial": {},
	}

	byPhase := make(map[string]*groupAccum)

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
			return nil, fmt.Errorf("unexpected status %q for item %s", it.Status, it.ID)
		}

		if it.Phase != "" {
			if byPhase[it.Phase] == nil {
				byPhase[it.Phase] = &groupAccum{}
			}

			byPhase[it.Phase].count++
			byPhase[it.Phase].points += it.Points
		}
	}

	ready := rc.Graph.Ready()
	readyIDs := make([]string, len(ready))

	for i, it := range ready {
		readyIDs[i] = it.ID
	}

	deepestLayer, deepestItems := rc.Graph.DeepestLayer()

	return &statusData{
		totalItems:   len(rc.Items),
		totalPoints:  totalPoints,
		byType:       byType,
		groups:       groups,
		byPhase:      byPhase,
		activePhase:  rc.Graph.ActivePhase(),
		readyIDs:     readyIDs,
		deepestLayer: deepestLayer,
		deepestItems: deepestItems,
	}, nil
}

func toStatusGroups(m map[string]*groupAccum) map[string]statusGroup {
	out := make(map[string]statusGroup, len(m))
	for name, g := range m {
		out[name] = statusGroup{Count: g.count, Points: g.points}
	}

	return out
}

func (s *StatusCmd) writeJSON(rc *RunContext, d *statusData) error {
	return rc.writeJSON(statusJSON{
		TotalItems:   d.totalItems,
		TotalPoints:  d.totalPoints,
		ByType:       d.byType,
		ByStatus:     toStatusGroups(d.groups),
		ActivePhase:  d.activePhase,
		ByPhase:      toStatusGroups(d.byPhase),
		ReadyNow:     d.readyIDs,
		DeepestLayer: d.deepestLayer,
		DeepestItems: d.deepestItems,
	})
}

func (s *StatusCmd) writeText(rc *RunContext, d *statusData) error {
	typeParts := formatTypeCounts(d.byType)
	rc.printf("%d items: %s\n", d.totalItems, strings.Join(typeParts, ", "))
	rc.printf("Total points: %d\n\n", d.totalPoints)

	w := rc.newTabWriter()
	for _, entry := range []struct {
		label string
		key   string
	}{
		{"done", "done"},
		{"active", "active"},
		{"initial", "initial"},
	} {
		g := d.groups[entry.key]
		fmt.Fprintf(w, "  %s:\t%d\t(%d pts)\n", entry.label, g.count, g.points)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	if len(d.byPhase) > 0 {
		rc.printf("\nPhases:\n")

		phaseNames := make([]string, 0, len(d.byPhase))
		for name := range d.byPhase {
			phaseNames = append(phaseNames, name)
		}

		sort.Strings(phaseNames)

		pw := rc.newTabWriter()

		for _, name := range phaseNames {
			g := d.byPhase[name]
			marker := ""

			if name == d.activePhase {
				marker = " *"
			}

			fmt.Fprintf(pw, "  %s:\t%d\t(%d pts)%s\n", name, g.count, g.points, marker)
		}

		if err := pw.Flush(); err != nil {
			return err
		}
	}

	rc.printf("\n")

	if len(d.readyIDs) > 0 {
		rc.printf("Ready now: %s\n", strings.Join(d.readyIDs, ", "))
	}

	if len(d.deepestItems) > 0 {
		rc.printf("Deepest layer: %d (%s)\n", d.deepestLayer, strings.Join(d.deepestItems, ", "))
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
