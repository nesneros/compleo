package spec

import (
	"strings"
	"testing"

	carapacespec "github.com/carapace-sh/carapace-spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlagDescriptionAndDefault(t *testing.T) {
	cmd, err := decodeYAML(strings.NewReader(`
name: mytool
flags:
  --count=: {description: how many times, default: "1"}
`))
	require.NoError(t, err)

	f, ok := cmd.Flags["count"]
	require.True(t, ok, "flag \"count\" not decoded")
	assert.Equal(t, "how many times", f.Description)
	assert.Equal(t, "1", f.Default)
}

func TestParseRequiredFlag(t *testing.T) {
	cmd, err := decodeYAML(strings.NewReader(`
name: mytool
flags:
  --verbose!: verbose output
`))
	require.NoError(t, err)

	f, ok := cmd.Flags["verbose"]
	require.True(t, ok, "flag \"verbose\" not decoded")
	assert.True(t, f.Required)
}

func TestParsePositionalTypeAndRequired(t *testing.T) {
	cmd, err := decodeYAML(strings.NewReader(`
name: mytool
positionals:
  - name: file
    description: input file
    required: true
    type: path
`))
	require.NoError(t, err)
	require.Len(t, cmd.Positionals, 1)

	p := cmd.Positionals[0]
	assert.Equal(t, "file", p.Name)
	assert.True(t, p.Required)
	assert.Equal(t, TypePath, p.Type)
}

func TestParseFlagTypeOverlay(t *testing.T) {
	cmd, err := decodeYAML(strings.NewReader(`
name: mytool
flags:
  --count=: consumes a count
flagtypes:
  count: int
`))
	require.NoError(t, err)

	assert.Equal(t, TypeInt, cmd.FlagTypes["count"])
}

func TestParseSubcommandCarriesItsOwnPositionals(t *testing.T) {
	cmd, err := decodeYAML(strings.NewReader(`
name: mytool
commands:
  - name: sub
    description: a subcommand
    positionals:
      - name: arg
        required: true
`))
	require.NoError(t, err)
	require.Len(t, cmd.Commands, 1)

	sub := cmd.Commands[0]
	assert.Equal(t, "sub", sub.Name)
	require.Len(t, sub.Positionals, 1)
	assert.Equal(t, "arg", sub.Positionals[0].Name)
	assert.True(t, sub.Positionals[0].Required)

	// The embedded command.Command's own Commands field is superseded by cmd.Commands
	// above and must be cleared, not left as a stale duplicate.
	assert.Empty(t, cmd.Command.Commands)
}

func TestCommandToCarapace(t *testing.T) {
	cmd, err := decodeYAML(strings.NewReader(`
name: mytool
flags:
  --count=: {description: how many times, default: "1"}
positionals:
  - name: file
    required: true
commands:
  - name: sub
    positionals:
      - name: arg
`))
	require.NoError(t, err)

	converted := cmd.toCarapace()

	assert.Equal(t, "mytool", converted.Name)
	assert.Contains(t, converted.Flags, "count")
	require.Len(t, converted.Commands, 1)
	assert.Equal(t, "sub", converted.Commands[0].Name)

	//nolint:staticcheck // QF1011: the explicit type is the point — it's a compile-time
	// check that toCarapace returns carapacespec.Command, not just command.Command.
	var _ carapacespec.Command = converted
}
