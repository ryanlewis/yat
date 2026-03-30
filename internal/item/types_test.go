package item

import "testing"

func TestStatus_Valid(t *testing.T) {
	tests := []struct {
		status Status
		valid  bool
	}{
		{StatusTodo, true},
		{StatusInProgress, true},
		{StatusDone, true},
		{"", true}, // empty is valid (normalized to todo)
		{"Todo", false},
		{"TODO", false},
		{"draft", false},
		{"In-Progress", false},
		{"in_progress", false},
		{"Done", false},
		{"DONE", false},
		{"banana", false},
		{"todo ", false},       // trailing space
		{" todo", false},       // leading space
		{"in progress", false}, // space instead of hyphen
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := tt.status.Valid()
			if got != tt.valid {
				t.Errorf("Status(%q).Valid() = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestPriority_Valid(t *testing.T) {
	tests := []struct {
		priority Priority
		valid    bool
	}{
		{PriorityCritical, true},
		{PriorityHigh, true},
		{PriorityMedium, true},
		{PriorityLow, true},
		{"", true}, // empty sorts last
		{"critical", false},
		{"CRITICAL", false},
		{"high", false},
		{"HIGH", false},
		{"medium", false},
		{"low", false},
		{"Urgent", false},
		{"P0", false},
		{"Critical ", false}, // trailing space
	}

	for _, tt := range tests {
		t.Run(string(tt.priority), func(t *testing.T) {
			got := tt.priority.Valid()
			if got != tt.valid {
				t.Errorf("Priority(%q).Valid() = %v, want %v", tt.priority, got, tt.valid)
			}
		})
	}
}

func TestPriorityRank(t *testing.T) {
	tests := []struct {
		priority Priority
		rank     int
	}{
		{PriorityCritical, 0},
		{PriorityHigh, 1},
		{PriorityMedium, 2},
		{PriorityLow, 3},
		{"", 4},         // unknown
		{"Unknown", 4},  // unknown
		{"critical", 4}, // wrong case = unknown
	}

	for _, tt := range tests {
		t.Run(string(tt.priority), func(t *testing.T) {
			got := PriorityRank(tt.priority)
			if got != tt.rank {
				t.Errorf("PriorityRank(%q) = %d, want %d", tt.priority, got, tt.rank)
			}
		})
	}
}

func TestPriorityRank_Ordering(t *testing.T) {
	// Verify that sorting by rank gives the expected order
	priorities := []Priority{PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical}
	for i := 1; i < len(priorities); i++ {
		if PriorityRank(priorities[i]) >= PriorityRank(priorities[i-1]) {
			t.Errorf("PriorityRank(%s) >= PriorityRank(%s): %d >= %d",
				priorities[i], priorities[i-1],
				PriorityRank(priorities[i]), PriorityRank(priorities[i-1]))
		}
	}
}
