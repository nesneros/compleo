package zsh

import "strings"

// Quote returns s as a single-quoted zsh word, safe to splice into emitted zsh code
// regardless of spaces, quotes, $, backticks, or newlines it contains.
func (Emitter) Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
