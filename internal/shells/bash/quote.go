package bash

import "strings"

// Quote returns s as a single-quoted word. Bash accepts the same form zsh does.
func (Emitter) Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
