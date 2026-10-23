package bash

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/shells"
)

// ActivationHint is the instruction for activating name's completion in a bash shell right now.
func (Emitter) ActivationHint(name string) string {
	return fmt.Sprintf(
		"To activate completion for %s in this shell, run: complete -o noquote -F _%s_completion %s",
		name, name, name,
	)
}

// EmitGenerated writes bash that prints hint (built by the completion-target shell's own
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

// EmitUp writes bash that invokes "compleo parse" with "--shell=bash" (so the nested call
// doesn't need to redetect it). ${0##*/} is bash for zsh's ${0:t}.
func (e Emitter) EmitUp(rawSpec string) error {
	_, err := fmt.Fprintf(e.Out, `eval "$(compleo parse --shell=%s --name="${0##*/}" -- "$@" <<<%s)"`+"\n", e.Name(), e.Quote(rawSpec))
	return err
}

// activateTemplate exports the interactive shell, defines a compleo function, and sources
// every completion file in dir. Carapace's bash snippet registers itself with `complete` when sourced.
const activateTemplate = `export %[2]s=%[3]s
compleo() {
  command compleo "$@"
}
for f in %[1]s/*; do
  [ -f "$f" ] && . "$f"
done
`

// EmitActivate writes bash that sources the completion files in dir and records that shell
// for later completion.
func (e Emitter) EmitActivate(dir string) error {
	_, err := fmt.Fprintf(e.Out, activateTemplate, e.Quote(dir), cmdutil.EnvCompletionShell, e.Quote(e.Name()))
	return err
}

// EmitResult writes one <prefix><name> variable per flag and positional (prefix carries its
// own separator, if any — see shells.VarName). A repeatable flag or variadic positional is a
// real array, written after the scalars. Scalars are plain assignments, so this runs on bash
// 3.2 (no declare -A).
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

func arrayAssignment(prefix, name string, values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = Em.Quote(v)
	}
	return "declare -a " + shells.VarName(prefix, name) + "=(" + strings.Join(quoted, " ") + ")"
}
