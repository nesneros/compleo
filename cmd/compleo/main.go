package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"

	carapacespec "github.com/carapace-sh/carapace-spec"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/nesneros/compleo/internal/usage"
)

//go:embed spec.yaml
var specYAML []byte

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "compleo:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	sp, err := spec.ParseYAML(bytes.NewReader(specYAML))
	if err != nil {
		return fmt.Errorf("invalid embedded spec: %w", err)
	}

	cmd, err := carapacespec.Command(sp.ToCarapace()).ToCobraE()
	if err != nil {
		return err
	}
	usage.Apply(cmd, sp)

	cmd.SetArgs(args)
	return cmd.Execute()
}
