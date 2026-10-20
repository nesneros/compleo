package zsh

import (
	"fmt"
	"strings"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/shells"
	"github.com/spf13/cobra"
)

// WrappedCompletion generates the completion function for a wrapped script's cmd,
// registering it under name (which may differ from cmd.Name()). specFilePath is embedded
// into the snippet as EnvSpecFile.
func (e Emitter) WrappedCompletion(cmd *cobra.Command, name string, specFilePath string) (string, error) {
	renameCommand(cmd, name)
	snippet, err := shells.Completion(cmd, "zsh")
	if err != nil {
		return "", err
	}
	anchor := "function _" + name + "_completion {\n"
	if !strings.Contains(snippet, anchor) {
		return "", fmt.Errorf("zsh: generated snippet did not contain expected function definition %q (carapace's zsh template may have changed)", anchor)
	}
	inject := anchor + "  local -x " + cmdutil.EnvSpecFile + "=" + e.Quote(specFilePath) + "\n"
	return strings.Replace(snippet, anchor, inject, 1), nil
}

// renameCommand overwrites cmd.Use's leading token — what cmd.Name() reads, and what
// carapace's own snippet generation names #compdef/the function after — keeping any
// trailing usage synopsis ("[flags] <file>") intact.
func renameCommand(cmd *cobra.Command, name string) {
	if i := strings.IndexByte(cmd.Use, ' '); i >= 0 {
		cmd.Use = name + cmd.Use[i:]
		return
	}
	cmd.Use = name
}
