package bash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bashRoundTrip execs bash with no startup files to print the quoted string.
func bashRoundTrip(t *testing.T, s string) string {
	t.Helper()
	out, err := bashCmd("printf '%s\\n' " + Em.Quote(s)).Output()
	require.NoError(t, err)
	return string(out)
}

func TestQuoteRoundTripsSpaces(t *testing.T) {
	assert.Equal(t, "hello world\n", bashRoundTrip(t, "hello world"))
}

func TestQuoteRoundTripsSingleQuote(t *testing.T) {
	assert.Equal(t, "it's\n", bashRoundTrip(t, "it's"))
}

func TestQuoteRoundTripsDollarAndBacktick(t *testing.T) {
	assert.Equal(t, "$HOME `whoami`\n", bashRoundTrip(t, "$HOME `whoami`"))
}

func TestQuoteRoundTripsNewline(t *testing.T) {
	assert.Equal(t, "line1\nline2\n", bashRoundTrip(t, "line1\nline2"))
}
