package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"
)

// TreeCmd renders the dependency graph as a column-based ASCII graph.
type TreeCmd struct{}

type treeNodeJSON struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	Column       int      `json:"column"`
	Dependencies []string `json:"dependencies"`
	Dependents   []string `json:"dependents"`
}

type treeOutputJSON struct {
	Nodes []treeNodeJSON `json:"nodes"`
	Edges []graphEdge    `json:"edges"`
}

type treeSlot struct {
	remaining int
}

type treeRow struct {
	id      string
	nodeCol int
	depCols map[int]bool
	closing map[int]bool
	active  map[int]bool
}

// charKind classifies graph characters for color rendering.
type charKind int

const (
	kindEmpty     charKind = iota // spaces
	kindConnected                 // node marker, dep junctions, horizontal bridges
	kindPassive                   // vertical threads, crossings
)

const (
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
	ansiReset = "\033[0m"
)

// Run executes the tree command.
func (t *TreeCmd) Run(rc *RunContext) error {
	layers := rc.Graph.TopologicalLayers()

	if len(layers) == 0 {
		if rc.JSON {
			return rc.writeJSON(treeOutputJSON{
				Nodes: []treeNodeJSON{},
				Edges: []graphEdge{},
			})
		}

		rc.printf("No items found.\n")

		return nil
	}

	var order []string
	for _, layer := range layers {
		order = append(order, layer...)
	}

	rows, nodeColumn, maxCol := t.assignColumns(rc, order)

	if rc.JSON {
		return t.renderJSON(rc, order, nodeColumn)
	}

	color := isTerminalWriter(rc.Stdout)

	return t.renderText(rc, rows, maxCol, color)
}

func (t *TreeCmd) assignColumns(
	rc *RunContext, order []string,
) (rows []treeRow, nodeColumn map[string]int, maxCol int) {
	columns := make([]*treeSlot, 0)
	nodeColumn = make(map[string]int, len(order))
	rows = make([]treeRow, 0, len(order))

	for _, id := range order {
		it, _ := rc.Graph.Item(id)
		numDependents := len(rc.Graph.DependentsOf(id))

		depCols := t.findDepColumns(it.Dependencies, nodeColumn)
		freeingCols := t.findFreeingColumns(depCols, columns)
		targetCol := t.chooseColumn(freeingCols, columns)

		active := snapshotActive(columns)

		closing := t.decrementAndClose(depCols, columns, targetCol)

		for len(columns) <= targetCol {
			columns = append(columns, nil)
		}

		if numDependents > 0 {
			columns[targetCol] = &treeSlot{remaining: numDependents}
		} else {
			columns[targetCol] = nil
		}

		nodeColumn[id] = targetCol
		if targetCol > maxCol {
			maxCol = targetCol
		}

		rows = append(rows, treeRow{
			id:      id,
			nodeCol: targetCol,
			depCols: depCols,
			closing: closing,
			active:  active,
		})
	}

	return rows, nodeColumn, maxCol
}

func (t *TreeCmd) findDepColumns(deps []string, nodeColumn map[string]int) map[int]bool {
	depCols := make(map[int]bool)

	for _, dep := range deps {
		if col, ok := nodeColumn[dep]; ok {
			depCols[col] = true
		}
	}

	return depCols
}

func (t *TreeCmd) findFreeingColumns(depCols map[int]bool, columns []*treeSlot) []int {
	var freeing []int

	for col := range depCols {
		if col < len(columns) && columns[col] != nil && columns[col].remaining == 1 {
			freeing = append(freeing, col)
		}
	}

	sort.Ints(freeing)

	return freeing
}

func (t *TreeCmd) chooseColumn(freeingCols []int, columns []*treeSlot) int {
	if len(freeingCols) > 0 {
		return freeingCols[0]
	}

	for i, s := range columns {
		if s == nil {
			return i
		}
	}

	return len(columns)
}

func snapshotActive(columns []*treeSlot) map[int]bool {
	active := make(map[int]bool, len(columns))

	for i, s := range columns {
		if s != nil {
			active[i] = true
		}
	}

	return active
}

func (t *TreeCmd) decrementAndClose(
	depCols map[int]bool, columns []*treeSlot, targetCol int,
) map[int]bool {
	closing := make(map[int]bool)

	for col := range depCols {
		if col >= len(columns) || columns[col] == nil {
			continue
		}

		columns[col].remaining--

		if columns[col].remaining == 0 && col != targetCol {
			closing[col] = true
			columns[col] = nil
		}
	}

	return closing
}

func (t *TreeCmd) renderText(rc *RunContext, rows []treeRow, maxCol int, color bool) error {
	graphWidth := maxCol*2 + 1

	for _, row := range rows {
		it, ok := rc.Graph.Item(row.id)
		if !ok {
			return fmt.Errorf("internal error: unknown item %q", row.id)
		}

		prefix := renderGraphPrefix(row, maxCol, color)
		label := fmt.Sprintf("%s  %s [%s]", it.ID, it.Title, string(it.Status))

		if graphWidth > 0 {
			rc.printf("%s %s\n", prefix, label)
		} else {
			rc.printf("%s\n", label)
		}
	}

	return nil
}

func renderGraphPrefix(row treeRow, maxCol int, color bool) string {
	spanLeft, spanRight := computeSpan(row)
	hasSpan := spanLeft < spanRight

	var buf strings.Builder

	lastKind := charKind(-1)

	writeColored := func(r rune, kind charKind) {
		if color && kind != lastKind {
			switch kind {
			case kindConnected:
				buf.WriteString(ansiBold)
			case kindPassive:
				buf.WriteString(ansiDim)
			case kindEmpty:
				buf.WriteString(ansiReset)
			}

			lastKind = kind
		}

		buf.WriteRune(r)
	}

	for col := 0; col <= maxCol; col++ {
		ch, kind := columnCharAndKind(col, row, spanLeft, spanRight, hasSpan)
		writeColored(ch, kind)

		if col < maxCol {
			if hasSpan && col >= spanLeft && col < spanRight {
				writeColored('─', kindConnected)
			} else {
				writeColored(' ', kindEmpty)
			}
		}
	}

	if color {
		buf.WriteString(ansiReset)
	}

	return buf.String()
}

func computeSpan(row treeRow) (spanLeft, spanRight int) {
	spanLeft = row.nodeCol
	spanRight = row.nodeCol

	for col := range row.depCols {
		if col < spanLeft {
			spanLeft = col
		}

		if col > spanRight {
			spanRight = col
		}
	}

	return spanLeft, spanRight
}

//nolint:cyclop // column character selection requires exhaustive case handling
func columnCharAndKind(col int, row treeRow, spanLeft, spanRight int, hasSpan bool) (rune, charKind) {
	switch {
	case col == row.nodeCol:
		return '●', kindConnected
	case row.depCols[col] && col < row.nodeCol:
		if row.closing[col] {
			return '└', kindConnected
		}

		return '├', kindConnected
	case row.depCols[col]:
		if row.closing[col] {
			return '┘', kindConnected
		}

		return '┤', kindConnected
	case hasSpan && col > spanLeft && col < spanRight:
		if row.active[col] {
			return '┼', kindPassive
		}

		return '─', kindConnected
	case row.active[col]:
		return '│', kindPassive
	default:
		return ' ', kindEmpty
	}
}

func isTerminalWriter(w interface{}) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(f.Fd())) //nolint:gosec // file descriptors fit in int
}

func (t *TreeCmd) renderJSON(rc *RunContext, order []string, nodeColumn map[string]int) error {
	nodes := make([]treeNodeJSON, 0, len(order))

	for _, id := range order {
		it, _ := rc.Graph.Item(id)

		deps := it.Dependencies
		if deps == nil {
			deps = []string{}
		}

		dependents := rc.Graph.DependentsOf(id)
		if dependents == nil {
			dependents = []string{}
		}

		nodes = append(nodes, treeNodeJSON{
			ID:           it.ID,
			Title:        it.Title,
			Status:       string(it.Status),
			Column:       nodeColumn[id],
			Dependencies: deps,
			Dependents:   dependents,
		})
	}

	graphEdges := rc.Graph.Edges()
	edges := make([]graphEdge, len(graphEdges))

	for i, e := range graphEdges {
		edges[i] = graphEdge{From: e.From, To: e.To}
	}

	return rc.writeJSON(treeOutputJSON{
		Nodes: nodes,
		Edges: edges,
	})
}
