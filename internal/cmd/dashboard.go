package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ryanlewis/yat/internal/item"
)

// dashboardReadyLimit caps how many ready items the dashboard lists.
const dashboardReadyLimit = 5

// dashboardInProgressLimit caps how many in-progress items the dashboard lists.
const dashboardInProgressLimit = 5

// DashboardCmd shows a compact "orient me" view. It is what bare `yat` runs.
type DashboardCmd struct{}

type dashboardJSON struct {
	ProjectFound   bool                `json:"project_found"`
	Dir            string              `json:"dir"`
	Counts         dashboardCountsJSON `json:"counts"`
	InProgress     []readyJSON         `json:"in_progress"`
	InProgressMore int                 `json:"in_progress_more"`
	Ready          []readyJSON         `json:"ready"`
	ReadyMore      int                 `json:"ready_more"`
	Phase          *dashboardPhaseJSON `json:"phase"`
}

type dashboardCountsJSON struct {
	Total      int `json:"total"`
	InProgress int `json:"in_progress"`
	Ready      int `json:"ready"`
	Blocked    int `json:"blocked"`
	Done       int `json:"done"`
}

type dashboardPhaseJSON struct {
	Name        string `json:"name"`
	DonePoints  int    `json:"done_points"`
	TotalPoints int    `json:"total_points"`
}

type noProjectJSON struct {
	ProjectFound bool   `json:"project_found"`
	Dir          string `json:"dir"`
	Hint         string `json:"hint"`
}

// Run executes the dashboard command.
func (d *DashboardCmd) Run(rc *RunContext) error {
	out := d.collect(rc)

	if rc.JSON {
		return rc.writeJSON(out)
	}

	return d.writeText(rc, &out)
}

func (d *DashboardCmd) collect(rc *RunContext) dashboardJSON {
	var counts dashboardCountsJSON
	var inProgress []*item.Item

	counts.Total = len(rc.Items)

	for _, it := range rc.Items {
		st := string(it.Status)

		switch {
		case rc.Statuses.IsDone(st):
			counts.Done++
		case rc.Statuses.IsActive(st):
			counts.InProgress++
			inProgress = append(inProgress, it)
		case !rc.Graph.AllDepsDone(it.ID):
			counts.Blocked++
		}
	}

	sort.Slice(inProgress, func(a, b int) bool {
		pa, pb := item.PriorityRank(inProgress[a].Priority), item.PriorityRank(inProgress[b].Priority)
		if pa != pb {
			return pa < pb
		}

		return inProgress[a].ID < inProgress[b].ID
	})

	ready := rc.Graph.Ready()
	counts.Ready = len(ready)

	shown := ready
	if len(shown) > dashboardReadyLimit {
		shown = shown[:dashboardReadyLimit]
	}

	active := inProgress
	if len(active) > dashboardInProgressLimit {
		active = active[:dashboardInProgressLimit]
	}

	return dashboardJSON{
		ProjectFound:   true,
		Dir:            rc.Dir,
		Counts:         counts,
		InProgress:     toReadyJSON(active),
		InProgressMore: len(inProgress) - len(active),
		Ready:          toReadyJSON(shown),
		ReadyMore:      len(ready) - len(shown),
		Phase:          d.phase(rc),
	}
}

func (d *DashboardCmd) phase(rc *RunContext) *dashboardPhaseJSON {
	name := rc.Graph.ActivePhase()
	if name == "" {
		return nil
	}

	p := &dashboardPhaseJSON{Name: name}

	for _, it := range rc.Items {
		if it.Phase != name {
			continue
		}

		p.TotalPoints += it.Points
		if rc.Statuses.IsDone(string(it.Status)) {
			p.DonePoints += it.Points
		}
	}

	return p
}

func toReadyJSON(items []*item.Item) []readyJSON {
	out := make([]readyJSON, len(items))
	for i, it := range items {
		out[i] = readyJSON{
			ID:       it.ID,
			Priority: string(it.Priority),
			Type:     it.Type,
			Status:   string(it.Status),
			Title:    it.Title,
		}
	}

	return out
}

func (d *DashboardCmd) writeText(rc *RunContext, out *dashboardJSON) error {
	s := rc.styles()
	c := out.Counts

	dir := out.Dir
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}

	rc.printf("%s · %s · %d items (%d in progress, %d ready, %d blocked, %d done)\n",
		s.Bold("yat"), dir, c.Total, c.InProgress, c.Ready, c.Blocked, c.Done)

	if len(out.InProgress) > 0 {
		rc.printf("\nIn progress:\n")
		d.writeItems(rc, out.InProgress)

		if out.InProgressMore > 0 {
			rc.printf("  ...and %d more — run `yat list`\n", out.InProgressMore)
		}
	}

	rc.printf("\nUp next (ready):\n")

	if len(out.Ready) == 0 {
		rc.printf("  No items are ready.\n")
	} else {
		d.writeItems(rc, out.Ready)
	}

	if out.ReadyMore > 0 {
		rc.printf("  ...and %d more — run `yat ready`\n", out.ReadyMore)
	}

	if out.Phase != nil {
		rc.printf("\nPhase: %s (%d/%d pts done)\n", out.Phase.Name, out.Phase.DonePoints, out.Phase.TotalPoints)
	}

	return nil
}

func (d *DashboardCmd) writeItems(rc *RunContext, items []readyJSON) {
	s := rc.styles()

	var maxID, maxTitle int
	for _, it := range items {
		maxID = max(maxID, displayWidth(it.ID))
		maxTitle = max(maxTitle, displayWidth(oneLine(it.Title)))
	}

	for _, it := range items {
		title := oneLine(it.Title)

		if it.Priority == "" {
			fmt.Fprintf(rc.Stdout, "  %s  %s\n", padWidth(it.ID, maxID), title)

			continue
		}

		fmt.Fprintf(rc.Stdout, "  %s  %s  [%s]\n",
			padWidth(it.ID, maxID), padWidth(title, maxTitle), s.Priority(item.Priority(it.Priority)))
	}
}

// WriteNoProject prints the onboarding hint shown when bare `yat` runs
// outside a project, i.e. when the items directory does not exist.
func WriteNoProject(w io.Writer, jsonOutput bool, dir string) error {
	hint := "Run `yat init` to create one, or use --dir to point at an existing items directory."

	if jsonOutput {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")

		return enc.Encode(noProjectJSON{ProjectFound: false, Dir: dir, Hint: hint})
	}

	_, err := fmt.Fprintf(w, "No yat project found here (no %q directory).\n%s\n", dir, hint)

	return err
}
