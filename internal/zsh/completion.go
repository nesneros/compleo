package zsh

import (
	"strings"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"
)

// Completion generates the zsh completion function for cmd.
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
