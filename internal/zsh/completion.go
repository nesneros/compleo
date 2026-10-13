package zsh

import (
	"fmt"
	"strings"

	"github.com/carapace-sh/carapace"
	"github.com/carapace-sh/carapace/pkg/uid"
	"github.com/spf13/cobra"
)

// Completion generates the zsh completion function for cmd. Suitable for compleo's own
// CLI, which can always re-invoke itself to serve live completions — its spec is embedded
// in the binary, not read from stdin. For a wrapped script's spec, use WrappedCompletion.
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

// WrappedCompletion generates the zsh completion function for a wrapped script's cmd.
// carapace's own generated snippet re-invokes the running binary (compleo) via
// "_carapace zsh", but compleo has no spec to serve on its own at that point — a wrapped
// script's spec only ever arrives piped on compleo's stdin, per invocation. So this patches
// the one line that matters: instead of "xargs <compleo> _carapace zsh", the snippet
// re-invokes the wrapped script itself (cmd.Name(), found via $PATH same as for completion
// to apply at all) with COMPLEO_MODE=complete, letting its own heredoc re-deliver the spec
// exactly like a normal run does. Everything else — the quote-retry dance, CARAPACE_* env
// vars, candidate formatting — is carapace's own logic, left untouched.
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
