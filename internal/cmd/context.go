// Package cmd implements the CLI commands for yat.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
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

	// statusIsAlias is set when another field is aliased to "status", so a
	// status key in a file holds that field, not the status.
	statusIsAlias bool

	// fieldAliases maps canonical field names to the keys item files use.
	fieldAliases map[string]string

	// unlock releases the items directory's lock, when rc holds it.
	unlock func()

	// skipped lists the files and folders the load skipped as unreadable.
	skipped []string
}

// statusKey is the canonical frontmatter key for an item's status.
const statusKey = "status"

// NewRunContext loads items and builds the graph.
func NewRunContext(dir string, jsonOutput bool, cfg *config.Config) (*RunContext, error) {
	return newRunContext(dir, jsonOutput, cfg, false)
}

// NewLockedRunContext takes the items directory's lock, then loads items and
// builds the graph under it, for a command that checks items and then writes:
// it sees what other yat commands wrote, and none writes in between. The lock
// is held until rc.Unlock is called.
func NewLockedRunContext(dir string, jsonOutput bool, cfg *config.Config) (*RunContext, error) {
	return newRunContext(dir, jsonOutput, cfg, true)
}

func newRunContext(dir string, jsonOutput bool, cfg *config.Config, lock bool) (*RunContext, error) {
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

	isDone := func(s item.Status) bool {
		return cfg.Statuses.IsDone(string(s))
	}
	isActive := func(s item.Status) bool {
		return cfg.Statuses.IsActive(string(s))
	}

	buildOpts := []graphpkg.BuildOption{graphpkg.WithIsDone(isDone), graphpkg.WithIsActive(isActive)}
	if len(cfg.Phases) > 0 {
		buildOpts = append(buildOpts, graphpkg.WithPhaseOrder(cfg.Phases))
	}

	unlock := func() {}

	if lock {
		var err error
		if unlock, err = item.LockDir(dir); err != nil {
			return nil, err
		}
	}

	items, skipped, g, err := loadItems(dir, parseOpts, buildOpts)
	if err != nil {
		unlock()

		return nil, err
	}

	statusField := statusKey
	if alias, ok := cfg.FieldAliases[statusKey]; ok {
		statusField = alias
	}

	statusIsAlias := aliases[statusKey] != "" && aliases[statusKey] != statusKey

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

		statusIsAlias: statusIsAlias,
		fieldAliases:  cfg.FieldAliases,
		unlock:        unlock,
		skipped:       skipped,
	}, nil
}

// filesWere returns "1 file was" or "<n> files were".
func filesWere(n int) string {
	if n == 1 {
		return "1 file was"
	}

	return fmt.Sprintf("%d files were", n)
}

// loadItems loads the items in dir, checks no two share an id and builds their
// graph. It also returns the files and folders the load skipped as unreadable.
func loadItems(
	dir string, parseOpts item.ParseOptions, buildOpts []graphpkg.BuildOption,
) ([]*item.Item, []string, *graphpkg.Graph, error) {
	items, skipped, err := item.LoadAllWithOptions(dir, parseOpts)
	if err != nil {
		return nil, nil, nil, err
	}

	if _, err = item.Index(items); err != nil {
		return nil, nil, nil, err
	}

	g, err := graphpkg.Build(items, buildOpts...)
	if err != nil {
		if errors.Is(err, graphpkg.ErrUnknownDependency) && len(skipped) > 0 {
			err = fmt.Errorf("%w (%s skipped as unreadable; see warnings)", err, filesWere(len(skipped)))
		}

		return nil, nil, nil, err
	}

	return items, skipped, g, nil
}

// Unlock releases the items directory's lock if rc holds it, as a context from
// NewLockedRunContext does. Commands that write call it before printing, so a
// reader slow to drain stdout does not hold up other yat commands. Calling it
// again does nothing.
func (rc *RunContext) Unlock() {
	if rc.unlock != nil {
		rc.unlock()
		rc.unlock = nil
	}
}

// mutateOptions returns the options that write the item's status under the key
// its file uses, as parse found it.
func (rc *RunContext) mutateOptions(it *item.Item) (item.MutateOptions, error) {
	if it.StatusKey == "" {
		keys := rc.StatusField
		if !rc.statusIsAlias && keys != statusKey {
			keys = statusKey + " or " + keys
		}

		return item.MutateOptions{}, fmt.Errorf("no %s field found in frontmatter of %s", keys, it.FilePath)
	}

	return item.MutateOptions{
		FieldName:     it.StatusKey,
		ValidStatuses: item.NewStatusSet(rc.Statuses.AllValid()),
	}, nil
}

func (rc *RunContext) writeJSON(v any) error {
	enc := json.NewEncoder(rc.Stdout)
	enc.SetIndent("", "  ")

	return enc.Encode(v)
}

func formatExtra(extra map[string]interface{}) string {
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %v", k, extra[k]))
	}

	return strings.Join(parts, "  |  ")
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
