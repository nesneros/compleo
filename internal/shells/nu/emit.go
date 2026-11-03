package nu

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nesneros/compleo/internal/parse"
	"github.com/nesneros/compleo/internal/shells"
)

// EmitGenerated writes a nuon record the script prints and then exits 0.
// hint is the completion-target shell's ActivationHint. When missing is non-empty, the
// message starts with one warning listing those $bin.* completion macros.
func (e Emitter) EmitGenerated(hint string, missing []string) error {
	var lines []string
	if len(missing) > 0 {
		macros := make([]string, len(missing))
		for i, bin := range missing {
			macros[i] = "$" + bin + ".*"
		}
		lines = append(lines, "warning: not on PATH, so these completion macros will not work: "+strings.Join(macros, ", "))
	}
	lines = append(lines, shells.ActivationDescription, hint)
	return e.writeRecord("generated", "message", e.Quote(strings.Join(lines, "\n")))
}

// EmitUp writes nu that runs "compleo parse --shell=nu" and lets that record through.
// The caller runs this where $name is the script basename and $args is its argument list.
func (e Emitter) EmitUp(rawSpec string) error {
	_, err := fmt.Fprintf(e.out, "%s | ^compleo parse --shell=nu --name $name -- ...$args\n", e.Quote(rawSpec))
	return err
}

// EmitHelp writes a nuon record the script prints and then exits 0.
func (e Emitter) EmitHelp(text string) error {
	return e.writeRecord("help", "text", e.Quote(strings.TrimRight(text, "\n")))
}

// EmitError writes a nuon record the script prints to stderr and then exits 2.
func (e Emitter) EmitError(name string, parseErr error) error {
	return e.writeRecord("error", "message", e.Quote(fmt.Sprintf("%s: %s", name, parseErr)))
}

func (e Emitter) writeRecord(kind, field, quotedValue string) error {
	_, err := fmt.Fprintf(e.out, "{kind: %s, %s: %s}\n", e.Quote(kind), field, quotedValue)
	return err
}

// EmitResult writes one nuon record: kind "values", plus one field per flag and positional.
// A repeatable flag or variadic positional is a list. Nu resolves `source` before the script
// runs, so the script reads this record with `from nuon` instead of sourcing generated code.
func (e Emitter) EmitResult(result parse.Result, prefix string) error {
	fields := []string{"kind: " + e.Quote("values")}

	var arrays []string
	for _, name := range shells.SortedKeys(result.Flags) {
		fv := result.Flags[name]
		if fv.Kind == parse.KindStrings {
			arrays = append(arrays, nuonField(prefix, name, nuonList(fv.Strings)))
			continue
		}
		fields = append(fields, nuonField(prefix, name, e.Quote(flagScalar(fv))))
	}

	for _, name := range shells.SortedKeys(result.Positionals) {
		pv := result.Positionals[name]
		if pv.Variadic {
			arrays = append(arrays, nuonField(prefix, name, nuonList(pv.Values)))
			continue
		}
		var value string
		if len(pv.Values) > 0 {
			value = pv.Values[0]
		}
		fields = append(fields, nuonField(prefix, name, e.Quote(value)))
	}

	fields = append(fields, arrays...)
	_, err := fmt.Fprintln(e.out, "{"+strings.Join(fields, ", ")+"}")
	return err
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

func nuonField(prefix, name, value string) string {
	return shells.VarName(prefix, name) + ": " + value
}

func nuonList(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = Em.Quote(v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
