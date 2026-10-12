package zsh

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmitScalarStringFlag(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"name": {Kind: parse.KindString, Changed: true, String: "Jan"},
	}}

	assert.Contains(t, Emit(result), "compleo[name]='Jan'")
}

func TestEmitBoolFlagTrue(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"loud": {Kind: parse.KindBool, Bool: true},
	}}

	assert.Contains(t, Emit(result), "compleo[loud]='true'")
}

func TestEmitBoolFlagFalse(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"loud": {Kind: parse.KindBool, Bool: false},
	}}

	assert.Contains(t, Emit(result), "compleo[loud]='false'")
}

func TestEmitCountFlag(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"verbose": {Kind: parse.KindCount, Count: 3},
	}}

	assert.Contains(t, Emit(result), "compleo[verbose]='3'")
}

func TestEmitRepeatableFlagAsSeparateArray(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"tag": {Kind: parse.KindStrings, Strings: []string{"a", "b"}},
	}}

	snippet := Emit(result)
	assert.Contains(t, snippet, "typeset -a compleo_tag=()")
	assert.Contains(t, snippet, "compleo_tag=('a' 'b')")
	assert.NotContains(t, snippet, "compleo[tag]")
}

func TestEmitRepeatableFlagNeverPassedIsEmptyArray(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"tag": {Kind: parse.KindStrings, Strings: nil},
	}}

	snippet := Emit(result)
	assert.Contains(t, snippet, "typeset -a compleo_tag=()")
	assert.Contains(t, snippet, "compleo_tag=()")
}

func TestEmitScalarPositional(t *testing.T) {
	result := parse.Result{Positionals: map[string]parse.PositionalValue{
		"file": {Values: []string{"input.txt"}},
	}}

	assert.Contains(t, Emit(result), "compleo[file]='input.txt'")
}

func TestEmitOmittedOptionalPositionalIsEmptyString(t *testing.T) {
	result := parse.Result{Positionals: map[string]parse.PositionalValue{
		"file": {},
	}}

	assert.Contains(t, Emit(result), "compleo[file]=''")
}

func TestEmitVariadicPositionalAsSeparateArray(t *testing.T) {
	result := parse.Result{Positionals: map[string]parse.PositionalValue{
		"files": {Values: []string{"a.txt", "b.txt"}, Variadic: true},
	}}

	snippet := Emit(result)
	assert.Contains(t, snippet, "typeset -a compleo_files=()")
	assert.Contains(t, snippet, "compleo_files=('a.txt' 'b.txt')")
	assert.NotContains(t, snippet, "compleo[files]")
}

func TestEmitSanitizesHyphenInArrayVarName(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"dry-run-tag": {Kind: parse.KindStrings, Strings: []string{"x"}},
	}}

	snippet := Emit(result)
	assert.Contains(t, snippet, "typeset -a compleo_dry_run_tag=()")
	assert.Contains(t, snippet, "compleo_dry_run_tag=('x')")
}

func TestEmitQuotesValuesWithSpecialCharacters(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"name": {Kind: parse.KindString, String: "it's $dangerous"},
	}}

	assert.Contains(t, Emit(result), "compleo[name]="+Quote("it's $dangerous"))
}

func TestEmitDeclaresMainArrayEvenWhenEmpty(t *testing.T) {
	assert.Equal(t, "typeset -A compleo=()", Emit(parse.Result{}))
}

// zshEval sources snippet then prints each expr, proving the emitted code is valid zsh.
func zshEval(t *testing.T, snippet string, exprs ...string) []string {
	t.Helper()

	script := snippet + "\n"
	for _, e := range exprs {
		script += "print -r -- " + e + "\n"
	}

	cmd := exec.Command("zsh", "-fc", script)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "zsh output: %s", out)

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	return lines
}

func TestEmitEndToEndScalarsAndArraysUnderRealZsh(t *testing.T) {
	cmd, sp, err := spec.ParseYAML(strings.NewReader(`
name: greet
flags:
  --loud: {description: shout}
  --times=: {description: how many times, default: "1"}
  --tag*=: a tag
flagtypes:
  times: int
positionals:
  - name: name
    required: true
  - name: extra
    variadic: true
`))
	require.NoError(t, err)

	result, err := parse.Parse(cmd, sp, []string{
		"--loud", "--times", "3", "--tag", "a", "--tag", "b",
		"world", "one", "two",
	})
	require.NoError(t, err)

	snippet := Emit(result)

	lines := zshEval(t, snippet,
		`$compleo[loud]`,
		`$compleo[times]`,
		`$compleo[name]`,
		`${compleo_tag[@]}`,
		`${compleo_extra[@]}`,
	)
	assert.Equal(t, []string{
		"true",
		"3",
		"world",
		"a b",
		"one two",
	}, lines)
}

func TestEmitHelpPrintsTextAndExitsZero(t *testing.T) {
	snippet := EmitHelp("Usage:\n  mytool [flags]")

	cmd := exec.Command("zsh", "-fc", snippet)
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "Usage:\n  mytool [flags]\n", string(out))
}

func TestEmitHelpQuotesSpecialCharacters(t *testing.T) {
	snippet := EmitHelp("it's $dangerous `too`")

	cmd := exec.Command("zsh", "-fc", snippet)
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "it's $dangerous `too`\n", string(out))
}

func TestEmitErrorPrintsNameAndErrToStderrAndExitsTwo(t *testing.T) {
	snippet := EmitError("mytool", errors.New("bad flag"))

	cmd := exec.Command("zsh", "-fc", snippet)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, runErr, &exitErr)
	assert.Equal(t, 2, exitErr.ExitCode())
	assert.Equal(t, "mytool: bad flag\n", stderr.String())
}

func TestEmitErrorQuotesSpecialCharacters(t *testing.T) {
	snippet := EmitError("it's", errors.New("$dangerous `too`"))

	cmd := exec.Command("zsh", "-fc", snippet)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, runErr, &exitErr)
	assert.Equal(t, "it's: $dangerous `too`\n", stderr.String())
}

func TestEmitHelpStopsScriptEvenInsideAFunction(t *testing.T) {
	cmd := exec.Command("zsh", "-fc", `
function main {
  eval "$SNIPPET"
  print -r -- "unreachable"
}
main
print -r -- "outer after main"
`)
	cmd.Env = append(os.Environ(), "SNIPPET="+EmitHelp("help text"))

	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "help text\n", string(out))
}
