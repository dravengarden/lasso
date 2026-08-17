// Package config loads the minimal project registry used by Lasso.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	ProjectDefsDir = "project-defs"
	ProjectsRoot   = "projects"
	RootEnv        = "LASSO_ROOT"
	projectFile    = "project.toml"
	registryFile   = "registry.toml"
	registryV1     = 1
)

var registryMarker = filepath.Join(ProjectDefsDir, registryFile)

const ProjectNamePattern = `^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`

var projectNameRE = regexp.MustCompile(ProjectNamePattern)

// IsProjectName reports whether name is a safe canonical registry id.
func IsProjectName(name string) bool { return projectNameRE.MatchString(name) }

type registryHeader struct {
	Version int `toml:"version"`
}

// Root is the directory-defined project registry.
type Root struct {
	Projects map[string]Project `json:"projects"`
}

// Project contains only repository identity and checkout defaults. Commands,
// deployment, service health, and application catalogs belong to their owning
// project or host configuration.
type Project struct {
	Kind          string `json:"kind" toml:"kind"`
	Repo          string `json:"repo,omitempty" toml:"repo,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty" toml:"default_branch,omitempty"`
}

const (
	KindExternal = "external"
	KindSubdir   = "subdir"
)

func (p Project) IsExternal() bool { return p.Kind == KindExternal }

// WorkspaceRoot walks upward until it finds project-defs/registry.toml.
func WorkspaceRoot() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(RootEnv)); configured != "" {
		root, err := filepath.Abs(configured)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", RootEnv, err)
		}
		if info, err := os.Stat(filepath.Join(root, registryMarker)); err != nil || info.IsDir() {
			return "", fmt.Errorf("%w at %s from %s", ErrProjectRegistryNotFound, root, RootEnv)
		}
		return root, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, registryMarker)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%w above %s", ErrProjectRegistryNotFound, cwd)
		}
	}
}

func ProjectDefsPath(root string) string { return filepath.Join(root, ProjectDefsDir) }

func ProjectFilePath(root, name string) string {
	return filepath.Join(root, ProjectDefsDir, name, projectFile)
}

func Load() (*Root, string, error) {
	root, err := WorkspaceRoot()
	if err != nil {
		return nil, "", err
	}
	registry, err := LoadFromRoot(root)
	if err != nil {
		return nil, "", err
	}
	return registry, root, nil
}

// LoadFromRoot loads each project.toml independently with unknown-field
// rejection. Directory names are the project ids; no aggregate map is kept.
func LoadFromRoot(root string) (*Root, error) {
	pdDir := ProjectDefsPath(root)
	marker := filepath.Join(pdDir, registryFile)
	headerData, err := os.ReadFile(marker)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrProjectRegistryNotFound, marker)
	}
	var header registryHeader
	if err := decodeStrictTOML(headerData, &header); err != nil {
		return nil, fmt.Errorf("decode %s: %w", marker, err)
	}
	if header.Version != registryV1 {
		return nil, fmt.Errorf("unsupported project registry version %d", header.Version)
	}

	entries, err := os.ReadDir(pdDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", pdDir, err)
	}
	projects := make(map[string]Project)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		if !IsProjectName(name) {
			return nil, fmt.Errorf("invalid project directory %q (must match %s)", name, ProjectNamePattern)
		}
		path := ProjectFilePath(root, name)
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("project directory %q is missing %s", name, projectFile)
		}
		if err != nil {
			return nil, fmt.Errorf("read project %q: %w", name, err)
		}
		var project Project
		if err := decodeStrictTOML(data, &project); err != nil {
			return nil, fmt.Errorf("decode project %q: %w", name, err)
		}
		if err := validateProject(project); err != nil {
			return nil, fmt.Errorf("validate project %q: %w", name, err)
		}
		projects[name] = project
	}

	return &Root{Projects: projects}, nil
}

func decodeStrictTOML(data []byte, target any) error {
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func validateProject(project Project) error {
	switch project.Kind {
	case KindExternal:
		if strings.TrimSpace(project.Repo) == "" {
			return errors.New("external project requires repo")
		}
		if strings.TrimSpace(project.DefaultBranch) == "" {
			return errors.New("external project requires default_branch")
		}
	case KindSubdir:
		if project.Repo != "" || project.DefaultBranch != "" {
			return fmt.Errorf("%s project cannot declare repo or default_branch", project.Kind)
		}
	default:
		return fmt.Errorf("unknown kind %q", project.Kind)
	}
	return nil
}

func LoadProject(root, name string) (Project, error) {
	registry, err := LoadFromRoot(root)
	if err != nil {
		return Project{}, err
	}
	return registry.MustGet(name)
}

func (r *Root) MustGet(name string) (Project, error) {
	project, ok := r.Projects[name]
	if !ok {
		known := r.Names()
		slices.Sort(known)
		return Project{}, fmt.Errorf("%w %q. Known: %v", ErrUnknownProject, name, known)
	}
	return project, nil
}

func (*Root) ProjectDir(root, name string) string {
	return filepath.Join(root, ProjectsRoot, name)
}

// CheckoutDir resolves the stable local checkout for a registered project.
// Codex owns any additional task or permanent worktrees outside this path.
func (r *Root) CheckoutDir(root, name string) (string, error) {
	project, err := r.MustGet(name)
	if err != nil {
		return "", err
	}
	base := r.ProjectDir(root, name)
	if !project.IsExternal() {
		return existingDir(base)
	}
	if !isOrdinaryClone(base) {
		return "", fmt.Errorf("project checkout not found: %s", base)
	}
	return existingDir(base)
}

func isOrdinaryClone(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

func existingDir(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("project checkout not found: %s", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

func (r *Root) Names() []string {
	out := make([]string, 0, len(r.Projects))
	for name := range r.Projects {
		out = append(out, name)
	}
	return out
}
