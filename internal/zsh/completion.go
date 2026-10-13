package zsh

import (
	"fmt"
	"strings"

	"github.com/carapace-sh/carapace"
	"github.com/carapace-sh/carapace/pkg/uid"
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

// WrappedCompletion generates the zsh completion function for a wrapped script's cmd. Patches
// carapace's own snippet to re-invoke the wrapped script (COMPLEO_MODE=complete) instead of
// compleo itself, since compleo has no spec to serve outside of what's piped on its stdin.
func WrappedCompletion(cmd *cobra.Command) (string, error) {
	snippet, err := Completion(cmd)
	if err != nil {
		return "", err
	}
	oldInvocation := "xargs " + uid.Executable() + " _carapace zsh"
	if !strings.Contains(snippet, oldInvocation) {
		return "", fmt.Errorf("zsh: generated snippet did not contain expected invocation %q (carapace's zsh template may have changed)", oldInvocation)
	}
	newInvocation := "COMPLEO_MODE=complete xargs " + cmd.Name()
	return strings.ReplaceAll(snippet, oldInvocation, newInvocation), nil
}
