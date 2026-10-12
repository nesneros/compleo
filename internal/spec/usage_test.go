package spec

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaceholderRequired(t *testing.T) {
	got := placeholder(Positional{Name: "file", Required: true})
	assert.Equal(t, "<file>", got)
}

func TestPlaceholderOptional(t *testing.T) {
	got := placeholder(Positional{Name: "file"})
	assert.Equal(t, "[file]", got)
}

func TestPlaceholderRequiredVariadic(t *testing.T) {
	got := placeholder(Positional{Name: "file", Required: true, Variadic: true})
	assert.Equal(t, "<file>...", got)
}

func TestPlaceholderOptionalVariadic(t *testing.T) {
	got := placeholder(Positional{Name: "file", Variadic: true})
	assert.Equal(t, "[file...]", got)
}

func TestSynopsisLineWithFlagsAndPositionals(t *testing.T) {
	cmd := &cobra.Command{Use: "mytool"}
	cmd.Flags().String("count", "", "how many times")

	got := synopsisLine(cmd, []Positional{
		{Name: "src", Required: true},
		{Name: "dst"},
	})

	assert.Equal(t, "mytool [flags] <src> [dst]", got)
}

func TestSynopsisLineWithoutFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "mytool"}

	got := synopsisLine(cmd, []Positional{{Name: "src", Required: true}})

	assert.Equal(t, "mytool <src>", got)
}

func buildCobra(t *testing.T, yaml, root string) *cobra.Command {
	t.Helper()

	cmd, _, err := ParseYAML(strings.NewReader(yaml))
	require.NoError(t, err)

	if root == "" {
		return cmd
	}
	found, _, err := cmd.Find([]string{root})
	require.NoError(t, err)
	return found
}

func TestApplySetsSynopsisLine(t *testing.T) {
	cmd := buildCobra(t, `
name: mytool
flags:
  --count=: how many times
positionals:
  - name: file
    required: true
`, "")

	assert.Equal(t, "mytool [flags] <file>", cmd.Use)
}

func TestApplyAddsArgumentsSectionToUsage(t *testing.T) {
	cmd := buildCobra(t, `
name: mytool
positionals:
  - name: file
    description: input file to process
    required: true
`, "")

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Usage())

	assert.Contains(t, buf.String(), "Arguments:\n  file   input file to process")
}

func TestApplyLeavesPositionallessCommandUsageAlone(t *testing.T) {
	cmd := buildCobra(t, `
name: mytool
`, "")

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Usage())

	assert.NotContains(t, buf.String(), "Arguments:")
}

// Subcommands matched by name, not position, since cobra's Commands() sorts
// alphabetically by default — "banana" declared before "apple" in the spec would
// otherwise pair with the wrong one.
func TestApplyMatchesSubcommandsByNameDespiteSorting(t *testing.T) {
	bananaCmd := buildCobra(t, `
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
`, "banana")

	assert.Equal(t, "banana <peel>", bananaCmd.Use)
}

// A child command with no positionals of its own must not inherit its parent's
// "Arguments:" section via cobra's UsageFunc-falls-back-to-parent behavior.
func TestApplyChildDoesNotInheritParentArguments(t *testing.T) {
	root := buildCobra(t, `
name: mytool
positionals:
  - name: parent-arg
    required: true
commands:
  - name: sub
`, "")

	sub, _, err := root.Find([]string{"sub"})
	require.NoError(t, err)

	var buf bytes.Buffer
	sub.SetOut(&buf)
	require.NoError(t, sub.Usage())

	assert.NotContains(t, buf.String(), "parent-arg")
}
