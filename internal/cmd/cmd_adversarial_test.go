package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	graphpkg "github.com/ryanlewis/yat/internal/graph"
	"github.com/ryanlewis/yat/internal/item"
)

// --- ShowCmd edge cases ---

func TestShowCmd_NilDependencies_JSON(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Title: "No deps", Status: item.StatusTodo, Dependencies: nil},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &ShowCmd{ID: "TK-001"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result showJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	// nil dependencies should be serialized as empty array, not null
	if result.Dependencies == nil {
		t.Error("Dependencies should be [] not null in JSON")
	}
}

func TestShowCmd_SpecialCharsInTitle_JSON(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Title: `Title with "quotes" and <html> & stuff`, Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &ShowCmd{ID: "TK-001"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Should produce valid JSON
	var result showJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal failed (bad escaping?): %v\nRaw: %s", err, buf.String())
	}
	if result.Title != `Title with "quotes" and <html> & stuff` {
		t.Errorf("Title = %q", result.Title)
	}
}

func TestShowCmd_UnicodeBody_JSON(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Title: "Unicode", Status: item.StatusTodo, Body: "日本語 🚀 中文\n"},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &ShowCmd{ID: "TK-001"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result showJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.Body != "日本語 🚀 中文\n" {
		t.Errorf("Body = %q", result.Body)
	}
}

func TestShowCmd_NewlinesInBody_JSON(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Title: "Newlines", Status: item.StatusTodo, Body: "Line1\nLine2\nLine3\n"},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &ShowCmd{ID: "TK-001"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result showJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.Body != "Line1\nLine2\nLine3\n" {
		t.Errorf("Body = %q", result.Body)
	}
}

func TestShowCmd_EmptyFields_Text(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Title: "", Type: "", Priority: "", Status: item.StatusTodo, Phase: ""},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &ShowCmd{ID: "TK-001"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TK-001") {
		t.Errorf("should contain ID, got:\n%s", out)
	}
}

func TestShowCmd_LargeBody_Text(t *testing.T) {
	largeBody := strings.Repeat("A very long line of text. ", 500)
	items := []*item.Item{
		{ID: "TK-001", Title: "Large", Status: item.StatusTodo, Body: largeBody},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &ShowCmd{ID: "TK-001"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), largeBody) {
		t.Error("large body should be fully output")
	}
}

// --- ReadyCmd edge cases ---

func TestReadyCmd_AllSamePriority(t *testing.T) {
	items := []*item.Item{
		{ID: "C", Priority: item.PriorityHigh, Status: item.StatusTodo},
		{ID: "A", Priority: item.PriorityHigh, Status: item.StatusTodo},
		{ID: "B", Priority: item.PriorityHigh, Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &ReadyCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []readyJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3, got %d", len(result))
	}
	// Should be sorted by ID when priority is same
	if result[0].ID != "A" || result[1].ID != "B" || result[2].ID != "C" {
		t.Errorf("not sorted by ID: [%s, %s, %s]", result[0].ID, result[1].ID, result[2].ID)
	}
}

func TestReadyCmd_NoPriority(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Priority: "", Status: item.StatusTodo},
		{ID: "B", Priority: "", Status: item.StatusTodo},
	}
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
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestReadyCmd_EmptyJSON(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusDone},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &ReadyCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []readyJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

// --- NextCmd edge cases ---

func TestNextCmd_EmptyJSON(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusDone},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &NextCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result["id"] != nil {
		t.Errorf("expected null id for empty next, got %v", result["id"])
	}
}

// --- StatusCmd edge cases ---

func TestStatusCmd_EmptyProject(t *testing.T) {
	rc, buf := newTestContext(t, []*item.Item{}, false)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "0 items") {
		t.Errorf("expected '0 items', got:\n%s", out)
	}
}

func TestStatusCmd_EmptyProject_JSON(t *testing.T) {
	rc, buf := newTestContext(t, []*item.Item{}, true)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result statusJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.TotalItems != 0 {
		t.Errorf("TotalItems = %d, want 0", result.TotalItems)
	}
}

func TestStatusCmd_AllInProgress(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusInProgress, Type: "Task", Points: 3},
		{ID: "B", Status: item.StatusInProgress, Type: "Task", Points: 5},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result statusJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ByStatus["active"].Count != 2 {
		t.Errorf("ByStatus[active].Count = %d, want 2", result.ByStatus["active"].Count)
	}
	if result.ByStatus["active"].Points != 8 {
		t.Errorf("ByStatus[active].Points = %d, want 8", result.ByStatus["active"].Points)
	}
}

func TestStatusCmd_CustomType(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo, Type: "Bug"},
		{ID: "B", Status: item.StatusTodo, Type: "Bug"},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "bugs") {
		t.Errorf("expected custom type 'bugs' in output, got:\n%s", out)
	}
}

// --- StatusCmd phase output ---

func TestStatusCmd_PhasesInText(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Phase: "1", Status: item.StatusDone, Points: 3},
		{ID: "B", Phase: "1", Status: item.StatusDone, Points: 2},
		{ID: "C", Phase: "2", Status: item.StatusTodo, Points: 5},
		{ID: "D", Phase: "2", Status: item.StatusInProgress, Points: 8},
		{ID: "E", Phase: "3", Status: item.StatusTodo, Points: 1},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Phases:") {
		t.Errorf("expected 'Phases:' section, got:\n%s", out)
	}
	// Active phase (2) should have a marker
	if !strings.Contains(out, "2:") {
		t.Errorf("expected phase 2 in output, got:\n%s", out)
	}
	if !strings.Contains(out, "*") {
		t.Errorf("expected active phase marker '*', got:\n%s", out)
	}
}

func TestStatusCmd_PhasesInJSON(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Phase: "1", Status: item.StatusDone, Points: 3},
		{ID: "B", Phase: "2", Status: item.StatusTodo, Points: 5},
		{ID: "C", Phase: "2", Status: item.StatusInProgress, Points: 8},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result statusJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ActivePhase != "2" {
		t.Errorf("ActivePhase = %q, want 2", result.ActivePhase)
	}
	if len(result.ByPhase) != 2 {
		t.Errorf("ByPhase has %d entries, want 2", len(result.ByPhase))
	}
	if p1, ok := result.ByPhase["1"]; !ok || p1.Count != 1 || p1.Points != 3 {
		t.Errorf("ByPhase[1] = %+v, want {Count:1 Points:3}", result.ByPhase["1"])
	}
	if p2, ok := result.ByPhase["2"]; !ok || p2.Count != 2 || p2.Points != 13 {
		t.Errorf("ByPhase[2] = %+v, want {Count:2 Points:13}", result.ByPhase["2"])
	}
}

func TestStatusCmd_NoPhasesHidesSection(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
		{ID: "B", Status: item.StatusDone},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Phases:") {
		t.Errorf("should not show Phases section when no items have phases, got:\n%s", out)
	}
}

func TestStatusCmd_NoPhasesJSON(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result statusJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ActivePhase != "" {
		t.Errorf("ActivePhase = %q, want empty", result.ActivePhase)
	}
	if len(result.ByPhase) != 0 {
		t.Errorf("ByPhase should be empty, got %d entries", len(result.ByPhase))
	}
}

// --- BlockedCmd edge cases ---

func TestBlockedCmd_EmptyJSON(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &BlockedCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []blockedItemJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

// --- GraphCmd edge cases ---

func TestGraphCmd_SingleItem(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &GraphCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result graphJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if len(result.Layers) != 1 {
		t.Errorf("expected 1 layer, got %d", len(result.Layers))
	}
	if len(result.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(result.Edges))
	}
}

func TestGraphCmd_DeepChain(t *testing.T) {
	const depth = 20
	items := make([]*item.Item, depth)
	for i := range depth {
		it := &item.Item{
			ID:     string(rune('A' + i)),
			Status: item.StatusTodo,
		}
		if i > 0 {
			it.Dependencies = []string{string(rune('A' + i - 1))}
		}
		items[i] = it
	}

	g, err := graphpkg.Build(items)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var buf strings.Builder
	rc := &RunContext{Items: items, Graph: g, JSON: true, Stdout: &buf}

	cmd := &GraphCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result graphJSON
	if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if len(result.Layers) != depth {
		t.Errorf("expected %d layers, got %d", depth, len(result.Layers))
	}
	if len(result.Edges) != depth-1 {
		t.Errorf("expected %d edges, got %d", depth-1, len(result.Edges))
	}
}

// --- CompleteCmd edge cases ---

func TestCompleteCmd_NoUnblocked(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "a.md", "---\nid: A\ntitle: \"Solo\"\nstatus: todo\n---\n")

	items := []*item.Item{
		{ID: "A", Title: "Solo", Status: item.StatusTodo, FilePath: path},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "A"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Completed A") {
		t.Error("expected 'Completed A'")
	}
	// Should NOT contain "Now unblocked" or "Still blocked"
	if strings.Contains(out, "unblocked") || strings.Contains(out, "blocked") {
		t.Errorf("shouldn't mention blocking for item with no dependents, got:\n%s", out)
	}
}

func TestCompleteCmd_InProgressItem(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "a.md", "---\nid: A\ntitle: \"WIP\"\nstatus: in-progress\n---\n")

	items := []*item.Item{
		{ID: "A", Title: "WIP", Status: item.StatusInProgress, FilePath: path},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "A"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Completed A") {
		t.Error("expected 'Completed A'")
	}

	// Verify file updated
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "status: done") {
		t.Error("file not updated to done")
	}
}

func TestCompleteCmd_JSON_NoUnblocked(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "a.md", "---\nid: A\ntitle: \"Solo\"\nstatus: todo\n---\n")

	items := []*item.Item{
		{ID: "A", Title: "Solo", Status: item.StatusTodo, FilePath: path},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &CompleteCmd{ID: "A"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result completeJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if len(result.Unblocked) != 0 {
		t.Errorf("Unblocked should be empty, got %d", len(result.Unblocked))
	}
	if len(result.StillBlocked) != 0 {
		t.Errorf("StillBlocked should be empty, got %d", len(result.StillBlocked))
	}
}

// --- StartCmd edge cases ---

func TestStartCmd_ReadyItem_JSON(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "a.md", "---\nid: A\ntitle: \"Ready\"\nstatus: todo\n---\nBody text.\n")

	items := []*item.Item{
		{ID: "A", Title: "Ready", Status: item.StatusTodo, FilePath: path, Body: "Body text.\n"},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &StartCmd{ID: "A"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result startJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ID != "A" {
		t.Errorf("ID = %q, want A", result.ID)
	}
	if result.Title != "Ready" {
		t.Errorf("Title = %q, want Ready", result.Title)
	}
}

// --- NewRunContext integration edge cases ---

func TestNewRunContext_InvalidStatus(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "bad.md", "---\nid: BAD\nstatus: invalid\n---\n")

	_, err := NewRunContext(dir, false, defaultConfig())
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestNewRunContext_UnknownDependency(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "a.md", "---\nid: A\nstatus: todo\ndependencies: [GHOST]\n---\n")

	_, err := NewRunContext(dir, false, defaultConfig())
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
	if !strings.Contains(err.Error(), "GHOST") {
		t.Errorf("expected 'GHOST' in error, got: %v", err)
	}
}

func TestNewRunContext_MixedValid(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "a.md", "---\nid: A\nstatus: todo\n---\n")
	writeItemFile(t, dir, "b.md", "---\nid: B\nstatus: in-progress\ndependencies: [A]\n---\n")
	writeItemFile(t, dir, "c.md", "---\nid: C\nstatus: done\ndependencies: [A]\n---\n")
	// Non-item markdown
	writeItemFile(t, dir, "readme.md", "# Readme\nNot an item.\n")

	rc, err := NewRunContext(dir, false, defaultConfig())
	if err != nil {
		t.Fatalf("NewRunContext: %v", err)
	}
	if len(rc.Items) != 3 {
		t.Errorf("Items = %d, want 3", len(rc.Items))
	}
}

func TestNewRunContext_SubdirWithMixedContent(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "phase1")
	os.MkdirAll(sub, 0o755)

	writeItemFile(t, dir, "root.md", "---\nid: ROOT\nstatus: todo\n---\n")
	writeItemFile(t, sub, "child.md", "---\nid: CHILD\nstatus: todo\ndependencies: [ROOT]\n---\n")
	writeItemFile(t, sub, "notes.txt", "Not a markdown file")

	rc, err := NewRunContext(dir, false, defaultConfig())
	if err != nil {
		t.Fatalf("NewRunContext: %v", err)
	}
	if len(rc.Items) != 2 {
		t.Errorf("Items = %d, want 2", len(rc.Items))
	}
}

// --- pluralizeType edge cases ---

func TestPluralizeType_EdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Bug", "bugs"},
		{"", "s"},
		{"Y", "ys"},
		{"Deploy", "deploys"},
		{"Entry", "entries"},
	}

	for _, tt := range tests {
		got := pluralizeType(tt.input)
		if got != tt.want {
			t.Errorf("pluralizeType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
