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
	assert.Equal(t, typePath, p.Type)
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

	assert.Equal(t, typeInt, cmd.FlagTypes["count"])
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

	var _ carapacespec.Command = converted //nolint:staticcheck // QF1011: type is the point, asserts toCarapace's return type
}

func TestToCarapaceAddsTypeHintToTypedFlagDescription(t *testing.T) {
	cmd, err := decodeYAML(strings.NewReader(`
name: mytool
flags:
  --times=: how many times
flagtypes:
  times: int
`))
	require.NoError(t, err)

	converted := cmd.toCarapace()

	assert.Equal(t, "how many times (`int`)", converted.Flags["times"].Description)
}

func TestToCarapaceLeavesUntypedFlagDescriptionAlone(t *testing.T) {
	cmd, err := decodeYAML(strings.NewReader(`
name: mytool
flags:
  --name=: a name
`))
	require.NoError(t, err)

	converted := cmd.toCarapace()

	assert.Equal(t, "a name", converted.Flags["name"].Description)
}

// End-to-end: confirms the hint actually changes cobra's rendered usage text, not just
// the intermediate carapace-spec struct — pflag.UnquoteUsage is what does this, and it's
// easy to get its backtick convention subtly wrong without seeing the real output.
func TestParseYAMLUsageShowsDeclaredFlagType(t *testing.T) {
	cmd, _, err := ParseYAML(strings.NewReader(`
name: mytool
flags:
  --times=: how many times
flagtypes:
  times: int
`), "")
	require.NoError(t, err)

	assert.Contains(t, cmd.UsageString(), "--times int")
	assert.NotContains(t, cmd.UsageString(), "--times string")
}

func TestTypeValidateIntAcceptsValidValue(t *testing.T) {
	assert.NoError(t, typeInt.Validate("3"))
}

func TestTypeValidateIntRejectsNonNumeric(t *testing.T) {
	assert.Error(t, typeInt.Validate("abc"))
}

func TestTypeValidateFloatRejectsNonNumeric(t *testing.T) {
	assert.Error(t, typeFloat.Validate("abc"))
}

func TestTypeValidateBoolRejectsNonBool(t *testing.T) {
	assert.Error(t, typeBool.Validate("maybe"))
}

func TestTypeValidateDurationRejectsInvalid(t *testing.T) {
	assert.Error(t, typeDuration.Validate("soon"))
}

func TestTypeValidateDurationAcceptsValid(t *testing.T) {
	assert.NoError(t, typeDuration.Validate("5m"))
}

func TestTypeValidateStringAndPathAcceptAnything(t *testing.T) {
	assert.NoError(t, typeString.Validate("anything at all"))
	assert.NoError(t, typePath.Validate("anything at all"))
}
