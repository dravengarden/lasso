package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dravengarden/lasso/internal/config"
)

func TestProjectUpdateCheckUsesReadOnlyModeWithDefaultFastForward(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "project-defs", "registry.toml"), "version = 1\n")
	writeTestFile(t, filepath.Join(root, "project-defs", "local", "project.toml"), "kind = \"subdir\"\n")
	t.Setenv(config.RootEnv, root)

	cmd := projectUpdateCmd()
	cmd.SetArgs([]string{"--project=local", "--check"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("project update --check: %v", err)
	}
}

func TestProjectUpdateRejectsAmbiguousScope(t *testing.T) {
	cmd := projectUpdateCmd()
	cmd.SetArgs([]string{"--project=local", "--all-projects"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("project update scope error = %v", err)
	}
}
