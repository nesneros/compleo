package nu

import (
	"fmt"
	"strings"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/shells"
	"github.com/spf13/cobra"
)

// WrappedCompletion generates a nu file that registers name's completer closure.
// specFilePath is passed to the child compleo as EnvSpecFile. Nushell keeps one external
// completer, so the closure is stored for the dispatcher activate installs.
func (e Emitter) WrappedCompletion(cmd *cobra.Command, name string, specFilePath string) (string, error) {
	renameCommand(cmd, name)
	snippet, err := shells.Completion(cmd, e)
	if err != nil {
		return "", err
	}
	invocation, err := invocationLine(snippet)
	if err != nil {
		return "", err
	}
	// do { let } so sourcing the file twice does not fail on a redefinition.
	// upsert runs a bare closure; the block returns $completer so it is stored.
	return fmt.Sprintf(`do {
  let completer = {|place|
    let spans = (if ($place | describe) =~ "record" { $place.command } else { $place })
    with-env { %s: %s } {
      %s
    }
  }
  $env._COMPLEO_NU_COMPLETERS = ((try { $env._COMPLEO_NU_COMPLETERS } catch { {} }) | upsert %s { $completer })
}
`, cmdutil.EnvSpecFile, e.Quote(specFilePath), invocation, e.Quote(name)), nil
}

func invocationLine(snippet string) (string, error) {
	for _, line := range strings.Split(snippet, "\n") {
		if strings.Contains(line, "_carapace nushell") {
			return strings.TrimSpace(line), nil
		}
	}
	return "", fmt.Errorf("nu: generated snippet did not contain a _carapace nushell invocation (carapace's nushell template may have changed)")
}

// renameCommand overwrites cmd.Use's leading token — what cmd.Name() reads, and what
// carapace's snippet names the completion after.
func renameCommand(cmd *cobra.Command, name string) {
	if i := strings.IndexByte(cmd.Use, ' '); i >= 0 {
		cmd.Use = name + cmd.Use[i:]
		return
	}
	cmd.Use = name
}
