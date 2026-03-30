// Package cmd implements the CLI commands for yat.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/ryanlewis/yat/internal/config"
	graphpkg "github.com/ryanlewis/yat/internal/graph"
	"github.com/ryanlewis/yat/internal/item"
)

// RunContext holds shared state available to all commands.
type RunContext struct {
	Items       []*item.Item
	Graph       *graphpkg.Graph
	JSON        bool
	Dir         string
	ReadOnly    bool
	Stdout      io.Writer
	Styles      *Styles
	Statuses    config.StatusGroups
	StatusField string // "status" or the configured alias
}

// NewRunContext loads items and builds the graph.
func NewRunContext(dir string, jsonOutput bool, cfg *config.Config) (*RunContext, error) {
	// Invert field aliases: config stores canonical→alias, parse needs alias→canonical.
	aliases := make(map[string]string, len(cfg.FieldAliases))
	for canonical, alias := range cfg.FieldAliases {
		aliases[alias] = canonical
	}

	validStatuses := item.NewStatusSet(cfg.Statuses.AllValid())
	defaultStatus := item.Status(cfg.Statuses.DefaultInitial())

	parseOpts := item.ParseOptions{
		Aliases:       aliases,
		ValidStatuses: validStatuses,
		DefaultStatus: defaultStatus,
	}

	items, err := item.LoadAllWithOptions(dir, parseOpts)
	if err != nil {
		return nil, err
	}

	if _, indexErr := item.Index(items); indexErr != nil {
		return nil, indexErr
	}

	isDone := func(s item.Status) bool {
		return cfg.Statuses.IsDone(string(s))
	}

	buildOpts := []graphpkg.BuildOption{graphpkg.WithIsDone(isDone)}
	if len(cfg.Phases) > 0 {
		buildOpts = append(buildOpts, graphpkg.WithPhaseOrder(cfg.Phases))
	}

	g, err := graphpkg.Build(items, buildOpts...)
	if err != nil {
		return nil, err
	}

	statusField := "status"
	if alias, ok := cfg.FieldAliases["status"]; ok {
		statusField = alias
	}

	return &RunContext{
		Items:       items,
		Graph:       g,
		JSON:        jsonOutput,
		Dir:         dir,
		ReadOnly:    cfg.ReadOnly,
		Stdout:      os.Stdout,
		Styles:      newStyles(os.Stdout, cfg.Statuses),
		Statuses:    cfg.Statuses,
		StatusField: statusField,
	}, nil
}

func (rc *RunContext) mutateOptions() item.MutateOptions {
	return item.MutateOptions{
		FieldName:     rc.StatusField,
		ValidStatuses: item.NewStatusSet(rc.Statuses.AllValid()),
	}
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

func (rc *RunContext) styles() *Styles {
	if rc.Styles == nil {
		rc.Styles = newStyles(rc.Stdout, rc.Statuses)
	}

	return rc.Styles
}
