package posix

import "strings"

// Quote returns s as a single-quoted word. An embedded ' is written as '\”.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Quote returns s as a single-quoted word.
func (Emitter) Quote(s string) string { return Quote(s) }
