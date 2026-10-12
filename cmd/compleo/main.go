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

// version, commit, date are set at build time via GoReleaser's default ldflags
// (-X main.version=... -X main.commit=... -X main.date=...).
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

// versionString appends commit/date to version for untagged snapshot builds (GoReleaser's
// default snapshot template includes "SNAPSHOT"), leaving tagged releases unadorned.
func versionString() string {
	if strings.Contains(version, "SNAPSHOT") {
		return fmt.Sprintf("%s (%v) [%v]", version, date, commit)
	}
	return version
}

// run tries args against compleo's own spec first (via parse.Parse, so new flags just
// need a spec.yaml entry and a case below). If that fails or matches no flag, args is
// the wrapped script's own arguments, requiring a spec piped on stdin.
func run(args []string, piped bool, in io.Reader, out io.Writer) error {
	ownCmd, ownSpec, err := compleoCmd(out)
	if err != nil {
		return err
	}

	result, parseErr := parse.Parse(ownCmd, ownSpec, args)

	// cobra intercepts --help before RunE runs and already printed it above (via
	// SetOut), so it never reaches result.Flags; check Changed directly instead.
	if ownCmd.Flags().Changed("help") {
		return nil
	}

	action := "emit"
	if parseErr == nil {
		switch {
		case result.Flags["version"].Bool:
			fmt.Fprintln(out, versionString())
			return nil
		case result.Flags["validate"].Bool:
			action = "validate"
		}
	}

	// Both actions below read the spec piped on stdin.
	if !piped {
		return errors.New("no spec piped on stdin (compleo expects its target script's spec on stdin)")
	}
	wrappedCmd, wrappedSpec, err := spec.ParseYAML(in)
	if err != nil {
		return fmt.Errorf("invalid spec: %w", err)
	}

	switch action {
	case "validate":
		fmt.Fprintln(out, "ok")

	case "emit":
		parseResult, err := parse.Parse(wrappedCmd, wrappedSpec, args)
		if err != nil {
			return err
		}

		fmt.Fprintln(out, zsh.Emit(parseResult))

	default:
		panic("unreachable")
	}

	return nil
}

// compleoCmd builds compleo's own cobra command tree from its embedded spec.
func compleoCmd(stdout io.Writer) (*cobra.Command, spec.Command, error) {
	cmd, sp, err := spec.ParseYAML(bytes.NewReader(specYAML))
	if err != nil {
		return nil, spec.Command{}, fmt.Errorf("invalid embedded spec: %w", err)
	}
	cmd.SetOut(stdout)
	// Silenced: a failed trial parse (the wrapped script's own flags, unknown to
	// compleo) is expected and falls through to passthrough below, not an error.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	return cmd, sp, nil
}

// isPiped reports whether f has data piped in, vs. an interactive terminal.
func isPiped(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}
