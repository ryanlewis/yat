package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/ryanlewis/yat/internal/item"
	"gopkg.in/yaml.v3"
)

// AddCmd creates a new item file.
type AddCmd struct {
	Title    string   `arg:"" help:"Title of the new item."`
	Type     string   `help:"Item type (e.g. Task, Story, Spike). Also picks the id prefix and folder."`
	Priority string   `help:"Priority: Critical, High, Medium or Low."`
	Points   int      `help:"Effort estimate."`
	Dep      []string `help:"ID of an item this one depends on (repeatable)."`
	Phase    string   `help:"Phase label."`
	Status   string   `help:"Status (default: the first initial status)."`
	Body     *string  `help:"Item body. When not given, read from stdin if it is a pipe or a file."`
	ID       string   `name:"id" help:"ID to use instead of the next one for the type." xor:"id"`
	Prefix   string   `help:"ID prefix to number from, e.g. TK- (default: the prefix items of the type use)." xor:"id"`

	stdin io.Reader
}

type addJSON struct {
	ID           string   `json:"id"`
	Path         string   `json:"path"`
	Title        string   `json:"title"`
	Type         string   `json:"type"`
	Priority     string   `json:"priority"`
	Points       int      `json:"points"`
	Dependencies []string `json:"dependencies"`
	Status       string   `json:"status"`
	Phase        string   `json:"phase"`
	Body         string   `json:"body"`
}

const (
	maxSlugLen     = 50
	defaultIDWidth = 3

	// defaultSkippedIDWidth is the most digits a skipped file's name may number
	// an id with when no item has one of the prefix yet.
	defaultSkippedIDWidth = 4
)

// numberedID splits an id into a prefix and a trailing number (TK-007 → TK-, 007).
var numberedID = regexp.MustCompile(`^(.*?)(\d+)$`)

// Run executes the add command.
func (a *AddCmd) Run(rc *RunContext) error {
	if rc.ReadOnly {
		return fmt.Errorf("project is read-only (readonly: true in .yat.yaml)")
	}

	it, err := a.newItem(rc)
	if err != nil {
		return err
	}

	if it.Body, err = a.body(); err != nil {
		return err
	}

	if err := a.write(rc, it); err != nil {
		return err
	}

	if rc.JSON {
		return rc.writeJSON(addJSON{
			ID:           it.ID,
			Path:         it.FilePath,
			Title:        it.Title,
			Type:         it.Type,
			Priority:     string(it.Priority),
			Points:       it.Points,
			Dependencies: it.Dependencies,
			Status:       string(it.Status),
			Phase:        it.Phase,
			Body:         it.Body,
		})
	}

	rc.printf("Added %s at %s\n", it.ID, it.FilePath)

	return nil
}

// write picks it an id and folder and writes its file. It releases the items
// directory's lock, which rc may hold so that two adds never pick the same id,
// before anything is printed.
func (a *AddCmd) write(rc *RunContext, it *item.Item) error {
	defer rc.Unlock()

	var err error
	if it.ID, err = a.resolveID(rc); err != nil {
		return err
	}

	it.FilePath = filepath.Join(targetDir(rc, it.Type), fileName(it.ID, it.Title))

	data, err := rc.renderItem(it)
	if err != nil {
		return err
	}

	if err := item.CreateFile(it.FilePath, data, initFilePermissions); err != nil {
		if errors.Is(err, item.ErrFileExists) {
			return fmt.Errorf("refusing to overwrite %s", it.FilePath)
		}

		return fmt.Errorf("writing %s: %w", it.FilePath, err)
	}

	return nil
}

// newItem validates the flags and returns the item they describe, without an id.
func (a *AddCmd) newItem(rc *RunContext) (*item.Item, error) {
	title := strings.TrimSpace(a.Title)
	if title == "" {
		return nil, fmt.Errorf("title must not be empty")
	}

	priority, err := parsePriority(a.Priority)
	if err != nil {
		return nil, err
	}

	if a.Points < 0 {
		return nil, fmt.Errorf("points must not be negative")
	}

	status := a.Status
	if status == "" {
		status = rc.Statuses.DefaultInitial()
	} else if !slices.Contains(rc.Statuses.AllValid(), status) {
		return nil, fmt.Errorf("invalid status %q: must be one of %s", status,
			strings.Join(rc.Statuses.AllValid(), ", "))
	}

	deps := make([]string, 0, len(a.Dep))

	for _, dep := range a.Dep {
		if _, ok := rc.Graph.Item(dep); !ok {
			return nil, fmt.Errorf("dependency %q not found", dep)
		}

		if !slices.Contains(deps, dep) {
			deps = append(deps, dep)
		}
	}

	return &item.Item{
		Title:        title,
		Type:         existingType(rc.Items, a.Type),
		Priority:     priority,
		Points:       a.Points,
		Dependencies: deps,
		Status:       item.Status(status),
		Phase:        a.Phase,
	}, nil
}

// existingType returns typ in the casing existing items use for it (task →
// Task), the most common casing if they differ, or typ when no item has it.
func existingType(items []*item.Item, typ string) string {
	counts := map[string]int{}

	for _, it := range items {
		if it.Type != "" && strings.EqualFold(it.Type, typ) {
			counts[it.Type]++
		}
	}

	if existing, ok := mostCommon(counts); ok {
		return existing
	}

	return typ
}

// parsePriority returns the priority named by s, ignoring case.
func parsePriority(s string) (item.Priority, error) {
	if s == "" {
		return "", nil
	}

	for _, p := range []item.Priority{item.PriorityCritical, item.PriorityHigh, item.PriorityMedium, item.PriorityLow} {
		if strings.EqualFold(s, string(p)) {
			return p, nil
		}
	}

	return "", fmt.Errorf("invalid priority %q: must be Critical, High, Medium, or Low", s)
}

// resolveID returns --id when given, else the next id for the prefix.
func (a *AddCmd) resolveID(rc *RunContext) (string, error) {
	if a.ID != "" {
		if err := checkIDText("id", a.ID); err != nil {
			return "", err
		}

		if hasID(rc.Items, a.ID) {
			return "", fmt.Errorf("item %q already exists", a.ID)
		}

		if path, ok := skippedWithID(rc.skipped, a.ID); ok {
			return "", fmt.Errorf("item %q may already exist in %s, which was skipped as unreadable; "+
				"make it readable or pass another --id", a.ID, path)
		}

		return a.ID, nil
	}

	prefix := a.Prefix
	if prefix != "" {
		if err := checkIDText("prefix", prefix); err != nil {
			return "", err
		}
	} else {
		var ok bool
		if prefix, ok = inferPrefix(rc.Items, a.Type); !ok {
			kind := "untyped items"
			if a.Type != "" {
				kind = fmt.Sprintf("items of type %q", a.Type)
			}

			return "", fmt.Errorf("cannot pick an id: no %s have a numbered id; "+
				"pass --prefix (e.g. --prefix TK-) or --id", kind)
		}
	}

	taken, err := skippedIDs(rc.skipped, prefix, idWidth(rc.Items, prefix))
	if err != nil {
		return "", err
	}

	id := nextID(append(slices.Clip(rc.Items), taken...), prefix)
	if hasID(rc.Items, id) {
		return "", fmt.Errorf("item %q already exists; pass --id", id)
	}

	return id, nil
}

// skippedIDs returns an item for each skipped file whose name starts with an id
// numbered from prefix, no wider than width digits, so nextID counts that number
// as taken. Any other skipped file or folder (TK-20240115-retro.md) may hold the
// next id, so it refuses to pick one.
func skippedIDs(skipped []string, prefix string, width int) ([]*item.Item, error) {
	var (
		taken   []*item.Item
		unknown []string
	)

	for _, path := range skipped {
		if id, ok := skippedID(path, prefix, width); ok {
			taken = append(taken, &item.Item{ID: id})
		} else {
			unknown = append(unknown, path)
		}
	}

	if len(unknown) > 0 {
		return nil, fmt.Errorf("cannot pick an id: skipped as unreadable and may hold the next %s id: %s; "+
			"make them readable or pass --id", prefix, strings.Join(unknown, ", "))
	}

	return taken, nil
}

// skippedID returns the id the name of the skipped file at path starts with
// (TK-007 from TK-007-fix-login.md) when it is numbered from prefix with at
// most width digits.
func skippedID(path, prefix string, width int) (string, bool) {
	if info, err := os.Lstat(path); err != nil || info.IsDir() {
		return "", false
	}

	rest, ok := strings.CutPrefix(filepath.Base(path), prefix)
	if !ok {
		return "", false
	}

	digits := rest[:len(rest)-len(strings.TrimLeft(rest, "0123456789"))]
	if digits == "" || len(digits) > width {
		return "", false
	}

	return prefix + digits, true
}

// skippedWithID returns the first skipped path whose name starts with id
// followed by "-" or ".", as the file may hold the item with that id.
func skippedWithID(skipped []string, id string) (string, bool) {
	for _, path := range skipped {
		name := filepath.Base(path)
		if strings.HasPrefix(name, id+"-") || strings.HasPrefix(name, id+".") {
			return path, true
		}
	}

	return "", false
}

// idWidth returns the most digits an item id numbered from prefix has, or
// defaultSkippedIDWidth when no item has one.
func idWidth(items []*item.Item, prefix string) int {
	width := 0

	for _, it := range items {
		digits, ok := strings.CutPrefix(it.ID, prefix)
		if ok && digits != "" && strings.Trim(digits, "0123456789") == "" {
			width = max(width, len(digits))
		}
	}

	if width == 0 {
		return defaultSkippedIDWidth
	}

	return width
}

// hasID reports whether an item in items has id.
func hasID(items []*item.Item, id string) bool {
	return slices.ContainsFunc(items, func(it *item.Item) bool { return it.ID == id })
}

// checkIDText rejects an id or prefix that cannot be part of a file name.
func checkIDText(what, s string) error {
	if strings.TrimSpace(s) != s || s == "." || s == ".." ||
		strings.ContainsAny(s, `/\`) || strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid %s %q: it becomes part of the file name", what, s)
	}

	return nil
}

// inferPrefix returns the prefix most items of type typ number their ids with.
// Ties go to the alphabetically first prefix.
func inferPrefix(items []*item.Item, typ string) (string, bool) {
	counts := map[string]int{}

	for _, it := range items {
		if !strings.EqualFold(it.Type, typ) {
			continue
		}

		if m := numberedID.FindStringSubmatch(it.ID); m != nil {
			counts[m[1]]++
		}
	}

	return mostCommon(counts)
}

// nextID returns prefix followed by one more than the highest number any item
// uses with it, zero-padded to the widest such number (TK-007 after TK-006).
func nextID(items []*item.Item, prefix string) string {
	highest, width := 0, defaultIDWidth
	seen := false

	for _, it := range items {
		// Not numberedID: a prefix may end in a digit (V2 numbers V2001).
		digits, ok := strings.CutPrefix(it.ID, prefix)
		if !ok || digits == "" || strings.Trim(digits, "0123456789") != "" {
			continue
		}

		n, err := strconv.Atoi(digits)
		if err != nil {
			continue
		}

		if !seen {
			width, seen = 0, true
		}

		highest = max(highest, n)
		width = max(width, len(digits))
	}

	return fmt.Sprintf("%s%0*d", prefix, width, highest+1)
}

// targetDir returns the directory most items of type typ live in, or the items
// directory when there are none. Ties go to the alphabetically first directory.
func targetDir(rc *RunContext, typ string) string {
	counts := map[string]int{}

	for _, it := range rc.Items {
		if it.FilePath != "" && strings.EqualFold(it.Type, typ) {
			counts[filepath.Dir(it.FilePath)]++
		}
	}

	if dir, ok := mostCommon(counts); ok {
		return dir
	}

	return rc.Dir
}

// mostCommon returns the key with the highest count, breaking ties by the
// alphabetically first key.
func mostCommon(counts map[string]int) (string, bool) {
	best, bestN := "", 0

	for k, n := range counts {
		if n > bestN || (n == bestN && k < best) {
			best, bestN = k, n
		}
	}

	return best, bestN > 0
}

// fileName returns "<id>-<slug>.md", or "<id>.md" when the title has no
// letters or digits.
func fileName(id, title string) string {
	if slug := slugify(title); slug != "" {
		return id + "-" + slug + ".md"
	}

	return id + ".md"
}

// slugify lowercases title and joins its runs of letters and digits (in any
// script) with hyphens, cut to maxSlugLen runes.
func slugify(title string) string {
	var slug []rune

	gap := false

	for _, r := range strings.ToLower(title) {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			gap = true

			continue
		}

		if gap && len(slug) > 0 {
			slug = append(slug, '-')
		}

		gap = false

		slug = append(slug, r)
	}

	if len(slug) > maxSlugLen {
		slug = slug[:maxSlugLen]
	}

	return strings.TrimRight(string(slug), "-")
}

// ReadBody sets the body from stdin when --body is not given and stdin is piped
// or redirected from a file; --body, even if empty, means stdin is never read.
// Other stdins (a terminal, a socket an agent harness left open) are not read,
// as they may never reach EOF. Call it before taking the items directory's
// lock, so a slow pipe does not hold up other yat commands.
func (a *AddCmd) ReadBody() error {
	stdin := a.stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	if a.Body != nil || !pipedInput(stdin) {
		return nil
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("reading body from stdin: %w", err)
	}

	body := string(data)
	a.Body = &body

	return nil
}

// body returns the body ReadBody sets, reading it first if it has not been,
// ending in a newline.
func (a *AddCmd) body() (string, error) {
	if err := a.ReadBody(); err != nil {
		return "", err
	}

	var body string
	if a.Body != nil {
		body = *a.Body
	}

	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	return body, nil
}

// pipedInput reports whether r is a pipe or a regular file. Readers that are not
// files (tests) count as piped.
func pipedInput(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return true
	}

	info, err := f.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeNamedPipe != 0 || info.Mode().IsRegular()
}

// fieldKey returns the frontmatter key field is written under, honoring field_aliases.
func (rc *RunContext) fieldKey(field string) string {
	if alias := rc.fieldAliases[field]; alias != "" {
		return alias
	}

	return field
}

// renderItem returns the file contents for a new item: frontmatter with the
// fields that are set (id, title and status always), then the body. It checks
// the result reads back as the same item under the project's field_aliases.
func (rc *RunContext) renderItem(it *item.Item) ([]byte, error) {
	fields := []struct {
		name  string
		value any
		set   bool
	}{
		{"id", it.ID, true},
		{"title", it.Title, true},
		{"type", it.Type, it.Type != ""},
		{"priority", string(it.Priority), it.Priority != ""},
		{"points", it.Points, it.Points != 0},
		{"dependencies", it.Dependencies, len(it.Dependencies) > 0},
		{statusKey, string(it.Status), true},
		{"phase", it.Phase, it.Phase != ""},
	}

	mapping := &yaml.Node{Kind: yaml.MappingNode}
	keys := map[string]string{}

	for _, f := range fields {
		if !f.set {
			continue
		}

		key := rc.fieldKey(f.name)
		if prev, ok := keys[key]; ok {
			return nil, fmt.Errorf("cannot write %s and %s: field_aliases map both to %q", prev, f.name, key)
		}

		keys[key] = f.name

		var value yaml.Node
		if err := value.Encode(f.value); err != nil {
			return nil, err
		}

		if value.Kind == yaml.SequenceNode {
			value.Style = yaml.FlowStyle
		}

		mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, &value)
	}

	var fm bytes.Buffer

	enc := yaml.NewEncoder(&fm)
	enc.SetIndent(2)

	if err := enc.Encode(mapping); err != nil {
		return nil, err
	}

	if err := enc.Close(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fm.Bytes())
	buf.WriteString("---\n")

	if it.Body != "" {
		buf.WriteString("\n" + it.Body)
	}

	if err := rc.checkReadsBack(buf.Bytes(), it); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// checkReadsBack parses data as yat would load it and reports an error unless it
// gives back its fields.
func (rc *RunContext) checkReadsBack(data []byte, it *item.Item) error {
	aliases := make(map[string]string, len(rc.fieldAliases))
	for canonical, alias := range rc.fieldAliases {
		aliases[alias] = canonical
	}

	got, err := item.ParseWithOptions(data, it.FilePath, item.ParseOptions{
		Aliases:       aliases,
		ValidStatuses: item.NewStatusSet(rc.Statuses.AllValid()),
	})

	if err != nil || got == nil || got.ID != it.ID || got.Title != it.Title || got.Type != it.Type ||
		got.Priority != it.Priority || got.Points != it.Points || got.Status != it.Status ||
		got.Phase != it.Phase || !slices.Equal(got.Dependencies, it.Dependencies) {
		return fmt.Errorf("cannot write %s: it would not read back as written under this project's field_aliases",
			it.FilePath)
	}

	return nil
}
