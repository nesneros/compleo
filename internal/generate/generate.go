// Package generate handles completion file generation for compleo.
package generate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/nesneros/compleo/internal/zsh"
	"github.com/spf13/cobra"
	"io"
)

// declaredNamePattern matches a spec's top-level name: key, anchored to column 0 so an
// indented one (e.g. a subcommand's) doesn't match.
var declaredNamePattern = regexp.MustCompile(`(?m)^name:`)

// Generate writes a completion file for sp's command tree, with rawSpec embedded as its
// companion spec file, to the completion directory. It writes a zsh success report to out.
func Generate(cmd *cobra.Command, sp spec.Command, rawSpec string, out io.Writer) error {
	dir := cmdutil.CompletionDir()

	// name is what completion gets registered under. EnvGenerateName (from "generate
	// <script>") fills in an undeclared name:, but a declared one that disagrees is an error.
	name := sp.Name
	if requested := os.Getenv(cmdutil.EnvGenerateName); requested != "" {
		if sp.Name != "" && sp.Name != requested {
			return fmt.Errorf("generate: spec declares name: %q, but %q was invoked — these must match", sp.Name, requested)
		}
		name = requested
	}

	// A spec whose own text doesn't declare name: gets it prepended, so the companion file is
	// a complete, standalone spec — sp.Name alone can't tell us this (it may already be set
	// via --name with rawSpec's own text still lacking one).
	specFileText := rawSpec
	if !declaredNamePattern.MatchString(rawSpec) {
		specFileText = "name: '" + strings.ReplaceAll(name, "'", "''") + "'\n" + rawSpec
	}

	// A leading dot keeps this out of EmitActivate's "autoload -Uz dir/*" glob (zsh glob
	// doesn't match dotfiles by default) — it's data for compleo to read, not a completion
	// function for zsh to autoload.
	specFileName := "." + name + ".spec.yaml"
	if err := cmdutil.WriteCompletionFile(dir, specFileName, specFileText); err != nil {
		return err
	}

	snippet, err := zsh.WrappedCompletion(cmd, name, filepath.Join(dir, specFileName))
	if err != nil {
		return err
	}
	fileName := "_" + name
	if err := cmdutil.WriteCompletionFile(dir, fileName, snippet); err != nil {
		return err
	}
	return zsh.EmitGenerated(out, name)
}

// GenerateFor runs script (a bare name resolved via $PATH, or a path) as a subprocess with
// EnvGenerateName set, writing its output to out.
func GenerateFor(script string, out io.Writer) error {
	path, err := exec.LookPath(script)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if !spec.IsCompleoScript(path) {
		return fmt.Errorf("generate: %s does not look like a compleo-wrapped script", script)
	}

	// filepath.Base(script), not script itself: script may be a path (exec.LookPath resolves
	// one directly, without even consulting $PATH), but EnvGenerateName must be a bare
	// name to match what the script's own "${0:t}" computes from its argv[0].
	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(), cmdutil.EnvGenerateName+"="+filepath.Base(script))
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
