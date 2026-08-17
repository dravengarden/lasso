// Package workitems stores the small amount of durable coordination state that
// does not belong in a Codex task, goal, or plan.
package workitems

const (
	// CurrentVersion is the metadata version written to new item.yaml files.
	// Archived versions remain preserved as historical records; they are never
	// rewritten just because the active schema evolves.
	CurrentVersion = 2

	DirName      = "work-items"
	ActiveDir    = "active"
	MetadataFile = "item.yaml"
)

// Item is intentionally smaller than an execution task. Codex owns transient
// state such as the current goal, plan, progress, reviewers, and conversation.
// This record exists only when a blocker, decision, migration, runbook, or
// handoff needs a repository-auditable home outside the current Codex task.
type Item struct {
	Version       int      `json:"version" yaml:"version"`
	ID            string   `json:"id" yaml:"id"`
	Title         string   `json:"title" yaml:"title"`
	Projects      []string `json:"projects,omitempty" yaml:"projects,omitempty"`
	Recipe        string   `json:"recipe,omitempty" yaml:"recipe,omitempty"`
	BlockedReason string   `json:"blocked_reason,omitempty" yaml:"blocked_reason,omitempty"`
	Created       string   `json:"created" yaml:"created"`
}

// Issue is one validation failure associated with an active work item.
type Issue struct {
	ItemID string
	Err    error
}

func (i Issue) Error() string {
	if i.ItemID == "" {
		return i.Err.Error()
	}

	return i.ItemID + ": " + i.Err.Error()
}
