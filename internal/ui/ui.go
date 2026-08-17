// Package ui holds small presentation helpers: tables and styled output.
package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// Table returns a pre-styled table writer that prints to stdout.
func Table(title string) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	if title != "" {
		t.SetTitle(title)
	}

	t.SetStyle(table.StyleRounded)
	t.Style().Format.Header = text.FormatDefault
	t.Style().Title.Align = text.AlignLeft

	return t
}

// Dimf writes a dim/grey message to stderr (echoed shell commands, hints).
func Dimf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "\033[2m"+format+"\033[0m\n", args...)
}

// OKf prints a success line with a green checkmark.
func OKf(format string, args ...any) {
	//nolint:errcheck // stdout write; if it fails the CLI is already broken
	_, _ = fmt.Fprintf(os.Stdout, "\033[32m✓\033[0m "+format+"\n", args...)
}

// Warnf prints a yellow warning line.
func Warnf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "\033[33m!\033[0m "+format+"\n", args...)
}

// Errf prints a red error line to stderr (does not exit).
func Errf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "\033[31merror:\033[0m "+format+"\n", args...)
}

// Writer returns the underlying error writer (for places that need an
// io.Writer, e.g. cobra's SetErr).
func Writer() io.Writer { return os.Stderr }
