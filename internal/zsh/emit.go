package zsh

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/nesneros/compleo/internal/parse"
)

// EmitHelp wraps captured --help text as zsh that prints it and exits 0. Uses exit, not
// return, since the eval'd code may run at a script's top level where return isn't valid.
func EmitHelp(text string) string {
	return "print -r -- " + Quote(strings.TrimRight(text, "\n")) + "\nexit 0"
}

// EmitError wraps a wrapped script's own parse error as zsh that prints "name: err" to
// stderr and exits 2 (the conventional Unix "bad usage" code, distinct from main()'s
// generic exit(1) for compleo's own invocation failures) — so simply eval'ing compleo's
// output is enough to stop the script, no separate exit-code check needed by the caller.
func EmitError(name string, err error) string {
	return "print -u2 -- " + Quote(fmt.Sprintf("%s: %s", name, err)) + "\nexit 2"
}

// EmitGenerated wraps a COMPLEO_MODE=generate confirmation as zsh, same convention as
// EmitHelp/EmitError. Includes an activation hint for the current shell, since an
// already-running shell won't notice the new file without a compinit rerun (rescans all of
// $fpath) or this cheaper per-file equivalent: compleo can't run it directly, since compleo
// only ever runs as a subprocess, which can't modify its parent shell's completion state.
// Must autoload "_<name>" (the file's own name), not "_<name>_completion" (the function
// WrappedCompletion defines inside it) — autoload resolves by filename, and only "_<name>"
// matches a real file; calling it is what actually defines and rebinds the inner function.
func EmitGenerated(name, path string) string {
	msg := fmt.Sprintf(
		"%s: generated completion at %s (new shells pick it up automatically; to use it in this one now: autoload -Uz _%s && compdef _%s %s)",
		name, path, name, name, name,
	)
	return "print -u2 -- " + Quote(msg) + "\nexit 0"
}

// EmitComplete wraps a COMPLEO_MODE=complete result (carapace's raw completion wire format,
// not zsh) the same way EmitHelp wraps --help text: the wrapped script's own eval would
// otherwise try to execute the wire format as code, instead of the generated completion
// function's own "lines=$(...)" just capturing it as a string.
func EmitComplete(wireFormat string) string {
	return "print -r -- " + Quote(strings.TrimRight(wireFormat, "\n")) + "\nexit 0"
}

// Emit fills a fixed "compleo" assoc array with scalar values; a repeatable flag or
// variadic positional instead gets its own real zsh array, since assoc values are scalars only.
func Emit(result parse.Result) string {
	var scalars, arrays []string

	for _, name := range sortedKeys(result.Flags) {
		fv := result.Flags[name]
		if fv.Kind == parse.KindStrings {
			arrays = append(arrays, arrayAssignment(name, fv.Strings))
			continue
		}
		scalars = append(scalars, scalarAssignment(name, flagScalar(fv)))
	}

	for _, name := range sortedKeys(result.Positionals) {
		pv := result.Positionals[name]
		if pv.Variadic {
			arrays = append(arrays, arrayAssignment(name, pv.Values))
			continue
		}
		var value string
		if len(pv.Values) > 0 {
			value = pv.Values[0]
		}
		scalars = append(scalars, scalarAssignment(name, value))
	}

	var b strings.Builder
	b.WriteString("typeset -A compleo=()\n")
	for _, line := range scalars {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, line := range arrays {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimSuffix(b.String(), "\n")
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
	return "compleo[" + name + "]=" + Quote(value)
}

// arrayAssignment declares and fills a dedicated array for a multi-valued flag or positional.
func arrayAssignment(name string, values []string) string {
	varName := arrayVarName(name)

	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = Quote(v)
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

// sortedKeys returns m's keys sorted, for deterministic output despite map iteration order.
func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	return slices.Sorted(maps.Keys(m))
}
