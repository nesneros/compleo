package zsh

import (
	"fmt"
	"strings"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"
)

// Completion generates the zsh completion function for cmd. Only for compleo's own CLI,
// whose spec is embedded in the binary — use WrappedCompletion for a wrapped script's spec.
func Completion(cmd *cobra.Command) (string, error) {
	snippet, err := carapace.Gen(cmd).Snippet("zsh")
	if err != nil {
		return "", err
	}
	// carapace's snippet does not end with a newline.
	if !strings.HasSuffix(snippet, "\n") {
		snippet += "\n"
	}
	return snippet, nil
}

// WrappedCompletion generates the completion function for a wrapped script's cmd, registered
// under name — the real, invokable name, not necessarily cmd.Name() (which is just whatever
// the spec declares and can drift from it; the caller's call, not inferred here).
// specFilePath is where the caller already wrote the spec on disk; only the path is embedded
// (as a function-local exported COMPLEO_SPEC_FILE), to avoid an env var's size limits.
// carapace's own invocation is left untouched, so a Tab press reads that file directly — the
// wrapped script itself only needs to be on $PATH once, at generate time.
func WrappedCompletion(cmd *cobra.Command, name string, specFilePath string) (string, error) {
	renameCommand(cmd, name)
	snippet, err := Completion(cmd)
	if err != nil {
		return "", err
	}
	anchor := "function _" + name + "_completion {\n"
	if !strings.Contains(snippet, anchor) {
		return "", fmt.Errorf("zsh: generated snippet did not contain expected function definition %q (carapace's zsh template may have changed)", anchor)
	}
	inject := anchor + "  local -x COMPLEO_SPEC_FILE=" + Quote(specFilePath) + "\n"
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
