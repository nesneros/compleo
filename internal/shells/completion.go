package shells

import (
	"strings"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"
)

// Completion returns carapace's completion snippet for shell, the name from Emitter.Name.
// The snippet is the same for every shell except that name.
func Completion(cmd *cobra.Command, shell string) (string, error) {
	snippet, err := carapace.Gen(cmd).Snippet(shell)
	if err != nil {
		return "", err
	}
	// carapace's snippet does not end with a newline.
	if !strings.HasSuffix(snippet, "\n") {
		snippet += "\n"
	}
	return snippet, nil
}
