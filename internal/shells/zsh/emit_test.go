package zsh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func emitResult(t *testing.T, result parse.Result) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitResult(result))
	return buf.String()
}

func emitHelp(t *testing.T, text string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitHelp(text))
	return buf.String()
}

func emitError(t *testing.T, name string, err error) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitError(name, err))
	return buf.String()
}

func emitGenerated(t *testing.T, name string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitGenerated(Em.ActivationHint(name), nil))
	return buf.String()
}

func emitActivate(t *testing.T, dir string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitActivate(dir))
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

	assert.Contains(t, emitResult(t, result), "compleo[name]="+Em.Quote("it's $dangerous"))
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
	cmd, sp, err := spec.ParseYAML(`
name: greet
flags:
  --loud: {description: shout}
  --times=: {description: how many times, default: "1", type: int}
  --tag*=: a tag
positionals:
  - name: name
    required: true
  - name: extra
    variadic: true
`, "")
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
	snippet := emitGenerated(t, "greet")

	cmd := exec.Command("zsh", "-fc", snippet)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Contains(t, stderr.String(), "To activate completion for greet in this shell, run:")
	assert.Contains(t, stderr.String(), "autoload -Uz _greet && compdef _greet greet")
	assert.NotContains(t, stderr.String(), "warning:")
}

func TestEmitGeneratedPrintsOneWarningForMissingMacros(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitGenerated(Em.ActivationHint("greet"), []string{"carapace", "missingb"}))

	cmd := exec.Command("zsh", "-fc", buf.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Equal(t, 1, strings.Count(stderr.String(), "warning:"))
	assert.Contains(t, stderr.String(), "warning: not on PATH, so these completion macros will not work: $carapace.*, $missingb.*")
	assert.Contains(t, stderr.String(), "To activate completion for greet in this shell, run:")
}

// Uses real generated content and calls the autoloaded name, not just checking
// _comps[greet] — a hint that only sets _comps correctly without a real autoload target
// still fails at the actual Tab press with "command not found".
func TestEmitGeneratedActivationHintActuallyWorksFromColdStart(t *testing.T) {
	cmd, _, err := spec.ParseYAML("name: greet\nflags:\n  --loud: shout\n", "")
	require.NoError(t, err)
	content, err := Em.WrappedCompletion(cmd, "greet", "name: greet\nflags:\n  --loud: shout\n")
	require.NoError(t, err)

	fpathDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fpathDir, "_greet"), []byte(content), 0o644))

	activate := "autoload -Uz _greet && compdef _greet greet"
	require.Contains(t, emitGenerated(t, "greet"), activate,
		"test exercises the exact instruction EmitGenerated prints, not a hand-derived one")

	script := fmt.Sprintf(`
fpath+=(%[1]s)
autoload -Uz compinit
# -u: non-interactive compinit aborts and unfunctions compdef on an insecure dir (Ubuntu CI).
compinit -u -d %[2]s
%[3]s
_greet
print -r -- "${_comps[greet]}"
`, Em.Quote(fpathDir), Em.Quote(filepath.Join(t.TempDir(), "zcompdump")), activate)

	out, err := exec.Command("zsh", "-fc", script).Output()
	require.NoError(t, err)
	assert.Equal(t, "_greet_completion\n", string(out))
}

func TestEmitUpGeneratesReinvocationWithNameAndArgsFilledIn(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitUp("flags:\n  --loud: shout\n"))

	snippet := buf.String()
	assert.Contains(t, snippet, `compleo parse --shell=zsh --name="${0:t}" -- "$@"`)
	assert.Contains(t, snippet, "<<<")
}

func TestEmitUpIsSyntacticallyValidZsh(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitUp("flags:\n  --loud: shout\n"))

	require.NoError(t, exec.Command("zsh", "-n", "-c", buf.String()).Run())
}

// Stubs "compleo" as a fake shell function to capture what the generated code's inner
// invocation receives — confirms name/args reach it, and the spec round-trips byte for byte
// through the <<< here-string.
func TestEmitUpGeneratedCodeInvokesParseWithNameArgsAndSpec(t *testing.T) {
	original := "description: it's $dangerous `here`\nflags:\n  --loud: shout\n"
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitUp(original))

	captureFile := filepath.Join(t.TempDir(), "captured")
	script := fmt.Sprintf(`
compleo() {
  { print -r -- "ARGS:$*"; print -r -- "SPEC-START"; cat; print -rn -- "SPEC-END"; } > %s
  print -r -- "typeset -A compleo=()"
}
%s
`, Em.Quote(captureFile), buf.String())

	_, err := exec.Command("zsh", "-fc", script, "scriptname", "world", "--loud").Output()
	require.NoError(t, err)

	captured, err := os.ReadFile(captureFile)
	require.NoError(t, err)
	got := string(captured)
	assert.Contains(t, got, `ARGS:parse --shell=zsh --name=scriptname -- world --loud`)
	// <<< always appends its own trailing newline, on top of original's own — harmless to
	// YAML (confirmed: a real compleo parses this fine), but means one extra "\n" here.
	assert.Contains(t, got, "SPEC-START\n"+original+"\nSPEC-END")
}

func TestEmitActivatePrintsFpathWiring(t *testing.T) {
	assert.Contains(t, emitActivate(t, "/some/dir"), "fpath+=('/some/dir')")
}

func TestEmitActivateExportsCompletionShell(t *testing.T) {
	assert.Contains(t, emitActivate(t, "/some/dir"), "export "+cmdutil.EnvCompletionShell+"="+Em.Quote(Em.Name()))
}

func TestEmitActivateDefinesCompleoFunction(t *testing.T) {
	assert.Contains(t, emitActivate(t, "/some/dir"), "compleo() {\n  command compleo \"$@\"\n}\n")
}

// The function must forward via "command", or calling "compleo" inside it would recurse into
// itself instead of reaching the real binary.
func TestEmitActivateCompleoFunctionWrapsTheRealBinary(t *testing.T) {
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "compleo"), []byte("#!/bin/sh\necho \"wrapped: $*\"\n"), 0o755))

	completionDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(completionDir, "_compleo"), []byte("#compdef compleo\n"), 0o644))

	script := fmt.Sprintf(`
path=(%[1]s $path)
%[2]s
compleo foo bar
`, Em.Quote(binDir), emitActivate(t, completionDir))

	out, err := exec.Command("zsh", "-fc", script).Output()
	require.NoError(t, err)
	assert.Equal(t, "wrapped: foo bar\n", string(out))
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
# -u: non-interactive compinit aborts and unfunctions compdef on an insecure dir (Ubuntu CI).
compinit -u -d %[1]s
%[2]s
print -r -- "compleo: ${_comps[compleo]:-<none>}"
print -r -- "greet: ${_comps[greet]:-<none>}"
`, Em.Quote(filepath.Join(t.TempDir(), "zcompdump")), emitActivate(t, dir))

	out, err := exec.Command("zsh", "-fc", script).Output()
	require.NoError(t, err)
	assert.Equal(t, "compleo: _compleo\ngreet: _greet\n", string(out))
}
