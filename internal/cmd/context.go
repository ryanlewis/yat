// Package cmd implements the CLI commands for yat.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	graphpkg "github.com/ryanlewis/yat/internal/graph"
	"github.com/ryanlewis/yat/internal/item"
)

// RunContext holds shared state available to all commands.
type RunContext struct {
	Items    []*item.Item
	Graph    *graphpkg.Graph
	JSON     bool
	Dir      string
	ReadOnly bool
	Stdout   io.Writer
}

// NewRunContext loads items and builds the graph.
func NewRunContext(dir string, jsonOutput, readOnly bool) (*RunContext, error) {
	items, err := item.LoadAll(dir)
	if err != nil {
		return nil, err
	}

	if _, indexErr := item.Index(items); indexErr != nil {
		return nil, indexErr
	}

	g, err := graphpkg.Build(items)
	if err != nil {
		return nil, err
	}

	return &RunContext{
		Items:    items,
		Graph:    g,
		JSON:     jsonOutput,
		Dir:      dir,
		ReadOnly: readOnly,
		Stdout:   os.Stdout,
	}, nil
}

func (rc *RunContext) writeJSON(v any) error {
	enc := json.NewEncoder(rc.Stdout)
	enc.SetIndent("", "  ")

	return enc.Encode(v)
}

func (rc *RunContext) newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(rc.Stdout, 0, 0, 2, ' ', 0)
}

func (rc *RunContext) printf(format string, args ...any) {
	fmt.Fprintf(rc.Stdout, format, args...)
}
