package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYAMLFillsInNameWhenSpecDoesNotDeclareOne(t *testing.T) {
	cmd, sp, err := ParseYAML("flags:\n  --loud: shout\n", "greet")
	require.NoError(t, err)

	assert.Equal(t, "greet", sp.Name)
	assert.Equal(t, "greet", cmd.Name())
}

func TestParseYAMLAcceptsNameMatchingDeclaredOne(t *testing.T) {
	cmd, sp, err := ParseYAML("name: greet\n", "greet")
	require.NoError(t, err)

	assert.Equal(t, "greet", sp.Name)
	assert.Equal(t, "greet", cmd.Name())
}

// A declared name: that disagrees with the given name is an error, not a silent override.
func TestParseYAMLRejectsNameDisagreeingWithDeclaredOne(t *testing.T) {
	_, _, err := ParseYAML("name: greet\n", "other-name")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `spec declares name: "greet"`)
	assert.Contains(t, err.Error(), `"other-name" was given`)
}

func TestParseYAMLLeavesDeclaredNameAloneWhenNoneGiven(t *testing.T) {
	_, sp, err := ParseYAML("name: greet\n", "")
	require.NoError(t, err)

	assert.Equal(t, "greet", sp.Name)
}

// Command no longer has a custom UnmarshalYAML re-decoding the document through a fresh
// *yaml.Node (which didn't inherit the outer Decoder's KnownFields(true)) — so a stray
// top-level key, like the now-removed flagtypes:, is finally actually caught.
func TestParseYAMLRejectsUnknownTopLevelKey(t *testing.T) {
	_, _, err := ParseYAML("name: greet\nflagtypes:\n  times: int\n", "")
	assert.Error(t, err)
}

func TestParseYAMLLeavesPrefixEmptyWhenNotDeclared(t *testing.T) {
	_, sp, err := ParseYAML("name: greet\n", "")
	require.NoError(t, err)
	assert.Empty(t, sp.Prefix)
}

func TestParseYAMLAcceptsDeclaredPrefix(t *testing.T) {
	_, sp, err := ParseYAML("name: greet\nprefix: tool\n", "")
	require.NoError(t, err)
	assert.Equal(t, "tool", sp.Prefix)
}

func TestParseYAMLRejectsPrefixWithIllegalCharacters(t *testing.T) {
	_, _, err := ParseYAML("name: greet\nprefix: my-tool\n", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `prefix "my-tool"`)
}

func TestParseYAMLRejectsPrefixStartingWithDigit(t *testing.T) {
	_, _, err := ParseYAML("name: greet\nprefix: 1tool\n", "")
	assert.Error(t, err)
}
