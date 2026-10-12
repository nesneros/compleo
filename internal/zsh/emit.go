package zsh

import "github.com/nesneros/compleo/internal/parse"

// Emit generates the zsh script the caller evals. Not yet implemented — array naming,
// multi-valued layout, and boolean-flag convention are still open (see CLAUDE.md).
func Emit(result parse.Result) string {
	return `print -u2 -- "compleo: zsh script generation not implemented yet"`
}
