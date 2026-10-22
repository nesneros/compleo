package doctor

import (
	"fmt"
	"io"
	"strings"
)

// level is a finding's severity. Values are ordered low to high, so a line's severity can only
// go up as more findings are folded in (via the builtin max).
type level int

const (
	levelOK level = iota
	levelWarn
	levelError
)

// String renders l the way Format shows it (see statusPrefix for the bracketed form).
func (l level) String() string {
	switch l {
	case levelWarn:
		return "warn"
	case levelError:
		return "error"
	default:
		return "ok"
	}
}

// line is one reported finding: a severity plus its message. Both Format and Severity derive
// these from a Report the same way, so they can never disagree on what counts as a problem.
type line struct {
	level level
	text  string
}

type section struct {
	title string
	lines []line
}

func sections(r Report) []section {
	ss := []section{
		{"Shell detection", shellLines(r.Shell)},
		{"Env vars", envLines(r.Env)},
		{"Completion directory", completionDirLines(r)},
		{"Completions", completionLines(r.Completions)},
		{"External completion dispatcher (carapace)", carapaceLines(r.Carapace)},
	}
	if r.Bash.Relevant {
		ss = append(ss, section{"Bash", bashLines(r.Bash)})
	}
	return ss
}

func shellLines(s ShellInfo) []line {
	var ls []line

	if s.InteractiveErr != nil {
		ls = append(ls, line{levelError, fmt.Sprintf(
			"interactive shell (ParentProcessName): could not detect: %s", s.InteractiveErr)})
	} else {
		lvl, note := levelOK, ""
		if !s.InteractiveRegistered {
			lvl, note = levelWarn, " (not a shell compleo has an emitter for)"
		}
		ls = append(ls, line{lvl, fmt.Sprintf(
			"interactive shell (ParentProcessName):  %s%s", s.Interactive, note)})
	}

	if s.InterpreterErr != nil {
		ls = append(ls, line{levelError, fmt.Sprintf(
			"script interpreter (ParentInterpreter): could not detect: %s", s.InterpreterErr)})
	} else {
		lvl, note := levelOK, ""
		if !s.InterpreterRegistered {
			lvl, note = levelWarn, " (not a shell compleo has an emitter for)"
		}
		ls = append(ls, line{lvl, fmt.Sprintf(
			"script interpreter (ParentInterpreter): %s%s", s.Interpreter, note)})
	}

	switch {
	case s.CompletionShellEnv == "":
		ls = append(ls, line{levelWarn,
			`_COMPLEO_COMPLETION_SHELL: unset — not activated in this shell? run: eval "$(compleo activate)"`})
	case s.InteractiveErr == nil && s.CompletionShellEnv != s.Interactive:
		ls = append(ls, line{levelWarn, fmt.Sprintf(
			"_COMPLEO_COMPLETION_SHELL=%s, but this shell looks like %s", s.CompletionShellEnv, s.Interactive)})
	default:
		ls = append(ls, line{levelOK, "_COMPLEO_COMPLETION_SHELL=" + s.CompletionShellEnv})
	}

	return ls
}

func envLines(vars []EnvVar) []line {
	ls := make([]line, 0, len(vars))
	for _, v := range vars {
		val := v.Value
		if !v.Set {
			val = "(unset)"
		}
		ls = append(ls, line{levelOK, fmt.Sprintf("%-26s %s", v.Name, val)})
	}
	return ls
}

func completionDirLines(r Report) []line {
	if r.CompletionDirErr != nil {
		return []line{{levelWarn, fmt.Sprintf(
			`%s: %s — not activated yet? run: eval "$(compleo activate)"`, r.CompletionDir, r.CompletionDirErr)}}
	}
	return []line{{levelOK, r.CompletionDir}}
}

func completionLines(entries []CompletionEntry) []line {
	if len(entries) == 0 {
		return []line{{levelOK, "(none generated yet)"}}
	}

	ls := make([]line, 0, len(entries))
	for _, c := range entries {
		lvl := levelOK
		var details []string

		if c.ScriptPath == "" {
			lvl = max(lvl, levelWarn)
			details = append(details, "script not found on $PATH (orphaned completion?)")
		}
		if c.HasSpecFile && !c.HasCompletionFile {
			lvl = max(lvl, levelWarn)
			details = append(details, "spec file present, completion file missing — run: compleo generate "+c.Name)
		}
		if c.HasCompletionFile && !c.HasSpecFile {
			lvl = max(lvl, levelWarn)
			details = append(details, "completion file present, spec companion missing")
		}
		if c.SpecParseErr != nil {
			lvl = max(lvl, levelError)
			errText := strings.ReplaceAll(c.SpecParseErr.Error(), "\n", " ")
			details = append(details, fmt.Sprintf(
				"spec no longer parses under this compleo: %s — run: compleo generate %s", errText, c.Name))
		}
		if len(c.MissingMacroBins) > 0 {
			lvl = max(lvl, levelWarn)
			details = append(details, "macro binaries missing on $PATH: "+strings.Join(c.MissingMacroBins, ", "))
		}

		text := c.Name
		if len(details) > 0 {
			text += ": " + strings.Join(details, "; ")
		}
		ls = append(ls, line{lvl, text})
	}
	return ls
}

func carapaceLines(c CarapaceInfo) []line {
	if !c.Found {
		return []line{{levelWarn,
			"carapace binary not found on $PATH — any $carapace.* completion macro fails silently at Tab time"}}
	}
	text := c.Path
	if c.VersionErr == nil && c.Version != "" {
		text += " (" + c.Version + ")"
	}
	return []line{{levelOK, text}}
}

func bashLines(b BashInfo) []line {
	if !b.Found {
		return []line{{levelWarn, "bash not found on $PATH, but a generated completion targets bash"}}
	}
	if b.Major == 0 {
		return []line{{levelOK, b.Path + " (version undetermined)"}}
	}
	lvl, note := levelOK, ""
	if b.Major < 4 {
		lvl, note = levelWarn, " — compleo's bash completion needs bash >=4 (declare -A, mapfile)"
	}
	return []line{{lvl, fmt.Sprintf("%s (bash %d.%d)%s", b.Path, b.Major, b.Minor, note)}}
}

func statusPrefix(l level) string {
	return fmt.Sprintf("%-7s", fmt.Sprintf("[%s]", l))
}

// Severity counts warnings and errors across every section — the same findings Format renders,
// so a caller deciding an exit code can never disagree with what the report itself says.
func Severity(r Report) (warnings, errors int) {
	for _, s := range sections(r) {
		for _, l := range s.lines {
			switch l.level {
			case levelWarn:
				warnings++
			case levelError:
				errors++
			}
		}
	}
	return warnings, errors
}

// Format writes r to w as human-readable text, plain stdout output — not eval'd, so no shell
// quoting or syntax concerns apply here, unlike every other Emit*-style output in compleo.
func Format(w io.Writer, r Report) error {
	if r.CompleoVersion != "" {
		if _, err := fmt.Fprintf(w, "compleo %s\n\n", r.CompleoVersion); err != nil {
			return err
		}
	}
	for _, s := range sections(r) {
		if _, err := fmt.Fprintf(w, "%s\n", s.title); err != nil {
			return err
		}
		for _, l := range s.lines {
			if _, err := fmt.Fprintf(w, "  %s %s\n", statusPrefix(l.level), l.text); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	warnings, errs := Severity(r)
	_, err := fmt.Fprintf(w, "%d warning(s), %d error(s)\n", warnings, errs)
	return err
}
