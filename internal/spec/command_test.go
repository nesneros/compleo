package spec

import (
	"testing"

	carapacespec "github.com/carapace-sh/carapace-spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlagDescriptionAndDefault(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --count=: {description: how many times, default: "1"}
`)
	require.NoError(t, err)

	f, ok := cmd.Flags["count"]
	require.True(t, ok, "flag \"count\" not decoded")
	assert.Equal(t, "how many times", f.Description)
	assert.Equal(t, "1", f.Default)
}

func TestParseRequiredFlag(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --verbose!: verbose output
`)
	require.NoError(t, err)

	f, ok := cmd.Flags["verbose"]
	require.True(t, ok, "flag \"verbose\" not decoded")
	assert.True(t, f.Required)
}

func TestParsePositionalTypeAndRequired(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
positionals:
  - name: file
    description: input file
    required: true
    type: path
`)
	require.NoError(t, err)
	require.Len(t, cmd.Positionals, 1)

	p := cmd.Positionals[0]
	assert.Equal(t, "file", p.Name)
	assert.True(t, p.Required)
	assert.Equal(t, PathType{}, p.Type)
}

func TestParseFlagTypeIsSpecifiedOnTheFlagItself(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --count=: {description: consumes a count, type: int}
`)
	require.NoError(t, err)

	assert.Equal(t, IntType{}, cmd.Flags["count"].Type)
}

func TestParseFlagTypeBareStringIsShorthandForKindOnlyObject(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --count=: {type: int}
  --count2=: {type: {kind: int}}
`)
	require.NoError(t, err)

	assert.Equal(t, cmd.Flags["count2"].Type, cmd.Flags["count"].Type)
}

func TestParseFlagIntTypeWithMinMax(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --port=: {description: a port, type: {kind: int, min: 1, max: 65535}}
`)
	require.NoError(t, err)

	one, max := 1, 65535
	assert.Equal(t, IntType{Min: &one, Max: &max}, cmd.Flags["port"].Type)
}

func TestParseFlagFloatTypeWithMinMax(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --ratio=: {type: {kind: float, min: 0, max: 1.5}}
`)
	require.NoError(t, err)

	min, max := 0.0, 1.5
	assert.Equal(t, FloatType{Min: &min, Max: &max}, cmd.Flags["ratio"].Type)
}

func TestParseFlagEnumValuesAreSpecifiedOnTheFlagItself(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --color=: {description: a color, type: {kind: enum, values: [red, green, blue]}}
`)
	require.NoError(t, err)

	f := cmd.Flags["color"]
	assert.Equal(t, EnumType{Values: []string{"red", "green", "blue"}}, f.Type)
}

func TestParseFlagRejectsNonStringInValuesList(t *testing.T) {
	_, err := decodeYAML(`
name: mytool
flags:
  --color=: {type: {kind: enum, values: [red, 2, blue]}}
`)
	assert.Error(t, err)
}

func TestParseFlagRejectsUnknownTypeObjectKey(t *testing.T) {
	_, err := decodeYAML(`
name: mytool
flags:
  --count=: {type: {kind: int, maxx: 5}}
`)
	assert.Error(t, err)
}

func TestParseFlagRejectsUnknownTypeKind(t *testing.T) {
	_, err := decodeYAML(`
name: mytool
flags:
  --count=: {type: bogus}
`)
	assert.Error(t, err)
}

func TestParseFlagRejectsUnknownObjectKey(t *testing.T) {
	_, err := decodeYAML(`
name: mytool
flags:
  --count=: {description: consumes a count, tpye: int}
`)
	assert.Error(t, err)
}

func TestParseSubcommandCarriesItsOwnPositionals(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
commands:
  - name: sub
    description: a subcommand
    positionals:
      - name: arg
        required: true
`)
	require.NoError(t, err)
	require.Len(t, cmd.Commands, 1)

	sub := cmd.Commands[0]
	assert.Equal(t, "sub", sub.Name)
	require.Len(t, sub.Positionals, 1)
	assert.Equal(t, "arg", sub.Positionals[0].Name)
	assert.True(t, sub.Positionals[0].Required)
}

func TestCommandToCarapace(t *testing.T) {
	cmd, err := decodeYAML(`
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
`)
	require.NoError(t, err)

	converted := cmd.toCarapace()

	assert.Equal(t, "mytool", converted.Name)
	assert.Contains(t, converted.Flags, "count")
	require.Len(t, converted.Commands, 1)
	assert.Equal(t, "sub", converted.Commands[0].Name)

	var _ carapacespec.Command = converted //nolint:staticcheck // QF1011: type is the point, asserts toCarapace's return type
}

func TestToCarapaceAddsTypeHintToTypedFlagDescription(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --times=: {description: how many times, type: int}
`)
	require.NoError(t, err)

	converted := cmd.toCarapace()

	assert.Equal(t, "how many times (`int`)", converted.Flags["times"].Description)
}

func TestToCarapaceAddsIntMinMaxHintToFlagDescription(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --port=: {description: a port, type: {kind: int, min: 1, max: 65535}}
`)
	require.NoError(t, err)

	converted := cmd.toCarapace()

	assert.Equal(t, "a port (`int 1-65535`)", converted.Flags["port"].Description)
}

func TestToCarapaceAddsEnumValuesHintToFlagDescription(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --color=: {description: a color, type: {kind: enum, values: [red, green, blue]}}
`)
	require.NoError(t, err)

	converted := cmd.toCarapace()

	assert.Equal(t, "a color (`red|green|blue`)", converted.Flags["color"].Description)
}

// toCarapace derives completion.flag for an enum flag from its own Values, so Tab-completion
// and parse-time validation stay in sync without declaring the same list twice.
func TestToCarapaceDerivesCompletionFromEnumValues(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --color=: {type: {kind: enum, values: [red, green, blue]}}
`)
	require.NoError(t, err)

	converted := cmd.toCarapace()

	assert.Equal(t, []string{"red", "green", "blue"}, converted.Completion.Flag["color"])
}

// An explicitly declared completion (e.g. styled/described values) is never overridden by
// the derived one.
func TestToCarapaceLeavesExplicitCompletionForEnumFlagAlone(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --color=: {type: {kind: enum, values: [red, green, blue]}}
completion:
  flag:
    color: ["red\tlike a tomato"]
`)
	require.NoError(t, err)

	converted := cmd.toCarapace()

	assert.Equal(t, []string{"red\tlike a tomato"}, converted.Completion.Flag["color"])
}

func TestToCarapaceLeavesUntypedFlagDescriptionAlone(t *testing.T) {
	cmd, err := decodeYAML(`
name: mytool
flags:
  --name=: a name
`)
	require.NoError(t, err)

	converted := cmd.toCarapace()

	assert.Equal(t, "a name", converted.Flags["name"].Description)
}

// End-to-end: confirms the hint actually changes cobra's rendered usage text, not just
// the intermediate carapace-spec struct — pflag.UnquoteUsage is what does this, and it's
// easy to get its backtick convention subtly wrong without seeing the real output.
func TestParseYAMLUsageShowsDeclaredFlagType(t *testing.T) {
	cmd, _, err := ParseYAML(`
name: mytool
flags:
  --times=: {description: how many times, type: int}
`, "")
	require.NoError(t, err)

	assert.Contains(t, cmd.UsageString(), "--times int")
	assert.NotContains(t, cmd.UsageString(), "--times string")
}

// Flag.Validate/Positional.Validate's own behavior (nil Type, delegating to Type.Validate)
// is covered in type_test.go, alongside each concrete Type's own Validate tests.

func TestFlagValidateWithNilTypeAcceptsAnything(t *testing.T) {
	f := Flag{}
	assert.NoError(t, f.Validate("anything at all"))
}

func TestPositionalValidateWithNilTypeAcceptsAnything(t *testing.T) {
	p := Positional{}
	assert.NoError(t, p.Validate("anything at all"))
}
