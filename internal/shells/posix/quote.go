package posix

import "strings"

// Quote returns s as a single-quoted word. An embedded ' is written as '\”.
func (Emitter) Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
