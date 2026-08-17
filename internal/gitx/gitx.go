// Package gitx wraps the Git CLI for the small set of repository operations
// that Lasso owns. Native agent runtimes own live task state; the
// worktree lifecycle helpers in internal/worktree use this adapter.
package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run executes a command, streaming stdout/stderr. If quiet is false, the
// command line is echoed first.
//
//nolint:revive // quiet is an intentional control flag for echo-or-not on the shared exec wrapper
func Run(ctx context.Context, cwd string, quiet bool, name string, args ...string) error {
	if !quiet {
		loc := ""
		if cwd != "" {
			loc = " (" + cwd + ")"
		}

		fmt.Fprintf(os.Stderr, "\033[2m$ %s %s%s\033[0m\n", name, strings.Join(args, " "), loc)
	}

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204: name/args come from harness CLI; intentional shell-out wrapper
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}

	return nil
}

// Out runs a command and returns stdout, trimmed. Stderr is captured and
// included in the error message when the command fails.
func Out(ctx context.Context, cwd, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204: name/args come from harness CLI; intentional shell-out wrapper
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout

	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w (stderr: %s)",
			name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}

// RefExists checks whether a ref resolves in the given checkout.
func RefExists(ctx context.Context, repo, ref string) bool {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "rev-parse", "--verify", "--quiet", ref) //nolint:gosec // G204: repo/ref derive from harness CLI args
	return cmd.Run() == nil
}

// CurrentBranch returns the branch name for the worktree at cwd, or "" if
// detached.
func CurrentBranch(ctx context.Context, cwd string) (string, error) {
	return Out(ctx, cwd, "git", "branch", "--show-current")
}

// Toplevel returns the worktree root for the cwd.
func Toplevel(ctx context.Context, cwd string) (string, error) {
	return Out(ctx, cwd, "git", "rev-parse", "--show-toplevel")
}
