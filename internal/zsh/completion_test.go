package zsh

import (
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionGeneratesZshSnippet(t *testing.T) {
	cmd, sp, err := spec.ParseYAML(strings.NewReader(`
name: mytool
flags:
  --count=: how many times
`))
	require.NoError(t, err)

	snippet, err := Completion(cmd, sp)
	require.NoError(t, err)

	assert.Contains(t, snippet, "#compdef mytool")
	assert.Contains(t, snippet, "compdef _mytool_completion mytool")
}
