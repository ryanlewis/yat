package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryanlewis/yat/internal/config"
	graphpkg "github.com/ryanlewis/yat/internal/graph"
	"github.com/ryanlewis/yat/internal/item"
)

func makeTestItems() []*item.Item {
	return []*item.Item{
		{ID: "SP-001", Title: "Spike", Type: "Spike", Priority: item.PriorityCritical, Status: item.StatusDone, Dependencies: nil},
		{ID: "TK-001", Title: "Scaffolding", Type: "Task", Priority: item.PriorityCritical, Status: item.StatusDone, Dependencies: nil},
		{ID: "TK-002", Title: "Abstraction", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusTodo, Dependencies: []string{"TK-001", "SP-001"}},
		{ID: "TK-003", Title: "Error handling", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusTodo, Dependencies: []string{"TK-001"}},
		{ID: "ST-001", Title: "Command parsing", Type: "Story", Priority: item.PriorityCritical, Status: item.StatusTodo, Dependencies: []string{"TK-001", "TK-003"}},
		{ID: "ST-002", Title: "Profile parsing", Type: "Story", Priority: item.PriorityHigh, Status: item.StatusTodo, Dependencies: []string{"TK-003", "TK-002"}},
	}
}

func defaultConfig() *config.Config {
	var cfg config.Config
	cfg.Defaults()
	return &cfg
}

var defaultStatuses = config.StatusGroups{
	Done:    []string{"done"},
	Active:  []string{"in-progress"},
	Initial: []string{"todo"},
}

func newTestContext(t *testing.T, items []*item.Item, jsonMode bool) (*RunContext, *bytes.Buffer) {
	t.Helper()

	isDone := func(s item.Status) bool { return defaultStatuses.IsDone(string(s)) }
	isActive := func(s item.Status) bool { return defaultStatuses.IsActive(string(s)) }

	g, err := graphpkg.Build(items, graphpkg.WithIsDone(isDone), graphpkg.WithIsActive(isActive))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var buf bytes.Buffer

	return &RunContext{
		Items:       items,
		Graph:       g,
		JSON:        jsonMode,
		Statuses:    defaultStatuses,
		StatusField: "status",
		Stdout:      &buf,
	}, &buf
}

func writeItemFile(t *testing.T, dir, filename, content string) string {
	t.Helper()

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	return path
}

func customStatuses() config.StatusGroups {
	return config.StatusGroups{
		Done:    []string{"done", "shipped"},
		Active:  []string{"in-progress", "review"},
		Initial: []string{"backlog"},
	}
}

func newCustomTestContext(t *testing.T, items []*item.Item, jsonMode bool) (*RunContext, *bytes.Buffer) {
	t.Helper()

	isDone := func(s item.Status) bool { return s == "done" || s == "shipped" }
	g, err := graphpkg.Build(items, graphpkg.WithIsDone(isDone))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var buf bytes.Buffer

	return &RunContext{
		Items:       items,
		Graph:       g,
		JSON:        jsonMode,
		Statuses:    customStatuses(),
		StatusField: "state",
		Stdout:      &buf,
	}, &buf
}
