package spec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYAMLFillsInNameWhenSpecDoesNotDeclareOne(t *testing.T) {
	cmd, sp, err := ParseYAML(strings.NewReader("flags:\n  --loud: shout\n"), "greet")
	require.NoError(t, err)

	assert.Equal(t, "greet", sp.Name)
	assert.Equal(t, "greet", cmd.Name())
}

func TestParseYAMLAcceptsNameMatchingDeclaredOne(t *testing.T) {
	cmd, sp, err := ParseYAML(strings.NewReader("name: greet\n"), "greet")
	require.NoError(t, err)

	assert.Equal(t, "greet", sp.Name)
	assert.Equal(t, "greet", cmd.Name())
}

// A declared name: that disagrees with the given name is an error, not a silent override.
func TestParseYAMLRejectsNameDisagreeingWithDeclaredOne(t *testing.T) {
	_, _, err := ParseYAML(strings.NewReader("name: greet\n"), "other-name")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `spec declares name: "greet"`)
	assert.Contains(t, err.Error(), `"other-name" was given`)
}

func TestParseYAMLLeavesDeclaredNameAloneWhenNoneGiven(t *testing.T) {
	_, sp, err := ParseYAML(strings.NewReader("name: greet\n"), "")
	require.NoError(t, err)

	assert.Equal(t, "greet", sp.Name)
}
