package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearRootEnv(t *testing.T) {
	t.Helper()
	t.Setenv(RootEnv, "")
	t.Setenv(LegacyRootEnv, "")
}

func writeRegistryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	pd := filepath.Join(root, ProjectDefsDir)
	writeFixture(t, filepath.Join(pd, registryFile), "version = 1\n")
	writeFixture(t, filepath.Join(pd, "alpha", projectFile), "kind = \"external\"\nrepo = \"git@github.com:org/alpha.git\"\ndefault_branch = \"main\"\n")
	writeFixture(t, filepath.Join(pd, "notes", projectFile), "kind = \"subdir\"\n")
	return root
}

func writeFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestLoadTOMLRegistry(t *testing.T) {
	clearRootEnv(t)
	root := writeRegistryFixture(t)
	t.Chdir(filepath.Join(root, ProjectDefsDir, "alpha"))
	registry, gotRoot, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotRoot != root {
		t.Fatalf("root = %q, want %q", gotRoot, root)
	}
	alpha, err := registry.MustGet("alpha")
	if err != nil {
		t.Fatalf("MustGet: %v", err)
	}
	if alpha.Repo != "git@github.com:org/alpha.git" || alpha.DefaultBranch != "main" {
		t.Fatalf("alpha = %#v", alpha)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	root := writeRegistryFixture(t)
	writeFixture(t, ProjectFilePath(root, "alpha"), "kind = \"external\"\nrepo = \"x\"\ndefault_branch = \"main\"\ndeploy = \"hawk\"\n")
	if _, err := LoadFromRoot(root); err == nil || !strings.Contains(err.Error(), "strict mode") {
		t.Fatalf("LoadFromRoot error = %v", err)
	}
}

func TestLoadRejectsProjectDirectoryWithoutMetadata(t *testing.T) {
	root := writeRegistryFixture(t)
	if err := os.MkdirAll(filepath.Join(root, ProjectDefsDir, "orphan"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if _, err := LoadFromRoot(root); err == nil || !strings.Contains(err.Error(), "missing project.toml") {
		t.Fatalf("LoadFromRoot error = %v", err)
	}
}

func TestLoadRejectsUnsafeProjectDirectoryName(t *testing.T) {
	root := writeRegistryFixture(t)
	writeFixture(t, filepath.Join(root, ProjectDefsDir, "Bad_Name", projectFile), "kind = \"subdir\"\n")
	if _, err := LoadFromRoot(root); err == nil || !strings.Contains(err.Error(), "invalid project directory") {
		t.Fatalf("LoadFromRoot error = %v", err)
	}
}

func TestLoadRejectsRetiredProjectKinds(t *testing.T) {
	for name, body := range map[string]string{
		"empty":    "",
		"internal": "kind = \"internal\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := writeRegistryFixture(t)
			writeFixture(t, ProjectFilePath(root, "notes"), body)
			if _, err := LoadFromRoot(root); err == nil || !strings.Contains(err.Error(), "unknown kind") {
				t.Fatalf("LoadFromRoot error = %v, want unknown kind", err)
			}
		})
	}
}

func TestCheckoutDirResolvesStableProjectPath(t *testing.T) {
	root := writeRegistryFixture(t)
	registry, err := LoadFromRoot(root)
	if err != nil {
		t.Fatalf("LoadFromRoot: %v", err)
	}
	checkout := filepath.Join(root, ProjectsRoot, "alpha")
	if err := os.MkdirAll(filepath.Join(checkout, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if got, err := registry.CheckoutDir(root, "alpha"); err != nil || got != checkout {
		t.Fatalf("CheckoutDir = (%q, %v)", got, err)
	}
	if err := os.RemoveAll(checkout); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if _, err := registry.CheckoutDir(root, "alpha"); err == nil {
		t.Fatal("CheckoutDir unexpectedly found missing stable checkout")
	}
}

func TestCheckoutDirRejectsNestedLegacyProjectPath(t *testing.T) {
	root := writeRegistryFixture(t)
	registry, err := LoadFromRoot(root)
	if err != nil {
		t.Fatalf("LoadFromRoot: %v", err)
	}
	legacy := filepath.Join(root, ProjectsRoot, "alpha", "main")
	if err := os.MkdirAll(filepath.Join(legacy, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if got, err := registry.CheckoutDir(root, "alpha"); err == nil || got != "" {
		t.Fatalf("CheckoutDir = (%q, %v), want missing flat checkout error", got, err)
	}
}

func TestMissingRegistry(t *testing.T) {
	clearRootEnv(t)
	root := t.TempDir()
	t.Chdir(root)
	if _, _, err := Load(); err == nil {
		t.Fatal("expected missing registry error")
	}
}

func TestLoadUsesConfiguredRootOutsideHarnessTree(t *testing.T) {
	clearRootEnv(t)
	root := writeRegistryFixture(t)
	t.Setenv(RootEnv, root)
	t.Chdir(t.TempDir())

	_, gotRoot, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotRoot != root {
		t.Fatalf("root = %q, want %q", gotRoot, root)
	}
}

func TestLoadRejectsInvalidConfiguredRoot(t *testing.T) {
	clearRootEnv(t)
	t.Setenv(RootEnv, t.TempDir())
	if _, _, err := Load(); err == nil || !strings.Contains(err.Error(), RootEnv) {
		t.Fatalf("Load error = %v, want configured-root error", err)
	}
}
