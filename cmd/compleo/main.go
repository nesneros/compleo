package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/nesneros/compleo/internal/zsh"
	"github.com/spf13/cobra"
)

//go:embed spec.yaml
var specYAML []byte

// version, commit, date are set at build time via GoReleaser's default ldflags.
var (
	version = "dev"
	commit  string
	date    string
)

func main() {
	if err := run(os.Args[1:], isPiped(os.Stdin), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// versionString adds commit/date for untagged snapshot builds (GoReleaser's default snapshot template includes "SNAPSHOT").
func versionString() string {
	if strings.Contains(version, "SNAPSHOT") {
		return fmt.Sprintf("%s (%v) [%v]", version, date, commit)
	}
	return version
}

// run routes on an explicit "parse" token, since --help/--version could otherwise be compleo's or the wrapped script's.
func run(args []string, piped bool, in io.Reader, out io.Writer) error {
	if args == nil {
		panic("run: args must not be nil (use an empty slice for \"no arguments\")")
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
		snippet, err := zsh.Completion(ownCmd, ownSpec)
		if err != nil {
			return err
		}
		if err := writeCompletionFile(dir, "_compleo", snippet+"\n"); err != nil {
			return err
		}
		fmt.Fprintf(out, "fpath+=(%s)\n", zsh.Quote(dir))
	case "completion":
		snippet, err := zsh.Completion(ownCmd, ownSpec)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, snippet)
	case "parse":
		// A variadic positional with zero values binds to nil, not an empty slice; Parse panics on nil.
		wrappedArgs := result.Positionals["args"].Values
		if wrappedArgs == nil {
			wrappedArgs = []string{}
		}
		return runParse(wrappedArgs, piped, in, out, result.Flags["dump"].Bool)
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

// runParse parses the wrapped script's own args against its spec piped on stdin.
// dump, if set, writes the resulting zsh to stderr too, before writing it to out.
func runParse(args []string, piped bool, in io.Reader, out io.Writer, dump bool) error {
	if !piped {
		return errors.New("no spec piped on stdin (compleo expects its target script's spec on stdin)")
	}
	cmd, sp, err := spec.ParseYAML(in)
	if err != nil {
		return fmt.Errorf("invalid spec: %w", err)
	}

	// cobra's help writes straight to os.Stdout by default; buffer it so it can be
	// re-wrapped as zsh instead of corrupting the real zsh.Emit output on the same stream.
	var helpBuf bytes.Buffer
	cmd.SetOut(&helpBuf)
	// Silenced: main() already reports the error; avoid cobra printing its own.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

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
