// Package item handles parsing and mutation of YAML-frontmattered markdown item files.
package item

// Status represents the current state of an item.
type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in-progress"
	StatusDone       Status = "done"
)

// Valid reports whether s is a recognized status value.
// An empty status is treated as todo.
func (s Status) Valid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone, "":
		return true
	default:
		return false
	}
}

// StatusSet is a set of valid status values. It allows the item package to
// validate statuses against a dynamic set without importing config.
type StatusSet map[Status]struct{}

// NewStatusSet builds a StatusSet from a slice of status strings.
func NewStatusSet(statuses []string) StatusSet {
	ss := make(StatusSet, len(statuses))
	for _, s := range statuses {
		ss[Status(s)] = struct{}{}
	}
	return ss
}

// Contains reports whether the set contains s. An empty string is always accepted.
func (ss StatusSet) Contains(s Status) bool {
	if s == "" {
		return true
	}
	_, ok := ss[s]
	return ok
}

// Priority represents the importance of an item.
type Priority string

const (
	PriorityCritical Priority = "Critical"
	PriorityHigh     Priority = "High"
	PriorityMedium   Priority = "Medium"
	PriorityLow      Priority = "Low"
)

const (
	priorityRankLow     = 3
	priorityRankUnknown = 4
)

// PriorityRank returns a numeric rank for sorting (lower = higher priority).
func PriorityRank(p Priority) int {
	switch p {
	case PriorityCritical:
		return 0
	case PriorityHigh:
		return 1
	case PriorityMedium:
		return 2
	case PriorityLow:
		return priorityRankLow
	default:
		return priorityRankUnknown
	}
}

// Valid reports whether p is a recognized priority value.
// An empty priority sorts last.
func (p Priority) Valid() bool {
	switch p {
	case PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow, "":
		return true
	default:
		return false
	}
}

// Item represents a single tracked item parsed from a markdown file.
type Item struct {
	ID           string   `yaml:"id"           json:"id"`
	Title        string   `yaml:"title"        json:"title"`
	Type         string   `yaml:"type"         json:"type"`
	Priority     Priority `yaml:"priority"     json:"priority"`
	Points       int      `yaml:"points"       json:"points"`
	Dependencies []string `yaml:"dependencies" json:"dependencies"`
	Status       Status   `yaml:"status"       json:"status"`
	Phase        string   `yaml:"phase"        json:"phase"`

	FilePath string `yaml:"-" json:"file_path"`
	Body     string `yaml:"-" json:"-"`
}
