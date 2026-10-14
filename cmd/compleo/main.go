package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/nesneros/compleo/internal/zsh"
	"github.com/spf13/cobra"
)

//go:embed spec.yaml
var specYAML []byte

// COMPLEO_MODE values: what "compleo parse" does beyond parsing "$@" — same heredoc
// re-delivers the spec either way, so no mode-aware logic is needed in the script itself.
const modeGenerate = "generate"

// declaredNamePattern matches a spec's own top-level name: key — anchored to column 0, so it
// doesn't match an indented one (a subcommand's, or a --name flag's own description text).
var declaredNamePattern = regexp.MustCompile(`(?m)^name:`)

func main() {
	if err := run(os.Args[1:], isPiped(os.Stdin), os.Stdin, os.Stdout, os.Getenv("COMPLEO_MODE")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run routes on an explicit "parse" token, since --help/--version could otherwise be compleo's or the wrapped script's.
func run(args []string, piped bool, in io.Reader, out io.Writer, mode string) error {
	if args == nil {
		panic("run: args must not be nil (use an empty slice for 'no arguments')")
	}

	// A live Tab press for a wrapped script's own completion: its generated completion file
	// sets COMPLEO_SPEC_FILE (see WrappedCompletion) before calling "_carapace zsh ...", since
	// ownCmd's own _carapace subcommand below only knows how to complete compleo itself.
	if len(args) > 0 && args[0] == "_carapace" {
		if specFile := os.Getenv("COMPLEO_SPEC_FILE"); specFile != "" {
			f, err := os.Open(specFile)
			if err != nil {
				return fmt.Errorf("opening embedded spec file %q: %w", specFile, err)
			}
			defer f.Close() //nolint:errcheck
			return runCarapace(f, args, out)
		}
	}

	ownCmd, ownSpec := compleoCmd(out)

	result, err := parse.Parse(ownCmd, ownSpec, args)
	if err != nil || result.HelpRequested {
		return err
	}

	// _carapace is carapace's own hidden subcommand (wired by carapace.Gen); it already ran above.
	if len(args) > 0 && args[0] == "_carapace" {
		return nil
	}

	switch result.Path[len(result.Path)-1] {
	case "activate":
		dir, err := completionDir()
		if err != nil {
			return err
		}
		snippet, err := zsh.Completion(ownCmd)
		if err != nil {
			return err
		}
		if err := writeCompletionFile(dir, "_compleo", snippet); err != nil {
			return err
		}
		return zsh.EmitActivate(out, dir)
	case "completion":
		snippet, err := zsh.Completion(ownCmd)
		if err != nil {
			return err
		}
		fmt.Fprint(out, snippet)
	case "generate":
		return runGenerateFor(result.Positionals["script"].Values[0], out)
	case "parse":
		// A variadic positional with zero values binds to nil, not an empty slice; Parse panics on nil.
		wrappedArgs := result.Positionals["args"].Values
		if wrappedArgs == nil {
			wrappedArgs = []string{}
		}
		return runParse(wrappedArgs, piped, in, out, result.Flags["name"].String, mode)
	case "up":
		return runUp(piped, in, out)
	case "validate":
		if !piped {
			return errors.New("no spec piped on stdin (compleo expects a spec to validate on stdin)")
		}
		if _, _, err := spec.ParseYAML(in, ""); err != nil {
			return fmt.Errorf("invalid spec: %w", err)
		}
		fmt.Fprintln(out, "ok")
	case "version":
		fmt.Fprintln(out, versionString())
	default:
		return ownCmd.Help()
	}
	return nil
}

// runParse parses args against the piped spec, or (COMPLEO_MODE=generate) writes its
// completion file instead. name is --name's value, passed straight to spec.ParseYAML. Reads
// all of in upfront so the raw bytes are available to pass to runGenerate verbatim.
func runParse(args []string, piped bool, in io.Reader, out io.Writer, name string, mode string) error {
	if !piped {
		return errors.New("no spec piped on stdin (compleo expects its target script's spec on stdin)")
	}
	rawSpec, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("reading spec: %w", err)
	}
	cmd, sp, err := spec.ParseYAML(bytes.NewReader(rawSpec), name)
	if err != nil {
		return fmt.Errorf("invalid spec: %w", err)
	}
	// Silenced: main() already reports the error; avoid cobra printing its own.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if mode == modeGenerate {
		return runGenerate(cmd, sp, string(rawSpec), out)
	}
	return runEmit(cmd, sp, args, out)
}

// runUp reads the piped spec and writes zsh that re-invokes "compleo parse" with it
// embedded (see zsh.EmitUp) — doesn't validate the spec itself, that happens naturally
// once the generated command actually runs.
func runUp(piped bool, in io.Reader, out io.Writer) error {
	if !piped {
		return errors.New("no spec piped on stdin (compleo expects its target script's spec on stdin)")
	}
	rawSpec, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("reading spec: %w", err)
	}
	return zsh.EmitUp(out, string(rawSpec))
}

// runCarapace serves one live Tab-press request: specYAML is the companion file its
// generated completion points at, built into a fresh cobra tree and dispatched into
// directly. _carapace's own Run never returns a Go error (reports failures to stderr itself).
func runCarapace(specYAML io.Reader, args []string, out io.Writer) error {
	cmd, _, err := spec.ParseYAML(specYAML, "")
	if err != nil {
		return fmt.Errorf("invalid embedded spec: %w", err)
	}
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	cmd.SetOut(out)
	return cmd.Execute()
}

// runEmit parses args against sp and writes the resulting zsh to out — or, for --help or a
// bad arg, the equivalently-wrapped help/error text instead.
func runEmit(cmd *cobra.Command, sp spec.Command, args []string, out io.Writer) error {
	// cobra's help writes straight to stdout; buffer it so it can be re-wrapped instead of
	// corrupting Emit's own output on the same stream.
	var helpBuf bytes.Buffer
	cmd.SetOut(&helpBuf)

	parseResult, err := parse.Parse(cmd, sp, args)
	switch {
	// Encoded into the output, not a Go error: eval alone should stop the script on a bad
	// arg, no caller-side exit-code check needed.
	case err != nil:
		return zsh.EmitError(out, sp.Name, err)
	case parseResult.HelpRequested:
		return zsh.EmitHelp(out, helpBuf.String())
	default:
		return zsh.Emit(out, parseResult)
	}
}

// runGenerate writes sp's completion file (skip-if-unchanged, like activate's own _compleo)
// and reports success the same eval'd-zsh way EmitHelp/EmitError do.
func runGenerate(cmd *cobra.Command, sp spec.Command, rawSpec string, out io.Writer) error {
	dir, err := completionDir()
	if err != nil {
		return err
	}

	// name is what completion gets registered under. COMPLEO_GENERATE_NAME (from "generate
	// <script>") fills in an undeclared name:, but a declared one that disagrees is an error.
	name := sp.Name
	if requested := os.Getenv("COMPLEO_GENERATE_NAME"); requested != "" {
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
	if err := writeCompletionFile(dir, specFileName, specFileText); err != nil {
		return err
	}

	snippet, err := zsh.WrappedCompletion(cmd, name, filepath.Join(dir, specFileName))
	if err != nil {
		return err
	}
	fileName := "_" + name
	if err := writeCompletionFile(dir, fileName, snippet); err != nil {
		return err
	}
	return zsh.EmitGenerated(out, name)
}

// runGenerateFor is the "generate" subcommand: a convenience wrapper that runs script
// (a bare name resolved via $PATH, or a path) with COMPLEO_MODE=generate set, same as doing
// it by hand.
func runGenerateFor(script string, out io.Writer) error {
	path, err := exec.LookPath(script)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if !spec.IsCompleoScript(path) {
		return fmt.Errorf("generate: %s does not look like a compleo-wrapped script", script)
	}

	// filepath.Base(script), not script itself: script may be a path (exec.LookPath resolves
	// one directly, without even consulting $PATH), but COMPLEO_GENERATE_NAME must be a bare
	// name to match what the script's own "${0:t}" computes from its argv[0].
	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(), "COMPLEO_MODE="+modeGenerate, "COMPLEO_GENERATE_NAME="+filepath.Base(script))
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// compleoCmd builds compleo's own cobra command tree from its embedded spec. Panics on a
// parse failure — spec.yaml is built in via go:embed, so that'd be a bug, not a runtime error.
func compleoCmd(stdout io.Writer) (*cobra.Command, spec.Command) {
	cmd, sp, err := spec.ParseYAML(bytes.NewReader(specYAML), "")
	if err != nil {
		panic(fmt.Errorf("compleo's own embedded spec.yaml is invalid: %w", err))
	}
	cmd.SetOut(stdout)
	// Silenced: main() already reports the error; avoid cobra printing its own.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	return cmd, sp
}

// completionDir is where "activate" wires fpath to, and where generated completions live.
func completionDir() (string, error) {
	if dir := os.Getenv("COMPLEO_COMPLETION_DIR"); dir != "" {
		return dir, nil
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining completion directory: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "compleo", "completions"), nil
}

// writeCompletionFile skips the write when content is unchanged, so activate (which may run
// on every shell startup) doesn't bump the dir's mtime and force a full compinit rescan.
func writeCompletionFile(dir, name, content string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating completion directory: %w", err)
	}

	path := filepath.Join(dir, name)
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// isPiped reports whether f has data piped in, vs. an interactive terminal.
func isPiped(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}
