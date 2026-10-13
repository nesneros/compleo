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

	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/nesneros/compleo/internal/zsh"
	"github.com/spf13/cobra"
)

//go:embed spec.yaml
var specYAML []byte

// COMPLEO_MODE values: what "compleo parse" does beyond parsing "$@" — same heredoc
// re-delivers the spec either way, so no mode-aware logic is needed in the script itself.
const (
	modeGenerate = "generate"
	modeComplete = "complete"
)

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

	ownCmd, ownSpec, err := compleoCmd(out)
	if err != nil {
		return err
	}

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
		return runGenerateFor(result.Positionals["name"].Values[0], out)
	case "parse":
		// A variadic positional with zero values binds to nil, not an empty slice; Parse panics on nil.
		wrappedArgs := result.Positionals["args"].Values
		if wrappedArgs == nil {
			wrappedArgs = []string{}
		}
		return runParse(wrappedArgs, piped, in, out, result.Flags["dump"].Bool, mode)
	case "validate":
		if !piped {
			return errors.New("no spec piped on stdin (compleo expects a spec to validate on stdin)")
		}
		if _, _, err := spec.ParseYAML(in); err != nil {
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

// runParse parses args against the piped spec, or dispatches to a COMPLEO_MODE variant.
// dump, if set, also writes the result to stderr before writing it to out.
func runParse(args []string, piped bool, in io.Reader, out io.Writer, dump bool, mode string) error {
	if !piped {
		return errors.New("no spec piped on stdin (compleo expects its target script's spec on stdin)")
	}
	cmd, sp, err := spec.ParseYAML(in)
	if err != nil {
		return fmt.Errorf("invalid spec: %w", err)
	}
	// Silenced: main() already reports the error; avoid cobra printing its own.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	switch mode {
	case modeGenerate:
		return runGenerate(cmd, sp, out)
	case modeComplete:
		return runComplete(cmd, args, out)
	default:
		return runEmit(cmd, sp, args, out, dump)
	}
}

// runComplete serves one live Tab-press request via cmd's own hidden _carapace subcommand,
// reporting the result the same eval'd-zsh way EmitError/EmitHelp do.
func runComplete(cmd *cobra.Command, args []string, out io.Writer) error {
	// args[0] is the typed command name, per carapace's own "_carapace zsh <name> <words>" convention.
	var carapaceBuf bytes.Buffer
	cmd.SetArgs(append([]string{"_carapace", "zsh"}, args...))
	cmd.SetOut(&carapaceBuf)
	if err := cmd.Execute(); err != nil {
		return err
	}
	// Wrapped like EmitHelp/EmitError: carapace's wire format isn't zsh, and the wrapped
	// script's unconditional eval would otherwise try to execute it as code.
	return zsh.EmitComplete(out, carapaceBuf.String())
}

// runEmit parses args against sp and writes the resulting zsh to out — or, for --help or a
// bad arg, the equivalently-wrapped help/error text instead. dump also writes it to stderr.
func runEmit(cmd *cobra.Command, sp spec.Command, args []string, out io.Writer, dump bool) error {
	// cobra's help writes straight to stdout; buffer it so it can be re-wrapped instead of
	// corrupting Emit's own output on the same stream.
	var helpBuf bytes.Buffer
	cmd.SetOut(&helpBuf)

	w := out
	if dump {
		w = io.MultiWriter(out, os.Stderr)
	}

	parseResult, err := parse.Parse(cmd, sp, args)
	switch {
	// Encoded into the output, not a Go error: eval alone should stop the script on a bad
	// arg, no caller-side exit-code check needed.
	case err != nil:
		return zsh.EmitError(w, sp.Name, err)
	case parseResult.HelpRequested:
		return zsh.EmitHelp(w, helpBuf.String())
	default:
		return zsh.Emit(w, parseResult)
	}
}

// runGenerate writes sp's completion file (skip-if-unchanged, like activate's own _compleo)
// and reports success the same eval'd-zsh way EmitHelp/EmitError do.
func runGenerate(cmd *cobra.Command, sp spec.Command, out io.Writer) error {
	dir, err := completionDir()
	if err != nil {
		return err
	}
	snippet, err := zsh.WrappedCompletion(cmd)
	if err != nil {
		return err
	}
	name := "_" + sp.Name
	if err := writeCompletionFile(dir, name, snippet); err != nil {
		return err
	}
	return zsh.EmitGenerated(out, sp.Name, filepath.Join(dir, name))
}

// runGenerateFor is the "generate" subcommand: a convenience wrapper that runs name
// (resolved via $PATH) with COMPLEO_MODE=generate set, same as doing it by hand.
func runGenerateFor(name string, out io.Writer) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if !spec.IsCompleoScript(path) {
		return fmt.Errorf("generate: %s does not look like a compleo-wrapped script", name)
	}

	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(), "COMPLEO_MODE="+modeGenerate)
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// compleoCmd builds compleo's own cobra command tree from its embedded spec.
func compleoCmd(stdout io.Writer) (*cobra.Command, spec.Command, error) {
	cmd, sp, err := spec.ParseYAML(bytes.NewReader(specYAML))
	if err != nil {
		return nil, spec.Command{}, fmt.Errorf("invalid embedded spec: %w", err)
	}
	cmd.SetOut(stdout)
	// Silenced: main() already reports the error; avoid cobra printing its own.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	return cmd, sp, nil
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
