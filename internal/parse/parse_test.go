package parse

import (
	"strings"
	"testing"

	carapacespec "github.com/carapace-sh/carapace-spec"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildCobra parses a compleo spec and builds the real cobra tree the same way compleo's
// own CLI wiring would: spec.ParseYAML -> ToCarapace -> carapacespec's ToCobraE.
func buildCobra(t *testing.T, yaml string) (*cobra.Command, spec.Command) {
	t.Helper()

	sp, err := spec.ParseYAML(strings.NewReader(yaml))
	require.NoError(t, err)

	cmd, err := carapacespec.Command(sp.ToCarapace()).ToCobraE()
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

	result, err := Parse(cmd, sp, nil)
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

	_, err := Parse(cmd, sp, nil)
	assert.Error(t, err)
}

func TestParseOptionalPositionalOmitted(t *testing.T) {
	cmd, sp := buildCobra(t, `
name: mytool
positionals:
  - name: file
`)

	result, err := Parse(cmd, sp, nil)
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
