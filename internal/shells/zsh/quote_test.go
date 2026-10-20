package zsh

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zshRoundTrip execs zsh -f (no rc files) to print the quoted string, proving the quoting is
// actually safe to splice into zsh code, not just plausible-looking Go string output.
func zshRoundTrip(t *testing.T, s string) string {
	t.Helper()

	cmd := exec.Command("zsh", "-fc", "print -r -- "+Em.Quote(s))
	out, err := cmd.Output()
	require.NoError(t, err)
	return string(out)
}

func TestQuoteRoundTripsSpaces(t *testing.T) {
	assert.Equal(t, "hello world\n", zshRoundTrip(t, "hello world"))
}

func TestQuoteRoundTripsSingleQuote(t *testing.T) {
	assert.Equal(t, "it's\n", zshRoundTrip(t, "it's"))
}

func TestQuoteRoundTripsDollarAndBacktick(t *testing.T) {
	assert.Equal(t, "$HOME `whoami`\n", zshRoundTrip(t, "$HOME `whoami`"))
}

func TestQuoteRoundTripsNewline(t *testing.T) {
	assert.Equal(t, "line1\nline2\n", zshRoundTrip(t, "line1\nline2"))
}
