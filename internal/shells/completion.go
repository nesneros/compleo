package shells

import (
	"strings"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"
)

// Completion returns carapace's completion snippet for emitter.
// The snippet is the same for every shell except the shell name carapace is given.
// compleo calls nu "nu" (the process name); carapace's snippet name for it is "nushell".
func Completion(cmd *cobra.Command, emitter CompletionEmitter) (string, error) {
	snippet, err := carapace.Gen(cmd).Snippet(carapaceShell(emitter.Name()))
	if err != nil {
		return "", err
	}
	// carapace's snippet does not end with a newline.
	if !strings.HasSuffix(snippet, "\n") {
		snippet += "\n"
	}
	return snippet, nil
}

func carapaceShell(name string) string {
	if name == "nu" {
		return "nushell"
	}
	return name
}
