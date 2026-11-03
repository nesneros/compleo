package nu

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/parse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmitResultAssignsEnv(t *testing.T) {
	nuPath, err := exec.LookPath("nu")
	if err != nil {
		t.Skip("nu is not installed")
	}

	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitResult(parse.Result{
		Flags: map[string]parse.FlagValue{
			"loud":  {Kind: parse.KindBool, Bool: true},
			"times": {Kind: parse.KindString, String: "2"},
		},
		Positionals: map[string]parse.PositionalValue{
			"name": {Values: []string{"world"}},
		},
	}, "compleo_"))

	script := filepath.Join(t.TempDir(), "result.nu")
	body := "let result = (" + Em.Quote(strings.TrimRight(buf.String(), "\n")) + " | from nuon)\nprint $result.compleo_name\nprint $result.compleo_loud\nprint $result.compleo_times\n"
	require.NoError(t, os.WriteFile(script, []byte(body), 0o644))

	out, err := exec.Command(nuPath, script).Output()
	require.NoError(t, err)
	assert.Equal(t, "world\ntrue\n2\n", string(out))
}

func TestEmitUpNamesNuParse(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitUp("description: hi\n"))
	out := buf.String()
	assert.Contains(t, out, "^compleo parse --shell=nu --name $name -- ...$args")
	assert.Contains(t, out, Em.Quote("description: hi\n"))
	assert.NotContains(t, out, "source ")
}
