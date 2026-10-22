package doctor

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func formatString(t *testing.T, r Report) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, Format(&buf, r))
	return buf.String()
}

func okShell() ShellInfo {
	return ShellInfo{
		Interactive: "zsh", InteractiveRegistered: true,
		Interpreter: "zsh", InterpreterRegistered: true,
		CompletionShellEnv: "zsh",
	}
}

func TestSeverityCountsNothingForACleanReport(t *testing.T) {
	r := Report{Shell: okShell(), Carapace: CarapaceInfo{Found: true}}
	warnings, errs := Severity(r)
	assert.Zero(t, warnings)
	assert.Zero(t, errs)
}

func TestSeverityCountsShellDetectionErrorAsError(t *testing.T) {
	r := Report{Shell: ShellInfo{InteractiveErr: errors.New("boom"), InterpreterErr: errors.New("boom")}}
	_, errs := Severity(r)
	assert.Equal(t, 2, errs)
}

func TestSeverityCountsUnregisteredShellAsWarning(t *testing.T) {
	r := Report{Shell: ShellInfo{Interactive: "fish", Interpreter: "fish", CompletionShellEnv: "fish"}, Carapace: CarapaceInfo{Found: true}}
	warnings, _ := Severity(r)
	assert.Equal(t, 2, warnings)
}

func TestSeverityCountsUnactivatedShellAsWarning(t *testing.T) {
	r := Report{Shell: ShellInfo{
		Interactive: "zsh", InteractiveRegistered: true,
		Interpreter: "zsh", InterpreterRegistered: true,
	}, Carapace: CarapaceInfo{Found: true}}
	warnings, _ := Severity(r)
	assert.Equal(t, 1, warnings)
}

func TestFormatMentionsActivateWhenShellEnvUnset(t *testing.T) {
	r := Report{Shell: ShellInfo{
		Interactive: "zsh", InteractiveRegistered: true,
		Interpreter: "zsh", InterpreterRegistered: true,
	}, Carapace: CarapaceInfo{Found: true}}
	out := formatString(t, r)
	assert.Contains(t, out, "compleo activate")
}

func TestSeverityCountsCompletionShellMismatchAsWarning(t *testing.T) {
	r := Report{Shell: ShellInfo{
		Interactive: "zsh", InteractiveRegistered: true,
		Interpreter: "zsh", InterpreterRegistered: true,
		CompletionShellEnv: "bash",
	}, Carapace: CarapaceInfo{Found: true}}
	warnings, _ := Severity(r)
	assert.Equal(t, 1, warnings)
}

func TestSeverityCountsMissingCompletionDirAsWarning(t *testing.T) {
	r := Report{Shell: okShell(), CompletionDirErr: errors.New("no such dir"), Carapace: CarapaceInfo{Found: true}}
	warnings, _ := Severity(r)
	assert.Equal(t, 1, warnings)
}

func TestSeverityCountsCarapaceNotFoundAsWarning(t *testing.T) {
	r := Report{Shell: okShell(), Carapace: CarapaceInfo{Found: false}}
	warnings, _ := Severity(r)
	assert.Equal(t, 1, warnings)
}

func TestSeverityCountsOrphanedCompletionAsWarning(t *testing.T) {
	r := Report{Shell: okShell(), Carapace: CarapaceInfo{Found: true}, Completions: []CompletionEntry{
		{Name: "orphan", HasCompletionFile: true},
	}}
	warnings, _ := Severity(r)
	assert.Equal(t, 1, warnings)
}

func TestSeverityCountsBrokenSpecAsError(t *testing.T) {
	r := Report{Shell: okShell(), Carapace: CarapaceInfo{Found: true}, Completions: []CompletionEntry{
		{Name: "greet", HasCompletionFile: true, HasSpecFile: true, ScriptPath: "/bin/greet", SpecParseErr: errors.New("bad")},
	}}
	warnings, errs := Severity(r)
	assert.Zero(t, warnings)
	assert.Equal(t, 1, errs)
}

func TestSeverityCountsMissingMacroBinsAsWarning(t *testing.T) {
	r := Report{Shell: okShell(), Carapace: CarapaceInfo{Found: true}, Completions: []CompletionEntry{
		{Name: "greet", HasCompletionFile: true, HasSpecFile: true, ScriptPath: "/bin/greet", MissingMacroBins: []string{"jq"}},
	}}
	warnings, _ := Severity(r)
	assert.Equal(t, 1, warnings)
}

func TestSeverityCountsOldBashAsWarningOnlyWhenRelevant(t *testing.T) {
	notRelevant := Report{Shell: okShell(), Carapace: CarapaceInfo{Found: true}, Bash: BashInfo{Relevant: false, Found: true, Major: 3}}
	warnings, _ := Severity(notRelevant)
	assert.Zero(t, warnings)

	relevant := Report{Shell: okShell(), Carapace: CarapaceInfo{Found: true}, Bash: BashInfo{Relevant: true, Found: true, Major: 3, Minor: 2}}
	warnings, _ = Severity(relevant)
	assert.Equal(t, 1, warnings)
}

func TestFormatIncludesVersionHeaderWhenSet(t *testing.T) {
	out := formatString(t, Report{CompleoVersion: "1.2.3", Shell: okShell(), Carapace: CarapaceInfo{Found: true}})
	assert.Contains(t, out, "compleo 1.2.3\n")
}

func TestFormatOmitsVersionHeaderWhenUnset(t *testing.T) {
	out := formatString(t, Report{Shell: okShell(), Carapace: CarapaceInfo{Found: true}})
	assert.NotContains(t, out, "compleo \n")
}

func TestFormatRendersStatusPrefixes(t *testing.T) {
	r := Report{
		Shell:    ShellInfo{InteractiveErr: errors.New("boom"), InterpreterErr: errors.New("boom")},
		Carapace: CarapaceInfo{Found: false},
	}
	out := formatString(t, r)
	assert.Contains(t, out, "[error]")
	assert.Contains(t, out, "[warn]")
}

func TestFormatOmitsBashSectionWhenNotRelevant(t *testing.T) {
	out := formatString(t, Report{Shell: okShell(), Carapace: CarapaceInfo{Found: true}, Bash: BashInfo{Relevant: false}})
	assert.NotContains(t, out, "Bash\n")
}

func TestFormatIncludesBashSectionWhenRelevant(t *testing.T) {
	out := formatString(t, Report{Shell: okShell(), Carapace: CarapaceInfo{Found: true}, Bash: BashInfo{Relevant: true, Found: true, Major: 5, Minor: 2}})
	assert.Contains(t, out, "Bash\n")
	assert.Contains(t, out, "(bash 5.2)")
}

func TestFormatEndsWithSummaryLine(t *testing.T) {
	out := formatString(t, Report{Shell: okShell(), Carapace: CarapaceInfo{Found: true}})
	assert.Contains(t, out, "0 warning(s), 0 error(s)\n")
}
