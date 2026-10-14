package zsh

import (
	"cmp"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/nesneros/compleo/internal/parse"
)

// EmitHelp writes captured --help text as zsh that prints it and exits 0. Uses exit, not
// return, since the eval'd code may run at a script's top level where return isn't valid.
func EmitHelp(w io.Writer, text string) error {
	_, err := fmt.Fprintf(w, "print -r -- %s\nexit 0\n", Quote(strings.TrimRight(text, "\n")))
	return err
}

// EmitError prints "name: err" to stderr and exits 2 (conventional "bad usage", distinct
// from main()'s generic exit(1) for compleo's own failures) — eval'ing the output alone
// stops the script, no separate exit-code check needed by the caller.
func EmitError(w io.Writer, name string, parseErr error) error {
	_, err := fmt.Fprintf(w, "print -u2 -- %s\nexit 2\n", Quote(fmt.Sprintf("%s: %s", name, parseErr)))
	return err
}

// EmitGenerated prints the activation hint for the current shell (compleo can't run it
// itself — a subprocess can't touch its parent shell's state). Must autoload "_<name>" (the
// file), not "_<name>_completion" (the function inside it) — only the file is a real
// autoload target; calling it is what defines and rebinds the inner function.
func EmitGenerated(w io.Writer, name string) error {
	msg := Quote(fmt.Sprintf(
		"To activate completion for %s in this shell, run: autoload -Uz _%s && compdef _%s %s",
		name, name, name, name,
	))
	_, err := fmt.Fprintf(w, "print -u2 -- %s\nexit 0\n", msg)
	return err
}

// EmitUp wires compleo into a script without it writing --name/--/"$@" itself: the printed
// zsh fills those in and embeds rawSpec as a quoted here-string. Not mode-aware — the
// generated command just inherits whatever COMPLEO_MODE the real invocation runs under.
func EmitUp(w io.Writer, rawSpec string) error {
	_, err := fmt.Fprintf(w, `eval "$(compleo parse --name="${0:t}" -- "$@" <<<%s)"`+"\n", Quote(rawSpec))
	return err
}

// EmitActivate adds dir to fpath plus a guarded autoload+compdef for everything already in
// it. Guarded because compdef doesn't exist until compinit has run once — before compinit
// (the documented use) this is a harmless no-op; it only does real work when activate
// (re-)runs in an already-initialized shell. dir always has at least "_compleo".
func EmitActivate(w io.Writer, dir string) error {
	_, err := fmt.Fprintf(w, `fpath+=(%[1]s)
autoload -Uz %[1]s/*
if (( $+functions[compdef] )); then
  for f (%[1]s/*) compdef ${f:t} ${f:t:s/_//}
fi
`, Quote(dir))
	return err
}

// Emit fills a fixed "compleo" assoc array with scalar values; a repeatable flag or
// variadic positional gets its own real zsh array instead, since assoc values are scalars
// only. Arrays are deferred so they still print after every scalar.
func Emit(w io.Writer, result parse.Result) error {
	var writeErr error
	write := func(s string) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintln(w, s)
	}

	write("typeset -A compleo=()")

	var arrays []string
	for _, name := range sortedKeys(result.Flags) {
		fv := result.Flags[name]
		if fv.Kind == parse.KindStrings {
			arrays = append(arrays, arrayAssignment(name, fv.Strings))
			continue
		}
		write(scalarAssignment(name, flagScalar(fv)))
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
