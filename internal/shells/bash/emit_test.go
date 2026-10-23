package bash

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
	require.NoError(t, New(&buf, io.Discard).EmitResult(result, "compleo_"))
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

// bashCmd runs script under bash with no startup files. Extra args are $0, $1, ...
func bashCmd(script string, args ...string) *exec.Cmd {
	argv := append([]string{"--noprofile", "--norc", "-c", script}, args...)
	return exec.Command("bash", argv...)
}

func TestEmitResultAssignsPrefixedVariables(t *testing.T) {
	var buf bytes.Buffer
	err := New(&buf, io.Discard).EmitResult(parse.Result{
		Flags: map[string]parse.FlagValue{
			"name": {Kind: parse.KindString, String: "world"},
		},
	}, "compleo_")
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "declare -A")
	assert.Contains(t, buf.String(), "compleo_name='world'")
}

func TestEmitResultUsesTheGivenPrefix(t *testing.T) {
	var buf bytes.Buffer
	err := New(&buf, io.Discard).EmitResult(parse.Result{
		Flags: map[string]parse.FlagValue{
			"name": {Kind: parse.KindString, String: "world"},
		},
	}, "tool_")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "tool_name='world'")
}

// A prefix with no trailing separator joins straight onto the name — the caller's choice,
// not something EmitResult imposes.
func TestEmitResultPrefixWithNoSeparatorJoinsDirectly(t *testing.T) {
	var buf bytes.Buffer
	err := New(&buf, io.Discard).EmitResult(parse.Result{
		Flags: map[string]parse.FlagValue{
			"name": {Kind: parse.KindString, String: "world"},
		},
	}, "tool")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "toolname='world'")
}

func TestWrappedCompletionInjectsSpecFile(t *testing.T) {
	cmd, _, err := spec.ParseYAML("name: greet\nflags:\n  --loud: shout\n", "")
	require.NoError(t, err)

	snippet, err := Em.WrappedCompletion(cmd, "greet", "/tmp/spec.yaml")
	require.NoError(t, err)
	assert.Contains(t, snippet, "_greet_completion() {")
	assert.Contains(t, snippet, "local -x "+cmdutil.EnvSpecFile+"="+Em.Quote("/tmp/spec.yaml"))
	assert.Contains(t, snippet, "complete -o noquote -F _greet_completion greet")
}

func TestEmitUpUsesBashBasename(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitUp("name: greet\n"))
	assert.Contains(t, buf.String(), `${0##*/}`)
}

func TestEmitResultRunsUnderBash(t *testing.T) {
	snippet := emitResult(t, parse.Result{
		Flags: map[string]parse.FlagValue{
			"loud": {Kind: parse.KindBool, Bool: true},
			"tag":  {Kind: parse.KindStrings, Strings: []string{"a", "b"}},
		},
		Positionals: map[string]parse.PositionalValue{
			"name":  {Values: []string{"world"}},
			"extra": {Values: []string{"one", "two"}, Variadic: true},
		},
	})
	script := snippet + fmt.Sprintf(`
printf '%%s\n' "${%s}"
printf '%%s\n' "${%s}"
printf '%%s\n' "${%s[*]}"
printf '%%s\n' "${%s[*]}"
`, "compleo_loud", "compleo_name", "compleo_tag", "compleo_extra")
	out, err := bashCmd(script).Output()
	require.NoError(t, err)
	assert.Equal(t, "true\nworld\na b\none two\n", string(out))
}

func TestEmitHelpPrintsTextAndExitsZero(t *testing.T) {
	cmd := bashCmd(emitHelp(t, "Usage:\n  mytool [flags]"))
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "Usage:\n  mytool [flags]\n", string(out))
}

func TestEmitHelpQuotesSpecialCharacters(t *testing.T) {
	cmd := bashCmd(emitHelp(t, "it's $dangerous `too`"))
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "it's $dangerous `too`\n", string(out))
}

func TestEmitHelpStopsScriptEvenInsideAFunction(t *testing.T) {
	cmd := bashCmd(`
main() {
  eval "$SNIPPET"
  printf '%s\n' unreachable
}
main
printf '%s\n' "outer after main"
`)
	cmd.Env = append(os.Environ(), "SNIPPET="+emitHelp(t, "help text"))
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "help text\n", string(out))
}

func TestEmitErrorPrintsNameAndErrToStderrAndExitsTwo(t *testing.T) {
	cmd := bashCmd(emitError(t, "mytool", errors.New("bad flag")))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, runErr, &exitErr)
	assert.Equal(t, 2, exitErr.ExitCode())
	assert.Equal(t, "mytool: bad flag\n", stderr.String())
}

func TestEmitErrorQuotesSpecialCharacters(t *testing.T) {
	cmd := bashCmd(emitError(t, "it's", errors.New("$dangerous `too`")))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, runErr, &exitErr)
	assert.Equal(t, "it's: $dangerous `too`\n", stderr.String())
}

func TestEmitGeneratedPrintsConfirmationToStderrAndExitsZero(t *testing.T) {
	cmd := bashCmd(emitGenerated(t, "greet"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Contains(t, stderr.String(), "To activate completion for greet in this shell, run:")
	assert.Contains(t, stderr.String(), "complete -o noquote -F _greet_completion greet")
	assert.NotContains(t, stderr.String(), "warning:")
}

func TestEmitGeneratedPrintsOneWarningForMissingMacros(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitGenerated(Em.ActivationHint("greet"), []string{"carapace", "missingb"}))

	cmd := bashCmd(buf.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Equal(t, 1, strings.Count(stderr.String(), "warning:"))
	assert.Contains(t, stderr.String(), "warning: not on PATH, so these completion macros will not work: $carapace.*, $missingb.*")
	assert.Contains(t, stderr.String(), "To activate completion for greet in this shell, run:")
}

func TestEmitGeneratedActivationHintRunsUnderBash(t *testing.T) {
	hint := "complete -o noquote -F _greet_completion greet"
	require.Contains(t, emitGenerated(t, "greet"), hint)

	script := fmt.Sprintf(`
_greet_completion() { :; }
%s
complete -p greet
`, hint)
	out, err := bashCmd(script).CombinedOutput()
	if err != nil {
		t.Skipf("this bash rejected the activation hint: %s", out)
	}
	assert.Contains(t, string(out), "_greet_completion")
}

func TestEmitUpIsSyntacticallyValidBash(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitUp("flags:\n  --loud: shout\n"))
	require.NoError(t, bashCmd("set -n; "+buf.String()).Run())
}

func TestEmitUpGeneratedCodeInvokesParseWithNameArgsAndSpec(t *testing.T) {
	original := "description: it's $dangerous `here`\nflags:\n  --loud: shout\n"
	var buf bytes.Buffer
	require.NoError(t, New(&buf, io.Discard).EmitUp(original))

	captureFile := filepath.Join(t.TempDir(), "captured")
	script := fmt.Sprintf(`
compleo() {
  { printf '%%s\n' "ARGS:$*"; printf '%%s\n' "SPEC-START"; cat; printf '%%s' "SPEC-END"; } > %s
  printf '%%s\n' true
}
%s
`, Em.Quote(captureFile), buf.String())

	_, err := bashCmd(script, "scriptname", "world", "--loud").Output()
	require.NoError(t, err)

	captured, err := os.ReadFile(captureFile)
	require.NoError(t, err)
	got := string(captured)
	assert.Contains(t, got, "ARGS:parse --shell=bash --name=scriptname -- world --loud")
	assert.Contains(t, got, "SPEC-START\n"+original+"\nSPEC-END")
}

func TestEmitActivateExportsCompletionShell(t *testing.T) {
	assert.Contains(t, emitActivate(t, "/some/dir"), "export "+cmdutil.EnvCompletionShell+"="+Em.Quote(Em.Name()))
}

func TestEmitActivateSourcesCompletionFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "greet"), []byte("printf '%s\\n' sourced\n"), 0o644))

	out, err := bashCmd(emitActivate(t, dir)).Output()
	require.NoError(t, err)
	assert.Equal(t, "sourced\n", string(out))
}

func TestEmitActivateCompleoFunctionWrapsTheRealBinary(t *testing.T) {
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "compleo"), []byte("#!/bin/sh\necho \"wrapped: $*\"\n"), 0o755))

	completionDir := t.TempDir()
	script := fmt.Sprintf(`
export PATH=%[1]s:"$PATH"
%[2]s
compleo foo bar
`, Em.Quote(binDir), emitActivate(t, completionDir))

	out, err := bashCmd(script).Output()
	require.NoError(t, err)
	assert.Equal(t, "wrapped: foo bar\n", string(out))
}

func TestWrappedCompletionIsSyntacticallyValidBash(t *testing.T) {
	cmd, _, err := spec.ParseYAML("name: greet\nflags:\n  --loud: shout\n", "")
	require.NoError(t, err)
	snippet, err := Em.WrappedCompletion(cmd, "greet", "/some/dir/.greet.spec.yaml")
	require.NoError(t, err)

	require.NoError(t, exec.Command("bash", "--noprofile", "--norc", "-n", "-c", snippet).Run())
}
