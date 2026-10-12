package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
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
		fmt.Fprintln(os.Stderr, "compleo:", err)
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

	var arg0 string
	if len(args) > 0 {
		arg0 = args[0]
	}

	if arg0 == "parse" {
		return runPassthrough(args[1:], piped, in, out)
	}

	ownCmd, ownSpec, err := compleoCmd(out)
	if err != nil {
		return err
	}

	result, parseErr := parse.Parse(ownCmd, ownSpec, args)
	if parseErr != nil {
		return parseErr
	}

	// _carapace is carapace's own hidden subcommand (wired by carapace.Gen); it already ran above.
	if arg0 == "_carapace" {
		return nil
	}

	// cobra already printed --help and never reaches result.Flags; check Changed instead.
	if helpRequested(ownCmd) {
		return nil
	}

	switch result.Path[len(result.Path)-1] {
	case "version":
		fmt.Fprintln(out, versionString())
	case "validate":
		if !piped {
			return errors.New("no spec piped on stdin (compleo expects a spec to validate on stdin)")
		}
		if _, _, err := spec.ParseYAML(in); err != nil {
			return fmt.Errorf("invalid spec: %w", err)
		}
		fmt.Fprintln(out, "ok")
	case "completion":
		snippet, err := zsh.Completion(ownCmd, ownSpec)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, snippet)
	default:
		return ownCmd.Help()
	}
	return nil
}

// runPassthrough parses the wrapped script's own args against its spec piped on stdin.
func runPassthrough(args []string, piped bool, in io.Reader, out io.Writer) error {
	if !piped {
		return errors.New("no spec piped on stdin (compleo expects its target script's spec on stdin)")
	}
	wrappedCmd, wrappedSpec, err := spec.ParseYAML(in)
	if err != nil {
		return fmt.Errorf("invalid spec: %w", err)
	}

	parseResult, err := parse.Parse(wrappedCmd, wrappedSpec, args)
	if err != nil {
		return err
	}

	// Checked: a failed write here truncates the zsh the wrapped script evals.
	_, err = fmt.Fprintln(out, zsh.Emit(parseResult))
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

// helpRequested reports whether --help was changed anywhere in cmd's tree.
func helpRequested(cmd *cobra.Command) bool {
	if cmd.Flags().Changed("help") {
		return true
	}
	for _, sub := range cmd.Commands() {
		if helpRequested(sub) {
			return true
		}
	}
	return false
}

// isPiped reports whether f has data piped in, vs. an interactive terminal.
func isPiped(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}
