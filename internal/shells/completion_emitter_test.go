package shells_test

import (
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/shells"
	_ "github.com/nesneros/compleo/internal/shells/bash"
	_ "github.com/nesneros/compleo/internal/shells/zsh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCompletionEmitterReadsEnv(t *testing.T) {
	t.Setenv(cmdutil.EnvCompletionShell, "bash")

	em := shells.GetCompletionEmitter(io.Discard, io.Discard)
	assert.Equal(t, "bash", em.Name())
}

func TestGetCompletionEmitterRequiresEnv(t *testing.T) {
	if os.Getenv("COMPLEO_GET_COMPLETION_EMITTER_EXIT") == "unset" {
		t.Setenv(cmdutil.EnvCompletionShell, "")
		shells.GetCompletionEmitter(io.Discard, io.Discard)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestGetCompletionEmitterRequiresEnv$")
	cmd.Env = append(os.Environ(), "COMPLEO_GET_COMPLETION_EMITTER_EXIT=unset")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Contains(t, string(out), cmdutil.EnvCompletionShell+" is not set. Is compleo activated in the current shell?")
}

func TestGetCompletionEmitterRejectsUnknownShell(t *testing.T) {
	if os.Getenv("COMPLEO_GET_COMPLETION_EMITTER_EXIT") == "unknown" {
		t.Setenv(cmdutil.EnvCompletionShell, "nosuch")
		shells.GetCompletionEmitter(io.Discard, io.Discard)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestGetCompletionEmitterRejectsUnknownShell$")
	cmd.Env = append(os.Environ(), "COMPLEO_GET_COMPLETION_EMITTER_EXIT=unknown")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Contains(t, string(out), "nosuch")
}
