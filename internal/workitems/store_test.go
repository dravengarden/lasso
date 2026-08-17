package workitems

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dravengarden/lasso/internal/config"
)

func TestCreateAndLoad(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	item := Item{
		ID:       "cross-project-migration",
		Title:    "Move the shared protocol without coupling checkout state",
		Projects: []string{"cowboy", "liveview"},
		Recipe:   "migration",
		Created:  "2026-07-12T00:00:00Z",
	}

	dir, err := store.Create(item)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if filepath.Base(dir) != item.ID {
		t.Fatalf("Create dir = %q", dir)
	}

	got, err := store.Load(item.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != CurrentVersion || got.Recipe != "migration" || len(got.Projects) != 2 {
		t.Fatalf("Load = %#v", got)
	}

}

func TestValidateProjectReferences(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	for _, item := range []Item{
		{ID: "alpha", Title: "Alpha", Projects: []string{"cowboy"}, Created: "2026-07-12T00:00:00Z"},
		{ID: "beta", Title: "Beta", Projects: []string{"missing"}, Created: "2026-07-12T00:00:00Z"},
	} {
		if _, err := store.Create(item); err != nil {
			t.Fatalf("Create(%s): %v", item.ID, err)
		}
	}

	issues, err := store.Validate(map[string]config.Project{"cowboy": {Kind: config.KindExternal}})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	joined := ""
	for _, issue := range issues {
		joined += issue.Error() + "\n"
	}
	if !strings.Contains(joined, `unknown project "missing"`) {
		t.Fatalf("missing project issue not found:\n%s", joined)
	}
}

func TestValidateRejectsCodexRuntimeArtifacts(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if _, err := store.Create(Item{ID: "alpha", Title: "Alpha", Created: "2026-07-12T00:00:00Z"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, name := range []string{"plan.md", "STATUS.md", "progress.md"} {
		if err := os.WriteFile(filepath.Join(store.ActivePath("alpha"), name), []byte("transient\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}

	issues, err := store.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(issues) != 4 {
		t.Fatalf("issues = %v, want 4", issues)
	}
	for _, issue := range issues {
		if !strings.Contains(issue.Error(), "duplicates Codex-owned execution state") &&
			!strings.Contains(issue.Error(), "at most one is allowed") {
			t.Fatalf("unexpected issue: %v", issue)
		}
	}
}

func TestValidateEnforcesOneRecipeArtifact(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if _, err := store.Create(Item{
		ID: "alpha", Title: "Alpha", Recipe: "design", Created: "2026-07-12T00:00:00Z",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, name := range []string{"README.md", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(store.ActivePath("alpha"), name), []byte("durable\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}

	issues, err := store.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	joined := ""
	for _, issue := range issues {
		joined += issue.Error() + "\n"
	}
	for _, want := range []string{
		"recipe design artifact must be decision.md",
		"notes.txt is not a single durable Markdown artifact",
		"work item has 2 artifacts; at most one is allowed",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in issues:\n%s", want, joined)
		}
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	path := filepath.Join(root, DirName, ActiveDir, "broken", MetadataFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	body := "version: 2\nid: broken\ntitle: Broken\ncreated: 2026-07-12T00:00:00Z\nstatus: planned\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := store.Load("broken"); err == nil || !strings.Contains(err.Error(), "field status not found") {
		t.Fatalf("Load error = %v", err)
	}
}

func TestRemoveActive(t *testing.T) {
	store := NewStore(t.TempDir())
	item := Item{ID: "foundation", Title: "Foundation", Created: "2026-07-12T00:00:00Z"}
	if _, err := store.Create(item); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.RemoveActive("foundation"); err != nil {
		t.Fatalf("RemoveActive: %v", err)
	}
}
