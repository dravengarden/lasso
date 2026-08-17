package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommandJSON(t *testing.T) {
	oldVersion, oldRevision := Version, BuildRevision
	Version, BuildRevision = "0.2.0", "abc123"
	t.Cleanup(func() { Version, BuildRevision = oldVersion, oldRevision })

	cmd := newVersionCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--format=json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{`"version": "0.2.0"`, `"api_version": 2`, `"build_revision": "abc123"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %s:\n%s", want, got)
		}
	}
}
