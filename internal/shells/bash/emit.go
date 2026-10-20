package bash

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/shells"
)

// EmitGenerated writes bash that prints an activation hint. When missing is non-empty,
// it first prints one warning listing those $bin.* completion macros.
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
		"To activate completion for %s in this shell, run: complete -o noquote -F _%s_completion %s",
		name, name, name,
	)
	if err := e.PrintStderr(w, msg); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "exit %d\n", 0)
	return err
}

// EmitUp writes bash that invokes "compleo parse". ${0##*/} is bash for zsh's ${0:t}.
func (e Emitter) EmitUp(w io.Writer, rawSpec string) error {
	_, err := fmt.Fprintf(w, `eval "$(compleo parse --name="${0##*/}" -- "$@" <<<%s)"`+"\n", e.Quote(rawSpec))
	return err
}

// activateTemplate defines a compleo function and sources every completion file in dir.
// Carapace's bash snippet registers itself with `complete` when sourced.
const activateTemplate = `compleo() {
  command compleo "$@"
}
for f in %[1]s/*; do
  [ -f "$f" ] && . "$f"
done
`

// EmitActivate writes bash that sources the completion files in dir.
func (e Emitter) EmitActivate(w io.Writer, dir string) error {
	_, err := fmt.Fprintf(w, activateTemplate, e.Quote(dir))
	return err
}

// EmitResult writes bash that fills a "compleo" associative array. declare -A needs bash 4;
// macOS /bin/bash is 3.2 and cannot run this.
func (Emitter) EmitResult(w io.Writer, result parse.Result) error {
	var writeErr error
	write := func(s string) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintln(w, s)
	}

	write("declare -A compleo")

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

func scalarAssignment(name, value string) string {
	return "compleo[" + name + "]=" + Em.Quote(value)
}

func arrayAssignment(name string, values []string) string {
	varName := arrayVarName(name)
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = Em.Quote(v)
	}
	return "declare -a " + varName + "=(" + strings.Join(quoted, " ") + ")"
}

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
