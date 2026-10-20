package bash

import (
	"fmt"
	"strings"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/shells"
	"github.com/spf13/cobra"
)

// WrappedCompletion generates the bash completion function for a wrapped script's cmd,
// registering it under name. specFilePath is embedded into the snippet as EnvSpecFile.
func (e Emitter) WrappedCompletion(cmd *cobra.Command, name string, specFilePath string) (string, error) {
	renameCommand(cmd, name)
	snippet, err := shells.Completion(cmd, e.Name())
	if err != nil {
		return "", err
	}
	anchor := "_" + name + "_completion() {\n"
	if !strings.Contains(snippet, anchor) {
		return "", fmt.Errorf("%s: generated snippet did not contain expected function definition %q (carapace's %s template may have changed)", e.Name(), anchor, e.Name())
	}
	inject := anchor + "  local -x " + cmdutil.EnvSpecFile + "=" + e.Quote(specFilePath) + "\n"
	return strings.Replace(snippet, anchor, inject, 1), nil
}

// renameCommand overwrites cmd.Use's leading token — what cmd.Name() reads, and what
// carapace's snippet names the completion function after.
func renameCommand(cmd *cobra.Command, name string) {
	if i := strings.IndexByte(cmd.Use, ' '); i >= 0 {
		cmd.Use = name + cmd.Use[i:]
		return
	}
	cmd.Use = name
}
