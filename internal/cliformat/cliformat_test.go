package cliformat

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{"", FormatTable, false},
		{"table", FormatTable, false},
		{"TABLE", FormatTable, false},
		{"json", FormatJSON, false},
		{"JSON", FormatJSON, false},
		{"yaml", FormatYAML, false},
		{"yml", FormatYAML, false},
		{"xml", FormatTable, true},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("Parse(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("Parse(%q): got %v want %v", c.in, got, c.want)
		}
	}
}

func TestResolve(t *testing.T) {
	f, err := Resolve("")
	if err != nil || f != FormatTable {
		t.Errorf("Resolve(\"\"): got (%v, %v), want (table, nil)", f, err)
	}
}

func TestStringAndIsStructured(t *testing.T) {
	cases := []struct {
		f          Format
		wantStr    string
		structured bool
	}{
		{FormatTable, "table", false},
		{FormatJSON, "json", true},
		{FormatYAML, "yaml", true},
	}
	for _, c := range cases {
		if got := c.f.String(); got != c.wantStr {
			t.Errorf("Format(%d).String(): got %q want %q", c.f, got, c.wantStr)
		}
		if got := c.f.IsStructured(); got != c.structured {
			t.Errorf("Format(%d).IsStructured(): got %v want %v", c.f, got, c.structured)
		}
	}
}

func TestMarshal(t *testing.T) {
	type entry struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	v := []entry{{"Ada", 30}}

	jsonOut, err := Marshal(FormatJSON, v)
	if err != nil {
		t.Fatalf("Marshal json: %v", err)
	}
	if !strings.Contains(string(jsonOut), `"name": "Ada"`) {
		t.Errorf("json: %q", jsonOut)
	}

	yamlOut, err := Marshal(FormatYAML, v)
	if err != nil {
		t.Fatalf("Marshal yaml: %v", err)
	}
	if !strings.Contains(string(yamlOut), "name: Ada") {
		t.Errorf("yaml: %q", yamlOut)
	}

	if _, err := Marshal(FormatTable, v); err == nil {
		t.Error("Marshal(table, _): expected error")
	}
}

func TestPrintAddsTrailingNewline(t *testing.T) {
	var buf bytes.Buffer
	if err := Print(&buf, FormatJSON, map[string]int{"a": 1}); err != nil {
		t.Fatalf("Print: %v", err)
	}
	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Print: missing trailing newline; got %q", got)
	}
}

func TestBindWiresFlagAndPreRun(t *testing.T) {
	var f Format
	cmd := &cobra.Command{
		Use:  "x",
		RunE: func(_ *cobra.Command, _ []string) error { return nil },
	}
	Bind(cmd, &f)

	cmd.SetArgs([]string{"--format=yaml"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if f != FormatYAML {
		t.Errorf("Bind: got %v want yaml", f)
	}
}

func TestBindRejectsRemovedJSONAlias(t *testing.T) {
	var f Format
	cmd := &cobra.Command{
		Use:  "x",
		RunE: func(_ *cobra.Command, _ []string) error { return nil },
	}
	Bind(cmd, &f)

	cmd.SetArgs([]string{"--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("Execute error = %v, want unknown flag", err)
	}
}

func TestBindRejectsBadFormat(t *testing.T) {
	var f Format
	cmd := &cobra.Command{
		Use:           "x",
		RunE:          func(_ *cobra.Command, _ []string) error { return nil },
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	Bind(cmd, &f)
	cmd.SetArgs([]string{"--format=xml"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error from --format=xml")
	}
}

func TestBindPreservesExistingPreRunE(t *testing.T) {
	var (
		f         Format
		preCalled bool
		runCalled bool
	)
	cmd := &cobra.Command{
		Use:     "x",
		PreRunE: func(_ *cobra.Command, _ []string) error { preCalled = true; return nil },
		RunE:    func(_ *cobra.Command, _ []string) error { runCalled = true; return nil },
	}
	Bind(cmd, &f)

	cmd.SetArgs([]string{"--format=json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !preCalled || !runCalled {
		t.Errorf("hooks: pre=%v run=%v", preCalled, runCalled)
	}
	if f != FormatJSON {
		t.Errorf("Format: got %v want json", f)
	}
}
