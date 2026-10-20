package bash

import (
	"bytes"
	"testing"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmitResultDeclaresAssociativeArray(t *testing.T) {
	var buf bytes.Buffer
	err := Emitter{}.EmitResult(&buf, parse.Result{
		Flags: map[string]parse.FlagValue{
			"name": {Kind: parse.KindString, String: "Jan"},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "declare -A compleo")
	assert.Contains(t, buf.String(), "compleo[name]='Jan'")
}

func TestWrappedCompletionInjectsSpecFile(t *testing.T) {
	cmd, _, err := spec.ParseYAML("name: greet\nflags:\n  --loud: shout\n", "")
	require.NoError(t, err)

	snippet, err := Emitter{}.WrappedCompletion(cmd, "greet", "/tmp/spec.yaml")
	require.NoError(t, err)
	assert.Contains(t, snippet, "_greet_completion() {")
	assert.Contains(t, snippet, "local -x "+cmdutil.EnvSpecFile+"="+Emitter{}.Quote("/tmp/spec.yaml"))
	assert.Contains(t, snippet, "complete -o noquote -F _greet_completion greet")
}

func TestEmitUpUsesBashBasename(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Emitter{}.EmitUp(&buf, "name: greet\n"))
	assert.Contains(t, buf.String(), `${0##*/}`)
}
