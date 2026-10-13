package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/nesneros/compleo/internal/zsh"
	"github.com/spf13/cobra"
)

//go:embed spec.yaml
var specYAML []byte

// COMPLEO_MODE values: how a wrapped script's own "compleo parse -- "$@"" call behaves,
// beyond its default of parsing "$@". Set by the generated completion file ("complete")
// or by hand to (re)install it ("generate") — in both cases the wrapped script's own
// heredoc re-delivers its spec exactly as it does for a normal run, so no separate
// spec-delivery mechanism is needed for either mode.
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
		fmt.Fprintf(out, "fpath+=(%s)\n", zsh.Quote(dir))
	case "completion":
		snippet, err := zsh.Completion(ownCmd)
		if err != nil {
			return err
		}
		fmt.Fprint(out, snippet)
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

// runParse parses the wrapped script's own args against its spec piped on stdin, unless
// mode selects one of the COMPLEO_MODE variants instead (see the consts above).
// dump, if set, writes the resulting zsh to stderr too, before writing it to out.
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

// runComplete serves one live Tab-press completion request against cmd's own hidden
// _carapace subcommand (already wired by ToCobraE()), reporting the result the same way
// runEmit's EmitError/EmitHelp do: zsh the caller just evals, not a Go-level return value.
func runComplete(cmd *cobra.Command, args []string, out io.Writer) error {
	// args[0] is the typed command name (per carapace's own "_carapace zsh <name> <words>"
	// convention) — it's already there since args is the compline verbatim.
	var carapaceBuf bytes.Buffer
	cmd.SetArgs(append([]string{"_carapace", "zsh"}, args...))
	cmd.SetOut(&carapaceBuf)
	if err := cmd.Execute(); err != nil {
		return err
	}
	// carapace's wire format isn't zsh: the generated completion function captures it as
	// a plain string (its own lines="$(xargs greet ...)"), so wrap it the same way
	// EmitHelp/EmitError wrap their payloads, or the wrapped script's unconditional
	// eval would try to execute it as code instead of just passing it through.
	_, err := fmt.Fprintln(out, zsh.EmitComplete(carapaceBuf.String()))
	return err
}

// runEmit parses the wrapped script's own args against sp and writes the resulting zsh to
// out — or, for --help/a bad arg, the equivalently-wrapped help/error text instead.
// dump, if set, writes the same output to stderr too, before writing it to out.
func runEmit(cmd *cobra.Command, sp spec.Command, args []string, out io.Writer, dump bool) error {
	// cobra's help writes straight to os.Stdout by default; buffer it so it can be
	// re-wrapped as zsh instead of corrupting the real zsh.Emit output on the same stream.
	var helpBuf bytes.Buffer
	cmd.SetOut(&helpBuf)

	var output string
	parseResult, err := parse.Parse(cmd, sp, args)
	switch {
	// Encoded into the output, not returned as a Go error: the wrapped script's own
	// end-user args were bad, not compleo's invocation, so eval alone should stop the
	// script (print + exit 2) without the caller needing its own exit-code check.
	case err != nil:
		output = zsh.EmitError(sp.Name, err)
	case parseResult.HelpRequested:
		output = zsh.EmitHelp(helpBuf.String())
	default:
		output = zsh.Emit(parseResult)
	}

	if dump {
		fmt.Fprintln(os.Stderr, output)
	}
	// Checked: a failed write here truncates the zsh the wrapped script evals.
	_, err = fmt.Fprintln(out, output)
	return err
}

// runGenerate writes sp's wrapped-script completion file to completionDir() (same directory
// and skip-if-unchanged behavior as "activate"'s own _compleo) and reports success the same
// way EmitHelp/EmitError do: zsh that the caller just evals, not a Go-level return value.
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
	_, err = fmt.Fprintln(out, zsh.EmitGenerated(sp.Name, filepath.Join(dir, name)))
	return err
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

// completionDir is where "activate" wires fpath to, and where generated completions
// live: compleo's own (written by "activate" itself) and, eventually, wrapped scripts'.
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

// writeCompletionFile writes content to dir/name, creating dir if needed. It skips the
// write (and the file's mtime bump) when content already matches: activate may well run
// on every shell startup, and touching the file every time would make compinit think the
// directory changed, forcing a full fpath rescan instead of using its .zcompdump cache.
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
