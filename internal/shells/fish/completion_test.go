package fish

import (
	"bytes"
	"io"
	"testing"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionFileNameUsesFishSuffix(t *testing.T) {
	assert.Equal(t, "greet.fish", Em.CompletionFileName("greet"))
}

func TestWrappedCompletionInjectsSpecFile(t *testing.T) {
	cmd, _, err := spec.ParseYAML("name: greet\nflags:\n  --loud: shout\n", "")
	require.NoError(t, err)

	snippet, err := Em.WrappedCompletion(cmd, "greet", "/tmp/spec.yaml")
	require.NoError(t, err)
	assert.Contains(t, snippet, "function _greet_completion\n")
	assert.Contains(t, snippet, "set -lx "+cmdutil.EnvSpecFile+" "+Em.Quote("/tmp/spec.yaml"))
	assert.Contains(t, snippet, "complete -c 'greet'")
	assert.Contains(t, snippet, "_carapace fish")
}

func TestEmitActivateAddsDirToFishCompletePath(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitActivate("/some/dir"))
	out := buf.String()
	assert.Contains(t, out, "set -gx "+cmdutil.EnvCompletionShell+" "+Em.Quote("fish"))
	assert.Contains(t, out, "set -l __compleo_dir '/some/dir'")
	assert.Contains(t, out, "set -gp fish_complete_path $__compleo_dir")
	assert.Contains(t, out, "function compleo\n  command compleo $argv\nend")
}

func TestActivationHintSourcesTheFishFile(t *testing.T) {
	t.Setenv(cmdutil.EnvCompletionDir, "/completions")
	assert.Equal(t, "source "+Em.Quote("/completions/greet.fish"), Em.ActivationHint("greet"))
}

func TestEvalEmitterDoesNotSeeFish(t *testing.T) {
	assert.Nil(t, shells.EvalEmitterForShell("fish", io.Discard, io.Discard))
	em, err := shells.CompletionEmitterForShell("fish", io.Discard, io.Discard)
	require.NoError(t, err)
	assert.Equal(t, "fish", em.Name())
}
