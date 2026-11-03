package nu

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

func TestCompletionFileNameUsesNuSuffix(t *testing.T) {
	assert.Equal(t, "greet.nu", Em.CompletionFileName("greet"))
}

func TestQuoteEscapesNushellDoubleQuote(t *testing.T) {
	assert.Equal(t, `"it\$s"`, Em.Quote(`it$s`))
	assert.Equal(t, `"say \"hi\""`, Em.Quote(`say "hi"`))
}

func TestWrappedCompletionRegistersSpecFile(t *testing.T) {
	cmd, _, err := spec.ParseYAML("name: greet\nflags:\n  --loud: shout\n", "")
	require.NoError(t, err)

	snippet, err := Em.WrappedCompletion(cmd, "greet", "/tmp/spec.yaml")
	require.NoError(t, err)
	assert.Contains(t, snippet, "with-env { "+cmdutil.EnvSpecFile+": "+Em.Quote("/tmp/spec.yaml")+" }")
	assert.Contains(t, snippet, "_carapace nushell")
	assert.Contains(t, snippet, "upsert "+Em.Quote("greet")+" { $completer }")
	assert.NotContains(t, snippet, "_carapace nu ")
}

func TestEmitActivateInstallsDispatcher(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitActivate("/some/dir"))
	out := buf.String()
	assert.Contains(t, out, "$env."+cmdutil.EnvCompletionShell+" = "+Em.Quote("nu"))
	assert.Contains(t, out, "$env._COMPLEO_COMPLETION_DIR = "+Em.Quote("/some/dir"))
	assert.Contains(t, out, "_COMPLEO_NU_DISPATCHER")
	assert.Contains(t, out, "_carapace nushell")
	assert.Contains(t, out, "def --env compleo [...args] { ^compleo ...$args }")
}

func TestActivationHintSourcesTheNuFile(t *testing.T) {
	t.Setenv(cmdutil.EnvCompletionDir, "/completions")
	assert.Equal(t, "source "+Em.Quote("/completions/greet.nu"), Em.ActivationHint("greet"))
}

func TestEvalEmitterSeesNu(t *testing.T) {
	evalEm := shells.EvalEmitterForShell("nu", io.Discard, io.Discard)
	require.NotNil(t, evalEm)
	assert.Equal(t, "nu", evalEm.Name())
	assert.Nil(t, shells.EvalEmitterForShell("nushell", io.Discard, io.Discard))
	completionEm, err := shells.CompletionEmitterForShell("nu", io.Discard, io.Discard)
	require.NoError(t, err)
	assert.Equal(t, "nu", completionEm.Name())
	completionEm, err = shells.CompletionEmitterForShell("nushell", io.Discard, io.Discard)
	require.NoError(t, err)
	assert.Equal(t, "nu", completionEm.Name())
}
