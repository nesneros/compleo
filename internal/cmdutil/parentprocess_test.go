package cmdutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envHelperMode makes the compiled test binary act as a one-shot helper that prints
// ParentProcessName's result and exits, instead of running the test suite — the only way to
// observe it under a real shell parent, since go test's own parent is "go", not a shell.
const envHelperMode = "COMPLEO_PARENT_PROCESS_TEST_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(envHelperMode) != "" {
		name, err := ParentProcessName()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(name)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// detectUnder runs the compiled test binary as shellBin's child inside a command substitution
// — "$(...)", not a bare "shellBin -c exe" — because a bare "-c" may let the shell exec the
// helper in place of itself (skipping the fork that normally happens), which would report the
// wrong parent. A command substitution can't take that shortcut, since the substituting shell
// has to keep running afterward to use the captured output — the same reason eval "$(compleo
// ...)" always forks rather than replacing the calling shell outright.
func detectUnder(t *testing.T, shellBin string) string {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)

	cmd := exec.Command(shellBin, "-c", `echo "$(`+exe+`)"`)
	cmd.Env = append(os.Environ(), envHelperMode+"=1")
	out, err := cmd.Output()
	require.NoError(t, err, "helper output: %s", out)
	return strings.TrimSpace(string(out))
}

func TestParentProcessNameDetectsBash(t *testing.T) {
	assert.Equal(t, "bash", detectUnder(t, "bash"))
}

func TestParentProcessNameDetectsZsh(t *testing.T) {
	assert.Equal(t, "zsh", detectUnder(t, "zsh"))
}

func TestParentProcessNameDetectsSh(t *testing.T) {
	// "sh" is often a symlink (to dash, to bash in posix mode, ...) depending on the OS, so
	// assert it resolved to *something* real rather than pin one exact name.
	name := detectUnder(t, "sh")
	assert.NotEmpty(t, name)
	t.Logf("sh resolved to parent process name %q", name)
}
