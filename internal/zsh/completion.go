package zsh

import (
	"github.com/carapace-sh/carapace"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/spf13/cobra"
)

// Completion generates the zsh completion function for cmd. sp is unused for now (see CLAUDE.md open questions).
func Completion(cmd *cobra.Command, sp spec.Command) (string, error) {
	return carapace.Gen(cmd).Snippet("zsh")
}
