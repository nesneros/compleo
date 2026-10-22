// Package generate handles completion file generation for compleo.
package generate

import (
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/spf13/cobra"
)

// declaredNamePattern matches a spec's top-level name: key, anchored to column 0 so an
// indented one (e.g. a subcommand's) doesn't match.
var declaredNamePattern = regexp.MustCompile(`(?m)^name:`)

// Generate writes a completion file for sp's command tree, with rawSpec embedded as its
// companion spec file, to the completion directory. The snippet is for the interactive
// shell; the success report (via em, already bound to the script's own shell's stream) is
// eval'd by the script's own shell. out/errOut are only used to construct the completion-
// target emitter (shells.GetCompletionEmitter) for its ActivationHint/WrappedCompletion.
func Generate(cmd *cobra.Command, sp spec.Command, rawSpec, generateName string, em shells.EvalEmitter, out, errOut io.Writer) error {
	dir := cmdutil.CompletionDir()

	// name is what completion gets registered under. generateName (from "generate
	// <script>") fills in an undeclared name:, but a declared one that disagrees is an error.
	name := sp.Name
	if generateName != "" {
		if sp.Name != "" && sp.Name != generateName {
			return fmt.Errorf("generate: spec declares name: %q, but %q was invoked — these must match", sp.Name, generateName)
		}
		name = generateName
	}

	// The wrapping script is only known when "compleo generate <script>" passed its path.
	// A hand-run _COMPLEO_GENERATE_NAME trigger has no script to read a shebang from.
	scriptShell, err := wrappingScriptShell()
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if sp.ScriptShell != "" && scriptShell != "" && sp.ScriptShell != scriptShell {
		return fmt.Errorf("generate: spec declares scriptShell: %q, but the script's shebang names %q", sp.ScriptShell, scriptShell)
	}

	// A spec whose own text doesn't declare name: gets it prepended, so the companion file is
	// a complete, standalone spec — sp.Name alone can't tell us this (it may already be set
	// via --name with rawSpec's own text still lacking one). scriptShell is the same kind of
	// fill-in, from the shebang rather than from --name.
	specFileText := rawSpec
	if scriptShell != "" && sp.ScriptShell == "" {
		specFileText = "scriptShell: '" + strings.ReplaceAll(scriptShell, "'", "''") + "'\n" + specFileText
	}
	if !declaredNamePattern.MatchString(rawSpec) {
		specFileText = "name: '" + strings.ReplaceAll(name, "'", "''") + "'\n" + specFileText
	}

	// A leading dot keeps this out of EmitActivate's "autoload -Uz dir/*" glob (zsh glob
	// doesn't match dotfiles by default) — it's data for compleo to read, not a completion
	// function for zsh to autoload.
	specFileName := "." + name + ".spec.yaml"
	if err := cmdutil.WriteCompletionFile(dir, specFileName, specFileText); err != nil {
		return err
	}

	completionEm := shells.GetCompletionEmitter(out, errOut)
	snippet, err := completionEm.WrappedCompletion(cmd, name, filepath.Join(dir, specFileName))
	if err != nil {
		return err
	}
	fileName := "_" + name
	if err := cmdutil.WriteCompletionFile(dir, fileName, snippet); err != nil {
		return err
	}
	self := "compleo"
	if exe, err := os.Executable(); err == nil {
		self = filepath.Base(exe)
	}
	// The hint text is completion-target-shell-specific (may differ from em's own shell);
	// only the print/exit wrapper around it is in em's.
	// A missing $bin.* target fails at Tab time as a completion message, not a shell error.
	return em.EmitGenerated(completionEm.ActivationHint(name), MissingMacroBins(sp, self))
}

// wrappingScriptShell reads the shebang of the script named by EnvScriptFile.
// An empty path means no script was passed, and the result is empty.
func wrappingScriptShell() (string, error) {
	path := os.Getenv(cmdutil.EnvScriptFile)
	if path == "" {
		return "", nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck
	return shells.ShebangShell(f)
}

// MissingMacroBins returns binaries ActionMacro would exec for sp that are not on PATH, for
// self (the running binary's basename, which dispatches its own $self.* macros in-process
// rather than exec'ing them — see macroBin).
func MissingMacroBins(sp spec.Command, self string) []string {
	var missing []string
	for _, bin := range slices.Sorted(maps.Keys(externalMacros(sp, self))) {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	return missing
}

// externalMacros returns the set of binaries ActionMacro would exec for sp's completion macros.
func externalMacros(sp spec.Command, self string) map[string]struct{} {
	found := make(map[string]struct{})
	for expr := range sp.Completions() {
		found[macroBin(expr, self)] = struct{}{}
	}
	delete(found, "")
	return found
}

// macroBin is the command ActionMacro would exec: the macro name up to its first dot.
// "(" starts arguments ($files([.go]) has a dot that is not a name separator), and " ||| "
// starts a modifier. An empty return means the expression execs nothing. self is the
// running binary's basename: a macro starting with $<self>. is in-process.
func macroBin(expr, self string) string {
	head, _, _ := strings.Cut(expr, " ||| ")
	if !strings.HasPrefix(head, "$") || strings.HasPrefix(head, "$"+self+".") {
		return ""
	}
	name, _, _ := strings.Cut(strings.TrimPrefix(head, "$"), "(")
	// "$_…" is a deprecation message; "$_." is an in-process key. Neither execs a binary.
	if strings.HasPrefix(name, "_") {
		return ""
	}
	bin, _, ok := strings.Cut(name, ".")
	if !ok {
		return ""
	}
	return bin
}

// GenerateFor runs script (a bare name resolved via $PATH, or a path) as a subprocess with
// EnvGenerateName and EnvScriptFile set, writing its output to out. shell is the
// interactive shell the completion targets; empty leaves EnvCompletionShell unset.
func GenerateFor(script, shell string, out io.Writer) error {
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
	env := []string{
		cmdutil.EnvGenerateName + "=" + filepath.Base(script),
		cmdutil.EnvScriptFile + "=" + path,
	}
	if shell != "" {
		env = append(env, cmdutil.EnvCompletionShell+"="+shell)
	}
	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
