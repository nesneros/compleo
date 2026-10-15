package parse

import (
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/spec"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildCobra(t *testing.T, yaml string) (*cobra.Command, spec.Command) {
	t.Helper()

	cmd, sp, err := spec.ParseYAML(strings.NewReader(yaml), "")
	require.NoError(t, err)

	return cmd, sp
}

func TestParseScalarFlag(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
flags:
  --name=: a name
`)

	result, err := Parse(cmd, sp, []string{"--name", "alice"})
	require.NoError(t, err)

	f := result.Flags["name"]
	assert.Equal(t, KindString, f.Kind)
	assert.True(t, f.Changed)
	assert.Equal(t, "alice", f.String)
}

func TestParseScalarFlagDefaultNotChanged(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
flags:
  --name=: {description: a name, default: bob}
`)

	result, err := Parse(cmd, sp, []string{})
	require.NoError(t, err)

	f := result.Flags["name"]
	assert.False(t, f.Changed)
	assert.Equal(t, "bob", f.String)
}

func TestParseRepeatableFlag(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
flags:
  --tag*=: a tag
`)

	result, err := Parse(cmd, sp, []string{"--tag", "a", "--tag", "b"})
	require.NoError(t, err)

	f := result.Flags["tag"]
	assert.Equal(t, KindStrings, f.Kind)
	assert.Equal(t, []string{"a", "b"}, f.Strings)
}

func TestParseBoolFlag(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
flags:
  --verbose: verbose output
`)

	result, err := Parse(cmd, sp, []string{"--verbose"})
	require.NoError(t, err)

	f := result.Flags["verbose"]
	assert.Equal(t, KindBool, f.Kind)
	assert.True(t, f.Changed)
	assert.True(t, f.Bool)
}

func TestParseCountFlag(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
flags:
  -v, --verbose*: verbosity
`)

	result, err := Parse(cmd, sp, []string{"-vvv"})
	require.NoError(t, err)

	f := result.Flags["verbose"]
	assert.Equal(t, KindCount, f.Kind)
	assert.Equal(t, 3, f.Count)
}

func TestParseFlagWithIntTypeAcceptsValidValue(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
flags:
  --times=: {description: how many times, type: int}
`)

	result, err := Parse(cmd, sp, []string{"--times", "3"})
	require.NoError(t, err)
	assert.Equal(t, "3", result.Flags["times"].String)
}

func TestParseFlagWithIntTypeRejectsNonNumericValue(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
flags:
  --times=: {description: how many times, type: int}
`)

	_, err := Parse(cmd, sp, []string{"--times", "soon"})
	assert.Error(t, err)
}

// A persistent flag's type must still be validated on a subcommand — cobra merges an
// ancestor's persistent flags into a subcommand's own cmd.Flags(), so the type lookup has
// to follow the same inheritance, not just look at the matched (sub)command's own Flags.
func TestParsePersistentFlagWithIntTypeIsValidatedOnSubcommand(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
persistentflags:
  --times=: {description: how many times, type: int}
commands:
  - name: sub
`)

	_, err := Parse(cmd, sp, []string{"sub", "--times", "soon"})
	assert.Error(t, err)
}

func TestParsePositionalWithIntTypeRejectsNonNumericValue(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
positionals:
  - name: count
    type: int
`)

	_, err := Parse(cmd, sp, []string{"abc"})
	assert.Error(t, err)
}

func TestParseRequiredPositionalPresent(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
positionals:
  - name: file
    required: true
`)

	result, err := Parse(cmd, sp, []string{"input.txt"})
	require.NoError(t, err)

	assert.Equal(t, []string{"input.txt"}, result.Positionals["file"].Values)
}

func TestParseRequiredPositionalMissingFails(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
positionals:
  - name: file
    required: true
`)

	_, err := Parse(cmd, sp, []string{})
	assert.Error(t, err)
}

func TestParseOptionalPositionalOmitted(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
positionals:
  - name: file
`)

	result, err := Parse(cmd, sp, []string{})
	require.NoError(t, err)

	assert.Empty(t, result.Positionals["file"].Values)
}

func TestParseVariadicPositionalCapturesMultiple(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
positionals:
  - name: first
    required: true
  - name: rest
    variadic: true
`)

	result, err := Parse(cmd, sp, []string{"a", "b", "c"})
	require.NoError(t, err)

	assert.Equal(t, []string{"a"}, result.Positionals["first"].Values)
	assert.Equal(t, []string{"b", "c"}, result.Positionals["rest"].Values)
}

func TestParseTooManyPositionalsFails(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
positionals:
  - name: file
    required: true
`)

	_, err := Parse(cmd, sp, []string{"a", "b"})
	assert.Error(t, err)
}

// Subcommands matched by name, not position, since cobra's Commands() sorts
// alphabetically by default.
func TestParseMatchesSubcommandByNameDespiteSorting(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
commands:
  - name: banana
    positionals:
      - name: peel
        required: true
  - name: apple
    positionals:
      - name: core
        required: true
`)

	result, err := Parse(cmd, sp, []string{"banana", "yellow"})
	require.NoError(t, err)

	assert.Equal(t, []string{"mytool", "banana"}, result.Path)
	assert.Equal(t, []string{"yellow"}, result.Positionals["peel"].Values)
	assert.NotContains(t, result.Positionals, "core")
}

// A nil argv (as opposed to an empty slice) makes cobra's Execute fall back to the real
// process os.Args, since cobra can't tell "explicitly no args" from "never set" (SetArgs
// wasn't called) — so this is a caller bug, not something to normalize silently.
func TestParsePanicsOnNilArgv(t *testing.T) {
	cmd, sp := buildCobra(t, `name: mytool`)

	assert.Panics(t, func() {
		_, _ = Parse(cmd, sp, nil)
	})
}

// Regression test for the carapace-sh/carapace-pflag replace in go.mod: without it, a
// shorthand-only flag (no --longhand) silently fails to register at all — present in the
// spec, absent from --help, and unknown if actually typed — rather than erroring up front.
func TestParseShorthandOnlyFlagParses(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
flags:
  -v: verbose output
`)

	result, err := Parse(cmd, sp, []string{"-v"})
	require.NoError(t, err)
	assert.True(t, result.Flags["v"].Bool)
}

// Same regression, for a nargs-consuming flag (no longhand required here, but nargs is
// equally fork-only — without it, --pair silently never registers either).
func TestParseNargsFlagConsumesMultipleArgs(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
flags:
  --pair=: {description: two values, nargs: 2}
`)

	result, err := Parse(cmd, sp, []string{"--pair", "a", "b"})
	require.NoError(t, err)
	assert.Equal(t, "a,b", result.Flags["pair"].String)
}
