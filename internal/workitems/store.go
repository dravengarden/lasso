package workitems

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dravengarden/lasso/internal/config"
)

var idPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

var forbiddenRuntimeArtifacts = map[string]struct{}{
	"plan.md":     {},
	"progress.md": {},
	"status.md":   {},
}

var recipeArtifact = map[string]string{
	"bug":       "investigation.md",
	"design":    "decision.md",
	"feature":   "brief.md",
	"migration": "migration.md",
	"ops":       "runbook.md",
	"spike":     "findings.md",
}

// Store reads and writes work-items/ beneath a Lasso checkout.
type Store struct {
	root string
}

func NewStore(root string) *Store { return &Store{root: root} }

func (s *Store) ActivePath(id string) string {
	return filepath.Join(s.root, DirName, ActiveDir, id)
}

func (s *Store) metadataPath(id string) string {
	return filepath.Join(s.ActivePath(id), MetadataFile)
}

// Create writes metadata only. A skill may add the recipe-specific durable
// artifact after deciding that one is actually useful.
func (s *Store) Create(item Item) (string, error) {
	if item.Version == 0 {
		item.Version = CurrentVersion
	}
	if item.Created == "" {
		item.Created = time.Now().UTC().Format(time.RFC3339)
	}
	if err := validateItem(item, nil); err != nil {
		return "", err
	}

	dir := s.ActivePath(item.ID)
	if err := os.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
		return "", fmt.Errorf("create active work-items directory: %w", err)
	}
	if err := os.Mkdir(dir, 0o750); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("work item %q already exists", item.ID)
		}
		return "", fmt.Errorf("create work item directory: %w", err)
	}

	data, err := yaml.Marshal(item)
	if err != nil {
		_ = os.Remove(dir)
		return "", fmt.Errorf("encode metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, MetadataFile), data, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("write metadata: %w", err)
	}

	return dir, nil
}

// Load reads one active item with strict field checking.
func (s *Store) Load(id string) (*Item, error) {
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("invalid work item id %q", id)
	}

	item, err := loadFile(s.metadataPath(id))
	if err != nil {
		return nil, err
	}
	if item.ID != id {
		return nil, fmt.Errorf("metadata id %q does not match directory %q", item.ID, id)
	}
	if err := validateItem(*item, nil); err != nil {
		return nil, fmt.Errorf("validate %s: %w", s.metadataPath(id), err)
	}

	return item, nil
}

func loadFile(path string) (*Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var item Item
	if err := dec.Decode(&item); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("decode %s: multiple YAML documents are not allowed", path)
		}
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	return &item, nil
}

// ListActive returns every active item sorted by id. Invalid metadata fails
// closed so destructive callers never silently ignore a reference.
func (s *Store) ListActive() ([]*Item, error) {
	dir := filepath.Join(s.root, DirName, ActiveDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read active work items: %w", err)
	}

	items := make([]*Item, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		item, err := s.Load(entry.Name())
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b *Item) int { return strings.Compare(a.ID, b.ID) })

	return items, nil
}

// Validate checks active metadata and registered project references.
func (s *Store) Validate(projects map[string]config.Project) ([]Issue, error) {
	items, err := s.ListActive()
	if err != nil {
		return []Issue{{Err: err}}, nil
	}

	issues := make([]Issue, 0)
	for _, item := range items {
		if err := validateItem(*item, projects); err != nil {
			issues = append(issues, Issue{ItemID: item.ID, Err: err})
		}
		entries, readErr := os.ReadDir(s.ActivePath(item.ID))
		if readErr != nil {
			issues = append(issues, Issue{ItemID: item.ID, Err: fmt.Errorf("read artifacts: %w", readErr)})
			continue
		}
		artifactCount := 0
		for _, entry := range entries {
			name := strings.ToLower(entry.Name())
			if name == MetadataFile {
				continue
			}
			artifactCount++
			if _, forbidden := forbiddenRuntimeArtifacts[name]; forbidden {
				issues = append(issues, Issue{
					ItemID: item.ID,
					Err:    fmt.Errorf("%s duplicates Codex-owned execution state", entry.Name()),
				})
				continue
			}
			if entry.IsDir() || filepath.Ext(name) != ".md" {
				issues = append(issues, Issue{
					ItemID: item.ID,
					Err:    fmt.Errorf("%s is not a single durable Markdown artifact", entry.Name()),
				})
				continue
			}
			if expected := recipeArtifact[item.Recipe]; expected != "" && name != expected {
				issues = append(issues, Issue{
					ItemID: item.ID,
					Err:    fmt.Errorf("recipe %s artifact must be %s, got %s", item.Recipe, expected, entry.Name()),
				})
			}
		}
		if artifactCount > 1 {
			issues = append(issues, Issue{
				ItemID: item.ID,
				Err:    fmt.Errorf("work item has %d artifacts; at most one is allowed", artifactCount),
			})
		}
	}

	slices.SortFunc(issues, func(a, b Issue) int {
		if a.ItemID != b.ItemID {
			return strings.Compare(a.ItemID, b.ItemID)
		}
		return strings.Compare(a.Err.Error(), b.Err.Error())
	})

	return issues, nil
}

func validateItem(item Item, projects map[string]config.Project) error {
	if item.Version != CurrentVersion {
		return fmt.Errorf("version must be %d, got %d", CurrentVersion, item.Version)
	}
	if !idPattern.MatchString(item.ID) {
		return fmt.Errorf("invalid id %q", item.ID)
	}
	if title := strings.TrimSpace(item.Title); title == "" || len([]rune(title)) > 160 {
		return errors.New("title must contain 1-160 characters")
	}
	if _, err := time.Parse(time.RFC3339, item.Created); err != nil {
		return fmt.Errorf("created must be RFC3339: %w", err)
	}
	if item.Recipe != "" && (!idPattern.MatchString(item.Recipe) || len(item.Recipe) > 64) {
		return fmt.Errorf("recipe must be a short open-vocabulary slug, got %q", item.Recipe)
	}
	if item.BlockedReason != "" && strings.TrimSpace(item.BlockedReason) == "" {
		return errors.New("blocked_reason cannot be whitespace")
	}

	seenProjects := make(map[string]bool, len(item.Projects))
	for _, project := range item.Projects {
		if !config.IsProjectName(project) {
			return fmt.Errorf("invalid project id %q", project)
		}
		if seenProjects[project] {
			return fmt.Errorf("duplicate project %q", project)
		}
		seenProjects[project] = true
		if projects != nil {
			if _, ok := projects[project]; !ok {
				return fmt.Errorf("unknown project %q", project)
			}
		}
	}

	return nil
}

// RemoveActive permanently removes one active item. Callers must enforce an
// explicit confirmation before invoking this method.
func (s *Store) RemoveActive(id string) error {
	if _, err := s.Load(id); err != nil {
		return err
	}
	if err := os.RemoveAll(s.ActivePath(id)); err != nil {
		return fmt.Errorf("remove work item: %w", err)
	}

	return nil
}
