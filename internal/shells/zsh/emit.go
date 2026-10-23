package zsh

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/shells"
)

// ActivationHint is the instruction for activating name's completion in a zsh shell right
// now: autoloading "_<name>" and binding it via compdef.
func (Emitter) ActivationHint(name string) string {
	return fmt.Sprintf(
		"To activate completion for %s in this shell, run: autoload -Uz _%s && compdef _%s %s",
		name, name, name, name,
	)
}

// EmitGenerated writes zsh that prints hint (built by the completion-target shell's own
// ActivationHint, which may differ from this shell). When missing is non-empty, it first
// prints one warning listing those $bin.* completion macros.
func (e Emitter) EmitGenerated(hint string, missing []string) error {
	if len(missing) > 0 {
		macros := make([]string, len(missing))
		for i, bin := range missing {
			macros[i] = "$" + bin + ".*"
		}
		msg := "warning: not on PATH, so these completion macros will not work: " + strings.Join(macros, ", ")
		if err := e.PrintStderr(msg); err != nil {
			return err
		}
	}
	if err := e.PrintStderr(hint); err != nil {
		return err
	}
	_, err := fmt.Fprintf(e.Out, "exit %d\n", 0)
	return err
}

// EmitUp writes zsh that invokes "compleo parse" with "--shell=zsh" (so the nested call
// doesn't need to redetect it), "${0:t}", "$@", and rawSpec (as a quoted here-string) filled in.
func (e Emitter) EmitUp(rawSpec string) error {
	_, err := fmt.Fprintf(e.Out, `eval "$(compleo parse --shell=%s --name="${0:t}" -- "$@" <<<%s)"`+"\n", e.Name(), e.Quote(rawSpec))
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
func (e Emitter) EmitActivate(dir string) error {
	_, err := fmt.Fprintf(e.Out, activateTemplate, e.Quote(dir), cmdutil.EnvCompletionShell, e.Quote(e.Name()))
	return err
}

// EmitResult writes one <prefix><name> variable per flag and positional (prefix carries its
// own separator, if any — see shells.VarName). A repeatable flag or variadic positional is a
// real array, written after the scalars.
func (e Emitter) EmitResult(result parse.Result, prefix string) error {
	var writeErr error
	write := func(s string) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintln(e.Out, s)
	}

	var arrays []string
	for _, name := range shells.SortedKeys(result.Flags) {
		fv := result.Flags[name]
		if fv.Kind == parse.KindStrings {
			arrays = append(arrays, arrayAssignment(prefix, name, fv.Strings))
			continue
		}
		write(scalarAssignment(prefix, name, flagScalar(fv)))
	}

	for _, name := range shells.SortedKeys(result.Positionals) {
		pv := result.Positionals[name]
		if pv.Variadic {
			arrays = append(arrays, arrayAssignment(prefix, name, pv.Values))
			continue
		}
		var value string
		if len(pv.Values) > 0 {
			value = pv.Values[0]
		}
		write(scalarAssignment(prefix, name, value))
	}

	for _, line := range arrays {
		write(line)
	}
	return writeErr
}

// flagScalar renders a non-repeatable flag's value.
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

func scalarAssignment(prefix, name, value string) string {
	return shells.VarName(prefix, name) + "=" + Em.Quote(value)
}

// arrayAssignment declares and fills the array for a multi-valued flag or positional.
func arrayAssignment(prefix, name string, values []string) string {
	v := shells.VarName(prefix, name)

	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = Em.Quote(value)
	}

	var b strings.Builder
	b.WriteString("typeset -a ")
	b.WriteString(v)
	b.WriteString("=()\n")
	b.WriteString(v)
	b.WriteString("=(")
	b.WriteString(strings.Join(quoted, " "))
	b.WriteString(")")
	return b.String()
}
