package zsh

import (
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionGeneratesZshSnippet(t *testing.T) {
	cmd, _, err := spec.ParseYAML(strings.NewReader(`
name: mytool
flags:
  --count=: how many times
`))
	require.NoError(t, err)

	snippet, err := Completion(cmd)
	require.NoError(t, err)

	assert.Contains(t, snippet, "#compdef mytool")
	assert.Contains(t, snippet, "compdef _mytool_completion mytool")
	assert.True(t, strings.HasSuffix(snippet, "\n"))
}

// Same spec must produce byte-identical output: write-only-if-changed callers depend on this.
func TestCompletionIsDeterministic(t *testing.T) {
	yaml := `
name: mytool
flags:
  --count=: how many times
  --name=: a name
  --verbose: be verbose
commands:
  - name: sub
    flags:
      --tag=: a tag
`
	cmd1, _, err := spec.ParseYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	snippet1, err := Completion(cmd1)
	require.NoError(t, err)

	cmd2, _, err := spec.ParseYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	snippet2, err := Completion(cmd2)
	require.NoError(t, err)

	assert.Equal(t, snippet1, snippet2)
}
