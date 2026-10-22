package zsh

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/shells"
)

// EmitGenerated writes zsh that prints the hint for activating name's completion in the
// current shell: autoloading "_<name>" and binding it via compdef. When missing is
// non-empty, it first prints one warning listing those $bin.* completion macros.
func (e Emitter) EmitGenerated(w io.Writer, name string, missing []string) error {
	if len(missing) > 0 {
		macros := make([]string, len(missing))
		for i, bin := range missing {
			macros[i] = "$" + bin + ".*"
		}
		msg := "warning: not on PATH, so these completion macros will not work: " + strings.Join(macros, ", ")
		if err := e.PrintStderr(w, msg); err != nil {
			return err
		}
	}
	msg := fmt.Sprintf(
		"To activate completion for %s in this shell, run: autoload -Uz _%s && compdef _%s %s",
		name, name, name, name,
	)
	if err := e.PrintStderr(w, msg); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "exit %d\n", 0)
	return err
}

// EmitUp writes zsh that invokes "compleo parse" with "${0:t}", "$@", and rawSpec (as a
// quoted here-string) filled in.
func (e Emitter) EmitUp(w io.Writer, rawSpec string) error {
	_, err := fmt.Fprintf(w, `eval "$(compleo parse --name="${0:t}" -- "$@" <<<%s)"`+"\n", e.Quote(rawSpec))
	return err
}

// activateTemplate exports the interactive shell, defines a "compleo" shell function wrapping
// the real binary, adds dir to fpath, and autoloads and binds (via compdef, if available)
// every completion file in dir.
const activateTemplate = `export %[2]s=%[3]s
compleo() {
  command compleo "$@"
}
fpath+=(%[1]s)
autoload -Uz %[1]s/*
if (( $+functions[compdef] )); then
  for f (%[1]s/*) compdef ${f:t} ${f:t:s/_//}
fi
`

// EmitActivate writes zsh that wires the completions directory dir into the current shell
// and records that shell for later completion.
func (e Emitter) EmitActivate(w io.Writer, dir string) error {
	_, err := fmt.Fprintf(w, activateTemplate, e.Quote(dir), cmdutil.EnvCompletionShell, e.Quote(e.Name()))
	return err
}

// EmitResult writes zsh that populates a "compleo" assoc array from result's scalar flag and
// positional values. A repeatable flag or variadic positional instead gets its own zsh
// array, written after all scalar assignments.
func (Emitter) EmitResult(w io.Writer, result parse.Result) error {
	var writeErr error
	write := func(s string) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintln(w, s)
	}

	write("typeset -A compleo=()")

	var arrays []string
	for _, name := range shells.SortedKeys(result.Flags) {
		fv := result.Flags[name]
		if fv.Kind == parse.KindStrings {
			arrays = append(arrays, arrayAssignment(name, fv.Strings))
			continue
		}
		write(scalarAssignment(name, flagScalar(fv)))
	}

	for _, name := range shells.SortedKeys(result.Positionals) {
		pv := result.Positionals[name]
		if pv.Variadic {
			arrays = append(arrays, arrayAssignment(name, pv.Values))
			continue
		}
		var value string
		if len(pv.Values) > 0 {
			value = pv.Values[0]
		}
		write(scalarAssignment(name, value))
	}

	for _, line := range arrays {
		write(line)
	}
	return writeErr
}

// flagScalar renders a non-repeatable flag's value for "compleo".
func flagScalar(fv parse.FlagValue) string {
	switch fv.Kind {
	case parse.KindBool:
		if fv.Bool {
			return "true"
		}
		return "false"
	case parse.KindCount:
		return strconv.Itoa(fv.Count)
	default:
		return fv.String
	}
}

// name stays unquoted: quoting a zsh assoc-array key makes the literal quote chars part of it.
func scalarAssignment(name, value string) string {
	return "compleo[" + name + "]=" + Em.Quote(value)
}

// arrayAssignment declares and fills a dedicated array for a multi-valued flag or positional.
func arrayAssignment(name string, values []string) string {
	varName := arrayVarName(name)

	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = Em.Quote(v)
	}

	var b strings.Builder
	b.WriteString("typeset -a ")
	b.WriteString(varName)
	b.WriteString("=()\n")
	b.WriteString(varName)
	b.WriteString("=(")
	b.WriteString(strings.Join(quoted, " "))
	b.WriteString(")")
	return b.String()
}

// arrayVarName replaces non [A-Za-z0-9_] chars with "_": a hyphen breaks even referencing the var later.
func arrayVarName(name string) string {
	var b strings.Builder
	b.WriteString("compleo_")
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func init() {
	shells.RegisterEmitter(&Em)
}
