package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/config"
	graphpkg "github.com/ryanlewis/yat/internal/graph"
	"github.com/ryanlewis/yat/internal/item"
)

func makeTestItems() []*item.Item {
	return []*item.Item{
		{ID: "SP-001", Title: "Spike", Type: "Spike", Priority: item.PriorityCritical, Status: item.StatusDone, Dependencies: nil},
		{ID: "TK-001", Title: "Scaffolding", Type: "Task", Priority: item.PriorityCritical, Status: item.StatusDone, Dependencies: nil},
		{ID: "TK-002", Title: "Abstraction", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusDraft, Dependencies: []string{"TK-001", "SP-001"}},
		{ID: "TK-003", Title: "Error handling", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusDraft, Dependencies: []string{"TK-001"}},
		{ID: "ST-001", Title: "Command parsing", Type: "Story", Priority: item.PriorityCritical, Status: item.StatusDraft, Dependencies: []string{"TK-001", "TK-003"}},
		{ID: "ST-002", Title: "Profile parsing", Type: "Story", Priority: item.PriorityHigh, Status: item.StatusDraft, Dependencies: []string{"TK-003", "TK-002"}},
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
	Initial: []string{"draft"},
}

func newTestContext(t *testing.T, items []*item.Item, jsonMode bool) (*RunContext, *bytes.Buffer) {
	t.Helper()

	g, err := graphpkg.Build(items)
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

// --- ReadyCmd ---

func TestReadyCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &ReadyCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TK-002") {
		t.Errorf("expected TK-002 in ready output, got:\n%s", out)
	}
	if !strings.Contains(out, "TK-003") {
		t.Errorf("expected TK-003 in ready output, got:\n%s", out)
	}
}

func TestReadyCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &ReadyCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []readyJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 ready items, got %d", len(result))
	}
}

func TestReadyCmd_Empty(t *testing.T) {
	// All items done
	items := []*item.Item{
		{ID: "TK-001", Status: item.StatusDone},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &ReadyCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items are ready") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}

// --- NextCmd ---

func TestNextCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &NextCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	// TK-002 and TK-003 are both High priority; TK-002 sorts first by ID
	if !strings.Contains(out, "TK-002") {
		t.Errorf("expected TK-002 as next item, got:\n%s", out)
	}
}

func TestNextCmd_Empty(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Status: item.StatusDone},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &NextCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items are ready") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}

// --- ShowCmd ---

func TestShowCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &ShowCmd{ID: "TK-003"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TK-003") || !strings.Contains(out, "Error handling") {
		t.Errorf("expected TK-003 details, got:\n%s", out)
	}
}

func TestShowCmd_NotFound(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)

	cmd := &ShowCmd{ID: "NONEXISTENT"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
}

func TestShowCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &ShowCmd{ID: "ST-001"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result showJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if result.ID != "ST-001" {
		t.Errorf("ID = %q, want ST-001", result.ID)
	}
	if len(result.Dependencies) != 2 {
		t.Errorf("Dependencies = %v, want 2 items", result.Dependencies)
	}
}

// --- ReadOnly ---

func TestStartCmd_ReadOnly(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)
	rc.ReadOnly = true

	cmd := &StartCmd{ID: "TK-003"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for read-only project")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("expected 'read-only' in error, got: %v", err)
	}
}

func TestCompleteCmd_ReadOnly(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)
	rc.ReadOnly = true

	cmd := &CompleteCmd{ID: "TK-001"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for read-only project")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("expected 'read-only' in error, got: %v", err)
	}
}

// --- StartCmd ---

func TestStartCmd_Blocked(t *testing.T) {
	items := makeTestItems()
	// ST-001 depends on TK-001 (done) and TK-003 (draft) → blocked
	rc, _ := newTestContext(t, items, false)

	cmd := &StartCmd{ID: "ST-001"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for blocked item")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected 'blocked' in error, got: %v", err)
	}
}

func TestStartCmd_AlreadyDone(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)

	cmd := &StartCmd{ID: "SP-001"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for already-done item")
	}
	if !strings.Contains(err.Error(), "already done") {
		t.Errorf("expected 'already done' in error, got: %v", err)
	}
}

func TestStartCmd_AlreadyInProgress(t *testing.T) {
	items := makeTestItems()
	items[3].Status = item.StatusInProgress // TK-003, deps are done
	rc, buf := newTestContext(t, items, false)

	cmd := &StartCmd{ID: "TK-003"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "already in progress") {
		t.Errorf("expected 'already in progress' message, got:\n%s", buf.String())
	}
}

func TestStartCmd_NotFound(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)

	cmd := &StartCmd{ID: "NONEXISTENT"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
}

// --- CompleteCmd ---

func TestCompleteCmd_AlreadyDone(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "SP-001"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for already-done item")
	}
	if !strings.Contains(err.Error(), "already done") {
		t.Errorf("expected 'already done' in error, got: %v", err)
	}
}

func TestCompleteCmd_NotFound(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "NONEXISTENT"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
}

// --- BlockedCmd ---

func TestBlockedCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &BlockedCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	// ST-001 and ST-002 are blocked (their deps aren't all done)
	if !strings.Contains(out, "ST-001") {
		t.Errorf("expected ST-001 in blocked output, got:\n%s", out)
	}
	if !strings.Contains(out, "ST-002") {
		t.Errorf("expected ST-002 in blocked output, got:\n%s", out)
	}
}

func TestBlockedCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &BlockedCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []blockedItemJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 blocked items, got %d", len(result))
	}

	// Each blocked item should have non-empty WaitingOn
	for _, item := range result {
		if len(item.WaitingOn) == 0 {
			t.Errorf("blocked item %s has empty WaitingOn", item.ID)
		}
	}
}

func TestBlockedCmd_NoneBlocked(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Status: item.StatusDraft},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &BlockedCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items are blocked") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}

// --- StatusCmd ---

func TestStatusCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "6 items") {
		t.Errorf("expected '6 items' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "done:") {
		t.Errorf("expected 'done:' in output, got:\n%s", out)
	}
}

func TestStatusCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result statusJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if result.TotalItems != 6 {
		t.Errorf("TotalItems = %d, want 6", result.TotalItems)
	}
	if result.ByStatus["done"].Count != 2 {
		t.Errorf("ByStatus[done].Count = %d, want 2", result.ByStatus["done"].Count)
	}
	if result.ByStatus["initial"].Count != 4 {
		t.Errorf("ByStatus[initial].Count = %d, want 4", result.ByStatus["initial"].Count)
	}
}

// --- GraphCmd ---

func TestGraphCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &GraphCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Layer 0:") {
		t.Errorf("expected 'Layer 0:' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "->") {
		t.Errorf("expected edges in output, got:\n%s", out)
	}
}

func TestGraphCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &GraphCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result graphJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result.Layers) < 3 {
		t.Errorf("expected at least 3 layers, got %d", len(result.Layers))
	}
	if len(result.Edges) == 0 {
		t.Error("expected edges in graph JSON")
	}
}

func TestGraphCmd_Empty(t *testing.T) {
	rc, buf := newTestContext(t, []*item.Item{}, false)

	cmd := &GraphCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items found") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}

// --- StartCmd (happy path) ---

func writeItemFile(t *testing.T, dir, filename, content string) string {
	t.Helper()

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	return path
}

func TestStartCmd_HappyPath_Text(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "tk-002.md", `---
id: TK-002
title: "Abstraction"
type: Task
priority: High
status: draft
dependencies: [TK-001]
---

Task body.
`)
	items := []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-002", Title: "Abstraction", Status: item.StatusDraft, Dependencies: []string{"TK-001"}, FilePath: path, Body: "Task body.\n"},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &StartCmd{ID: "TK-002"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Started TK-002") {
		t.Errorf("expected 'Started TK-002', got:\n%s", out)
	}
	if !strings.Contains(out, "Task body.") {
		t.Errorf("expected body in output, got:\n%s", out)
	}

	// Verify file was updated on disc
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !strings.Contains(string(data), "status: in-progress") {
		t.Errorf("file not updated, got:\n%s", string(data))
	}
}

func TestStartCmd_HappyPath_JSON(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "tk-002.md", `---
id: TK-002
title: "Abstraction"
type: Task
priority: High
status: draft
dependencies: [TK-001]
---

Task body.
`)
	items := []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-002", Title: "Abstraction", Status: item.StatusDraft, Dependencies: []string{"TK-001"}, FilePath: path, Body: "Task body.\n"},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &StartCmd{ID: "TK-002"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result startJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ID != "TK-002" {
		t.Errorf("ID = %q, want TK-002", result.ID)
	}
	if result.Title != "Abstraction" {
		t.Errorf("Title = %q, want Abstraction", result.Title)
	}
	if result.Body != "Task body.\n" {
		t.Errorf("Body = %q", result.Body)
	}
}

// --- CompleteCmd (happy path) ---

func TestCompleteCmd_HappyPath_Text(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "tk-003.md", `---
id: TK-003
title: "Error handling"
type: Task
priority: High
status: draft
dependencies: [TK-001]
---
`)
	items := []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-003", Title: "Error handling", Status: item.StatusDraft, Priority: item.PriorityHigh, Dependencies: []string{"TK-001"}, FilePath: path},
		{ID: "ST-001", Title: "Command parsing", Status: item.StatusDraft, Dependencies: []string{"TK-001", "TK-003"}},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "TK-003"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Completed TK-003") {
		t.Errorf("expected 'Completed TK-003', got:\n%s", out)
	}
	// ST-001 depends on TK-001 (done) and TK-003 (now done) → should be unblocked
	if !strings.Contains(out, "Now unblocked") {
		t.Errorf("expected 'Now unblocked' section, got:\n%s", out)
	}
	if !strings.Contains(out, "ST-001") {
		t.Errorf("expected ST-001 in unblocked list, got:\n%s", out)
	}

	// Verify file was updated on disc
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !strings.Contains(string(data), "status: done") {
		t.Errorf("file not updated, got:\n%s", string(data))
	}
}

func TestCompleteCmd_HappyPath_JSON(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "tk-003.md", `---
id: TK-003
title: "Error handling"
type: Task
priority: High
status: draft
dependencies: [TK-001]
---
`)
	items := []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-003", Title: "Error handling", Status: item.StatusDraft, Priority: item.PriorityHigh, Dependencies: []string{"TK-001"}, FilePath: path},
		{ID: "ST-001", Title: "Command parsing", Status: item.StatusDraft, Dependencies: []string{"TK-001", "TK-003"}},
		{ID: "ST-002", Title: "Profile parsing", Status: item.StatusDraft, Dependencies: []string{"TK-003", "TK-001"}},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &CompleteCmd{ID: "TK-003"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result completeJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ID != "TK-003" {
		t.Errorf("ID = %q, want TK-003", result.ID)
	}
	// Both ST-001 and ST-002 should be unblocked (all their deps are now done)
	if len(result.Unblocked) != 2 {
		t.Errorf("Unblocked = %v, want 2 items", result.Unblocked)
	}
}

func TestCompleteCmd_StillBlocked_Text(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "tk-002.md", `---
id: TK-002
title: "Abstraction"
type: Task
priority: High
status: draft
dependencies: [TK-001]
---
`)
	items := []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-002", Title: "Abstraction", Status: item.StatusDraft, Dependencies: []string{"TK-001"}, FilePath: path},
		// ST-002 depends on TK-002 and TK-003 — completing TK-002 leaves it still blocked by TK-003
		{ID: "TK-003", Title: "Error handling", Status: item.StatusDraft, Dependencies: []string{"TK-001"}},
		{ID: "ST-002", Title: "Profile parsing", Status: item.StatusDraft, Dependencies: []string{"TK-002", "TK-003"}},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "TK-002"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Completed TK-002") {
		t.Errorf("expected 'Completed TK-002', got:\n%s", out)
	}
	if !strings.Contains(out, "Still blocked") {
		t.Errorf("expected 'Still blocked' section, got:\n%s", out)
	}
	if !strings.Contains(out, "ST-002") {
		t.Errorf("expected ST-002 in still-blocked list, got:\n%s", out)
	}
	if !strings.Contains(out, "TK-003") {
		t.Errorf("expected TK-003 in waiting-on, got:\n%s", out)
	}
}

// --- NextCmd JSON ---

func TestNextCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &NextCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result nextJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ID != "TK-002" {
		t.Errorf("ID = %q, want TK-002", result.ID)
	}
	if result.Priority != "High" {
		t.Errorf("Priority = %q, want High", result.Priority)
	}
}

// --- NewRunContext integration ---

func TestNewRunContext_Valid(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "tk-001.md", "---\nid: TK-001\ntitle: \"Task\"\nstatus: draft\n---\n")
	writeItemFile(t, dir, "tk-002.md", "---\nid: TK-002\ntitle: \"Task 2\"\nstatus: draft\ndependencies: [TK-001]\n---\n")

	rc, err := NewRunContext(dir, false, defaultConfig())
	if err != nil {
		t.Fatalf("NewRunContext: %v", err)
	}
	if len(rc.Items) != 2 {
		t.Errorf("Items = %d, want 2", len(rc.Items))
	}
	if rc.Graph == nil {
		t.Error("Graph is nil")
	}
}

func TestNewRunContext_DuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "a.md", "---\nid: TK-001\ntitle: \"First\"\nstatus: draft\n---\n")
	writeItemFile(t, dir, "b.md", "---\nid: TK-001\ntitle: \"Duplicate\"\nstatus: draft\n---\n")

	_, err := NewRunContext(dir, false, defaultConfig())
	if err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected 'duplicate' in error, got: %v", err)
	}
}

func TestNewRunContext_CyclicDeps(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "a.md", "---\nid: TK-001\ntitle: \"A\"\nstatus: draft\ndependencies: [TK-002]\n---\n")
	writeItemFile(t, dir, "b.md", "---\nid: TK-002\ntitle: \"B\"\nstatus: draft\ndependencies: [TK-001]\n---\n")

	_, err := NewRunContext(dir, false, defaultConfig())
	if err == nil {
		t.Fatal("expected error for cyclic dependencies")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected 'cycle' in error, got: %v", err)
	}
}

func TestNewRunContext_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	rc, err := NewRunContext(dir, false, defaultConfig())
	if err != nil {
		t.Fatalf("NewRunContext: %v", err)
	}
	if len(rc.Items) != 0 {
		t.Errorf("Items = %d, want 0", len(rc.Items))
	}
}

// --- Custom statuses ---

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
	g, err := graphpkg.Build(items, isDone)
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

func TestStartCmd_CustomStatuses_AlreadyDone(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Title: "Item A", Status: "shipped"},
	}
	rc, _ := newCustomTestContext(t, items, false)

	cmd := &StartCmd{ID: "A"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for already-done item with custom status")
	}
	if !strings.Contains(err.Error(), "already done") {
		t.Errorf("expected 'already done', got: %v", err)
	}
}

func TestStartCmd_CustomStatuses_AlreadyActive(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Title: "Item A", Status: "review"},
	}
	rc, buf := newCustomTestContext(t, items, false)

	cmd := &StartCmd{ID: "A"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "already in progress") {
		t.Errorf("expected 'already in progress', got:\n%s", buf.String())
	}
}

func TestCompleteCmd_CustomStatuses_AlreadyDone(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Title: "Item A", Status: "shipped"},
	}
	rc, _ := newCustomTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "A"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for already-done item with custom status")
	}
}

func TestStatusCmd_CustomStatuses(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: "shipped", Type: "Task", Points: 3},
		{ID: "B", Status: "review", Type: "Task", Points: 5},
		{ID: "C", Status: "backlog", Type: "Task", Points: 2},
	}
	rc, buf := newCustomTestContext(t, items, true)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result statusJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ByStatus["done"].Count != 1 {
		t.Errorf("done count = %d, want 1", result.ByStatus["done"].Count)
	}
	if result.ByStatus["active"].Count != 1 {
		t.Errorf("active count = %d, want 1", result.ByStatus["active"].Count)
	}
	if result.ByStatus["initial"].Count != 1 {
		t.Errorf("initial count = %d, want 1", result.ByStatus["initial"].Count)
	}
}

func TestNewRunContext_CustomStatuses(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "a.md", "---\nid: A\nstate: backlog\n---\n")

	cfg := &config.Config{
		Statuses: config.StatusGroups{
			Done:    []string{"done", "shipped"},
			Active:  []string{"in-progress", "review"},
			Initial: []string{"backlog"},
		},
		FieldAliases: map[string]string{"status": "state"},
	}

	rc, err := NewRunContext(dir, false, cfg)
	if err != nil {
		t.Fatalf("NewRunContext: %v", err)
	}
	if len(rc.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(rc.Items))
	}
	if rc.Items[0].Status != "backlog" {
		t.Errorf("Status = %q, want backlog", rc.Items[0].Status)
	}
	if rc.StatusField != "state" {
		t.Errorf("StatusField = %q, want state", rc.StatusField)
	}
}

// --- ListCmd ---

func TestListCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &ListCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	// Should have group headers
	if !strings.Contains(out, "READY") {
		t.Errorf("expected READY header, got:\n%s", out)
	}
	if !strings.Contains(out, "BLOCKED") {
		t.Errorf("expected BLOCKED header, got:\n%s", out)
	}
	if !strings.Contains(out, "DONE") {
		t.Errorf("expected DONE header, got:\n%s", out)
	}
	// Ready items: TK-002 and TK-003 (all deps done, not done themselves)
	if !strings.Contains(out, "TK-002") || !strings.Contains(out, "TK-003") {
		t.Errorf("expected ready items TK-002 and TK-003, got:\n%s", out)
	}
	// Blocked items: ST-001 and ST-002
	if !strings.Contains(out, "ST-001") || !strings.Contains(out, "ST-002") {
		t.Errorf("expected blocked items ST-001 and ST-002, got:\n%s", out)
	}
	// Blocked items should show waiting on
	if !strings.Contains(out, "waiting on:") {
		t.Errorf("expected 'waiting on:' for blocked items, got:\n%s", out)
	}
	// Done items: SP-001 and TK-001
	if !strings.Contains(out, "SP-001") || !strings.Contains(out, "TK-001") {
		t.Errorf("expected done items SP-001 and TK-001, got:\n%s", out)
	}
}

func TestListCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &ListCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []listItemJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result) != 6 {
		t.Fatalf("expected 6 items, got %d", len(result))
	}

	// Count by group
	groups := make(map[string]int)
	for _, r := range result {
		groups[r.Group]++
	}
	if groups["ready"] != 2 {
		t.Errorf("ready count = %d, want 2", groups["ready"])
	}
	if groups["blocked"] != 2 {
		t.Errorf("blocked count = %d, want 2", groups["blocked"])
	}
	if groups["done"] != 2 {
		t.Errorf("done count = %d, want 2", groups["done"])
	}
}

func TestListCmd_WithActive(t *testing.T) {
	items := makeTestItems()
	items[3].Status = item.StatusInProgress // TK-003
	rc, buf := newTestContext(t, items, false)

	cmd := &ListCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ACTIVE") {
		t.Errorf("expected ACTIVE header, got:\n%s", out)
	}
}

func TestListCmd_Empty(t *testing.T) {
	rc, buf := newTestContext(t, []*item.Item{}, false)

	cmd := &ListCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items found") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}

// --- pluralizeType ---

func TestPluralizeType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Task", "tasks"},
		{"Story", "stories"},
		{"Spike", "spikes"},
	}

	for _, tt := range tests {
		got := pluralizeType(tt.input)
		if got != tt.want {
			t.Errorf("pluralizeType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
