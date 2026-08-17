// Package cliformat is the shared output-format flag binding used
// by every harness CLI subcommand that emits structured data.
//
// Convention (see AGENTS.md "CLI conventions"):
//   - The default human view is FormatTable. Subcommands provide
//     this via internal/ui's table rendering directly; this package
//     does not own that path.
//   - --format=json|yaml switches to a machine-readable form.
//     stdout carries ONLY the structured payload; logs and progress
//     belong on stderr (via internal/ui).
//
// Why the unified flag (not per-format bool toggles): the harness
// CLI is consumed primarily by AI agents whose context windows
// benefit from structured output. A single --format keeps the contract small
// and lets scripts use standard decoders.
package cliformat

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Format identifies one of the supported output sinks. Zero value
// is FormatTable so subcommands that just need a default-on-flag
// don't have to special-case unset.
type Format int

const (
	// FormatTable is the human-oriented default. Subcommands render
	// tables via internal/ui — this package does not handle the
	// table path; callers branch on `f.IsStructured()`.
	FormatTable Format = iota
	// FormatJSON emits indented JSON to stdout.
	FormatJSON
	// FormatYAML emits YAML for human/agent diff workflows.
	FormatYAML
)

// String returns the flag-value-form name (matches the form a user
// passes via --format=<name>).
func (f Format) String() string {
	switch f {
	case FormatTable:
		return "table"
	case FormatJSON:
		return "json"
	case FormatYAML:
		return "yaml"
	}
	return "unknown"
}

// IsStructured reports whether the format is machine-readable
// (anything other than the human-table default).
func (f Format) IsStructured() bool {
	return f != FormatTable
}

// ErrUnknownFormat is returned by parsing helpers when an
// unrecognized name is supplied.
var ErrUnknownFormat = errors.New("cliformat: unknown format (want one of: table, json, yaml)")

// Parse maps a flag string to a Format. Empty input yields
// FormatTable (the spec'd default).
func Parse(name string) (Format, error) {
	switch strings.ToLower(name) {
	case "", "table":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "yaml", "yml":
		return FormatYAML, nil
	}
	return FormatTable, fmt.Errorf("%w: %q", ErrUnknownFormat, name)
}

// Bind attaches the standard `--format` flag to cmd, writing the resolved
// Format into *out before each RunE call. Caller still owns RunE — this helper
// just wires the flag and the parse hook.
//
// Callers that already have their own PreRunE chain can use Resolve
// directly instead.
func Bind(cmd *cobra.Command, out *Format) {
	if cmd == nil {
		panic("cliformat.Bind: nil cobra.Command")
	}
	if out == nil {
		panic("cliformat.Bind: nil *Format")
	}

	var formatStr string

	cmd.Flags().StringVar(&formatStr, "format", "",
		"output format: table (default human view), json, yaml")
	pre := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		f, err := Resolve(formatStr)
		if err != nil {
			return err
		}
		*out = f
		if pre != nil {
			return pre(c, args)
		}
		return nil
	}
}

// Resolve parses the raw --format value.
func Resolve(formatStr string) (Format, error) { return Parse(formatStr) }

// Marshal renders v in the chosen format. FormatTable is rejected
// — table rendering is the caller's responsibility (it usually
// involves tabwriter + per-row knowledge that this package can't
// see). Use IsStructured() to gate the call.
func Marshal(f Format, v any) ([]byte, error) {
	switch f {
	case FormatJSON:
		out, err := jsonMarshal(v)
		if err != nil {
			return nil, fmt.Errorf("cliformat: marshal json: %w", err)
		}
		return out, nil
	case FormatYAML:
		out, err := yaml.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("cliformat: marshal yaml: %w", err)
		}
		return out, nil
	case FormatTable:
		return nil, errors.New("cliformat.Marshal: table format must be rendered by the caller via internal/ui")
	}
	return nil, fmt.Errorf("%w: %d", ErrUnknownFormat, f)
}

// Print marshals v in the chosen format and writes it to w with a
// single trailing newline (matching the existing fmt.Println(...)
// convention in the migrated commands).
func Print(w io.Writer, f Format, v any) error {
	out, err := Marshal(f, v)
	if err != nil {
		return err
	}
	if _, err := w.Write(out); err != nil {
		return fmt.Errorf("cliformat: write: %w", err)
	}
	if len(out) == 0 || out[len(out)-1] != '\n' {
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return fmt.Errorf("cliformat: write newline: %w", err)
		}
	}
	return nil
}
