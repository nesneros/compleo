package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/generate"
	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/shells/zsh"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/spf13/cobra"
)

//go:embed spec.yaml
var specYAML []byte

func init() {
	// Detection will replace these. Until then both shells are zsh.
	shells.SetCompletionEmitter(zsh.Em)
	shells.SetEvalEmitter(zsh.Em)
}

func main() {
	generateName := os.Getenv(cmdutil.EnvGenerateName)
	if err := run(os.Args[1:], isPiped(os.Stdin), os.Stdin, os.Stdout, generateName); err != nil {
		fmt.Fprintln(os.Stderr, err)
		logDebug(os.Args[1:], err)
		os.Exit(1)
	}
}

// logDebug appends a timestamped record of args and err to the file named by EnvDebugLog, if
// set. It is a no-op if the env var is unset or the file can't be opened.
func logDebug(args []string, err error) {
	path := os.Getenv(cmdutil.EnvDebugLog)
	if path == "" {
		return
	}
	f, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if openErr != nil {
		return
	}
	defer f.Close() //nolint:errcheck
	ts := time.Now().UTC().Format(time.RFC3339)
	fmt.Fprintf(f, "%s args=%q: %s\n", ts, strings.Join(args, " "), err)
}

// isPiped reports whether f has data piped in, vs. an interactive terminal.
func isPiped(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}

// run parses args as a compleo invocation and dispatches to the matching subcommand,
// writing its output to out.
func run(args []string, piped bool, in io.Reader, out io.Writer, generateName string) error {
	if args == nil {
		panic("run: args must not be nil (use an empty slice for 'no arguments')")
	}

	// A live Tab press for a wrapped script's own completion: its generated completion file
	// sets EnvSpecFile (see WrappedCompletion) before calling "_carapace zsh ...", since
	// ownCmd's own _carapace subcommand below only knows how to complete compleo itself.
	if len(args) > 0 && args[0] == "_carapace" {
		if specFile := os.Getenv(cmdutil.EnvSpecFile); specFile != "" {
			data, err := os.ReadFile(specFile)
			if err != nil {
				return fmt.Errorf("reading embedded spec file %q: %w", specFile, err)
			}
			return runCarapace(string(data), args, out)
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

	cmdName := result.Path[len(result.Path)-1]
	var rawSpec string
	// Ensure stdin is piped for commands that require it, and read rawSpec from it.
	if cmdName == "parse" || cmdName == "up" || cmdName == "validate" {
		if !piped {
			return errors.New("no spec piped on stdin")
		}
		rawSpecBytes, err := io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("reading spec: %w", err)
		}
		rawSpec = string(rawSpecBytes)
	}
	switch cmdName {
	case "activate":
		dir := cmdutil.CompletionDir()
		em := shells.GetCompletionEmitter()
		snippet, err := shells.Completion(ownCmd, em.Name())
		if err != nil {
			return err
		}
		if err := cmdutil.WriteCompletionFile(dir, "_"+ownSpec.Name, snippet); err != nil {
			return err
		}
		return em.EmitActivate(out, dir)
	case "generate":
		return generate.GenerateFor(result.Positionals["script"].Values[0], out)
	case "parse":
		// A variadic positional with zero values binds to nil, not an empty slice; Parse panics on nil.
		wrappedArgs := result.Positionals["args"].Values
		if wrappedArgs == nil {
			wrappedArgs = []string{}
		}
		return runParse(wrappedArgs, rawSpec, out, result.Flags["name"].String, generateName)
	case "up":
		return shells.GetEvalEmitter().EmitUp(out, rawSpec)
	case "validate":
		if _, _, err := spec.ParseYAML(rawSpec, ""); err != nil {
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

// runParse parses args against the spec in rawSpec, or, when generateName is non-empty,
// writes a completion file for it instead. name is passed to spec.ParseYAML as the spec's name.
func runParse(args []string, rawSpec string, out io.Writer, name string, generateName string) error {
	cmd, sp, err := spec.ParseYAML(rawSpec, name)
	if err != nil {
		return fmt.Errorf("invalid spec: %w", err)
	}
	// Silenced: main() already reports the error; avoid cobra printing its own.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if generateName != "" {
		return generate.Generate(cmd, sp, rawSpec, generateName, out)
	}
	return runEmit(cmd, sp, args, out)
}

// runCarapace builds a cobra command tree from specYAML and executes args against it,
// writing to out. A failure in the underlying completion command is reported to out rather
// than returned.
func runCarapace(specYAML string, args []string, out io.Writer) error {
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
	// corrupting EmitResult's own output on the same stream.
	var helpBuf bytes.Buffer
	cmd.SetOut(&helpBuf)

	em := shells.GetEvalEmitter()
	parseResult, err := parse.Parse(cmd, sp, args)
	switch {
	// Encoded into the output, not a Go error: eval alone should stop the script on a bad
	// arg, no caller-side exit-code check needed.
	case err != nil:
		return em.EmitError(out, sp.Name, err)
	case parseResult.HelpRequested:
		return em.EmitHelp(out, helpBuf.String())
	default:
		return em.EmitResult(out, parseResult)
	}
}

// compleoCmd builds compleo's own cobra command tree from its embedded spec.yaml. It panics
// if that spec fails to parse.
func compleoCmd(stdout io.Writer) (*cobra.Command, spec.Command) {
	cmd, sp, err := spec.ParseYAML(string(specYAML), "")
	if err != nil {
		panic(fmt.Errorf("compleo's own embedded spec.yaml is invalid: %w", err))
	}
	cmd.SetOut(stdout)
	// Silenced: main() already reports the error; avoid cobra printing its own.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	return cmd, sp
}
