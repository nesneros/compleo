package zsh

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func emitResult(t *testing.T, result parse.Result) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, Emit(&buf, result))
	return buf.String()
}

func emitHelp(t *testing.T, text string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, EmitHelp(&buf, text))
	return buf.String()
}

func emitError(t *testing.T, name string, err error) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, EmitError(&buf, name, err))
	return buf.String()
}

func emitGenerated(t *testing.T, name, path string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, EmitGenerated(&buf, name, path))
	return buf.String()
}

func emitActivate(t *testing.T, dir string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, EmitActivate(&buf, dir))
	return buf.String()
}

func TestEmitScalarStringFlag(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"name": {Kind: parse.KindString, Changed: true, String: "Jan"},
	}}

	assert.Contains(t, emitResult(t, result), "compleo[name]='Jan'")
}

func TestEmitBoolFlagTrue(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"loud": {Kind: parse.KindBool, Bool: true},
	}}

	assert.Contains(t, emitResult(t, result), "compleo[loud]='true'")
}

func TestEmitBoolFlagFalse(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"loud": {Kind: parse.KindBool, Bool: false},
	}}

	assert.Contains(t, emitResult(t, result), "compleo[loud]='false'")
}

func TestEmitCountFlag(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"verbose": {Kind: parse.KindCount, Count: 3},
	}}

	assert.Contains(t, emitResult(t, result), "compleo[verbose]='3'")
}

func TestEmitRepeatableFlagAsSeparateArray(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"tag": {Kind: parse.KindStrings, Strings: []string{"a", "b"}},
	}}

	snippet := emitResult(t, result)
	assert.Contains(t, snippet, "typeset -a compleo_tag=()")
	assert.Contains(t, snippet, "compleo_tag=('a' 'b')")
	assert.NotContains(t, snippet, "compleo[tag]")
}

func TestEmitRepeatableFlagNeverPassedIsEmptyArray(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"tag": {Kind: parse.KindStrings, Strings: nil},
	}}

	snippet := emitResult(t, result)
	assert.Contains(t, snippet, "typeset -a compleo_tag=()")
	assert.Contains(t, snippet, "compleo_tag=()")
}

func TestEmitScalarPositional(t *testing.T) {
	result := parse.Result{Positionals: map[string]parse.PositionalValue{
		"file": {Values: []string{"input.txt"}},
	}}

	assert.Contains(t, emitResult(t, result), "compleo[file]='input.txt'")
}

func TestEmitOmittedOptionalPositionalIsEmptyString(t *testing.T) {
	result := parse.Result{Positionals: map[string]parse.PositionalValue{
		"file": {},
	}}

	assert.Contains(t, emitResult(t, result), "compleo[file]=''")
}

func TestEmitVariadicPositionalAsSeparateArray(t *testing.T) {
	result := parse.Result{Positionals: map[string]parse.PositionalValue{
		"files": {Values: []string{"a.txt", "b.txt"}, Variadic: true},
	}}

	snippet := emitResult(t, result)
	assert.Contains(t, snippet, "typeset -a compleo_files=()")
	assert.Contains(t, snippet, "compleo_files=('a.txt' 'b.txt')")
	assert.NotContains(t, snippet, "compleo[files]")
}

func TestEmitSanitizesHyphenInArrayVarName(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"dry-run-tag": {Kind: parse.KindStrings, Strings: []string{"x"}},
	}}

	snippet := emitResult(t, result)
	assert.Contains(t, snippet, "typeset -a compleo_dry_run_tag=()")
	assert.Contains(t, snippet, "compleo_dry_run_tag=('x')")
}

func TestEmitQuotesValuesWithSpecialCharacters(t *testing.T) {
	result := parse.Result{Flags: map[string]parse.FlagValue{
		"name": {Kind: parse.KindString, String: "it's $dangerous"},
	}}

	assert.Contains(t, emitResult(t, result), "compleo[name]="+Quote("it's $dangerous"))
}

func TestEmitDeclaresMainArrayEvenWhenEmpty(t *testing.T) {
	assert.Equal(t, "typeset -A compleo=()\n", emitResult(t, parse.Result{}))
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
`), "")
	require.NoError(t, err)

	result, err := parse.Parse(cmd, sp, []string{
		"--loud", "--times", "3", "--tag", "a", "--tag", "b",
		"world", "one", "two",
	})
	require.NoError(t, err)

	snippet := emitResult(t, result)

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
	snippet := emitHelp(t, "Usage:\n  mytool [flags]")

	cmd := exec.Command("zsh", "-fc", snippet)
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "Usage:\n  mytool [flags]\n", string(out))
}

func TestEmitHelpQuotesSpecialCharacters(t *testing.T) {
	snippet := emitHelp(t, "it's $dangerous `too`")

	cmd := exec.Command("zsh", "-fc", snippet)
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "it's $dangerous `too`\n", string(out))
}

func TestEmitErrorPrintsNameAndErrToStderrAndExitsTwo(t *testing.T) {
	snippet := emitError(t, "mytool", errors.New("bad flag"))

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
	snippet := emitError(t, "it's", errors.New("$dangerous `too`"))

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
	cmd.Env = append(os.Environ(), "SNIPPET="+emitHelp(t, "help text"))

	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "help text\n", string(out))
}

func TestEmitGeneratedPrintsConfirmationToStderrAndExitsZero(t *testing.T) {
	snippet := emitGenerated(t, "greet", "/some/dir/_greet")

	cmd := exec.Command("zsh", "-fc", snippet)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Contains(t, stderr.String(), "greet: generated completion at /some/dir/_greet")
	assert.Contains(t, stderr.String(), "autoload -Uz _greet && compdef _greet greet")
}

// Regression test: an earlier hint ("compdef _greet_completion greet" directly) looked right
// and even set _comps[greet] correctly, but _greet_completion was never defined or
// autoloaded, so real completion failed with "command not found". Uses real generated
// content and actually calls the autoloaded name, not just checking what _comps[greet] holds.
func TestEmitGeneratedActivationHintActuallyWorksFromColdStart(t *testing.T) {
	cmd, _, err := spec.ParseYAML(strings.NewReader("name: greet\nflags:\n  --loud: shout\n"), "")
	require.NoError(t, err)
	content, err := WrappedCompletion(cmd, "greet", "name: greet\nflags:\n  --loud: shout\n")
	require.NoError(t, err)

	fpathDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fpathDir, "_greet"), []byte(content), 0o644))

	activate := "autoload -Uz _greet && compdef _greet greet"
	require.Contains(t, emitGenerated(t, "greet", "/some/path"), activate,
		"test exercises the exact instruction EmitGenerated prints, not a hand-derived one")

	script := fmt.Sprintf(`
fpath+=(%[1]s)
autoload -Uz compinit
compinit -d %[2]s
%[3]s
_greet
print -r -- "${_comps[greet]}"
`, Quote(fpathDir), Quote(filepath.Join(t.TempDir(), "zcompdump")), activate)

	out, err := exec.Command("zsh", "-fc", script).Output()
	require.NoError(t, err)
	assert.Equal(t, "_greet_completion\n", string(out))
}

func TestEmitActivatePrintsFpathWiring(t *testing.T) {
	assert.Contains(t, emitActivate(t, "/some/dir"), "fpath+=('/some/dir')")
}

// Before compinit runs, compdef doesn't even exist (confirmed empirically) — activate's
// guarded autoload+compdef block must be a true no-op there, not an error, since activate's
// documented use is to run before compinit.
func TestEmitActivateIsNoopBeforeCompinit(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "_compleo"), []byte("#compdef compleo\n"), 0o644))

	cmd := exec.Command("zsh", "-fc", emitActivate(t, dir))
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "output: %s", out)
}

// Regression test for the scenario activate's guard exists for: compinit already ran earlier
// this session (so fpath additions from this activate wouldn't otherwise be noticed) —
// confirms the guarded block actually registers every file in dir, not just a no-op.
func TestEmitActivateRegistersEveryFileWhenCompdefAlreadyAvailable(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "_compleo"), []byte("#compdef compleo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "_greet"), []byte("#compdef greet\n"), 0o644))

	script := fmt.Sprintf(`
autoload -Uz compinit
compinit -d %[1]s
%[2]s
print -r -- "compleo: ${_comps[compleo]:-<none>}"
print -r -- "greet: ${_comps[greet]:-<none>}"
`, Quote(filepath.Join(t.TempDir(), "zcompdump")), emitActivate(t, dir))

	out, err := exec.Command("zsh", "-fc", script).Output()
	require.NoError(t, err)
	assert.Equal(t, "compleo: _compleo\ngreet: _greet\n", string(out))
}
