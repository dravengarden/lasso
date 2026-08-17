package cli

import (
	"testing"

	"github.com/dravengarden/lasso/internal/config"
)

func TestFilterSetupNamesRejectsUnknownProject(t *testing.T) {
	if _, err := filterSetupNames([]string{"alpha", "beta"}, []string{"missing"}); err == nil {
		t.Fatal("unknown --only project was silently ignored")
	}
}

func TestSetupOneSkipsSubdirProject(t *testing.T) {
	root := &config.Root{Projects: map[string]config.Project{
		"local": {Kind: config.KindSubdir},
	}}

	got, failed := setupOne(t.Context(), root, t.TempDir(), "local")
	if failed {
		t.Fatalf("subdir setup failed: %+v", got)
	}
	if got.state != "not-applicable" {
		t.Fatalf("state = %q, want not-applicable", got.state)
	}
}
