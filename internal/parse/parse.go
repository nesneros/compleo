// Package parse executes a spec's cobra command tree against a command line and returns
// the result as a plain Go struct: which (sub)command was invoked, and the values bound
// to its flags and positional arguments. carapace-spec's ToCobraE gives real cobra/pflag
// parsing for flags, but nothing for positionals at all — Args is always
// cobra.ArbitraryArgs — so binding them to spec.Command's Positionals, and validating
// their arity, is entirely this package's job.
package parse

import (
	"fmt"
	"strconv"

	"github.com/nesneros/compleo/internal/spec"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Result is a parsed command line: which (sub)command of the spec was invoked, and the
// values bound to its flags and positional arguments.
type Result struct {
	// Path is the invoked (sub)command's name path, e.g. ["mytool", "sub"].
	Path []string

	Flags       map[string]FlagValue
	Positionals map[string]PositionalValue
}

// FlagKind identifies which of FlagValue's value fields holds the value.
type FlagKind string

const (
	KindString  FlagKind = "string"
	KindStrings FlagKind = "strings"
	KindBool    FlagKind = "bool"
	KindCount   FlagKind = "count"
)

// FlagValue is one flag's parsed value. carapace/cobra register every flag as one of
// four pflag kinds, named by Kind — exactly one of String/Strings/Bool/Count is
// meaningful. Changed reports whether the flag was actually present on the command
// line, as opposed to holding its zero or spec-provided default value.
type FlagValue struct {
	Kind    FlagKind
	Changed bool
	String  string
	Strings []string
	Bool    bool
	Count   int
}

// PositionalValue is one named positional argument's parsed value(s). Values has more
// than one element only for a variadic positional, and is empty if an optional
// positional was omitted.
type PositionalValue struct {
	Values []string
}

// Parse executes cmd (built from sp, e.g. via spec.Command(sp.ToCarapace()).ToCobraE())
// against argv and returns the result of whichever (sub)command argv invokes. Any
// run: the spec may define is irrelevant here and is overridden: compleo parses, it
// doesn't execute.
//
// Subcommands are matched to sp.Commands by name rather than by position: cobra sorts
// Commands() alphabetically by default (EnableCommandSorting), so it generally won't
// match sp.Commands' declaration order.
func Parse(cmd *cobra.Command, sp spec.Command, argv []string) (Result, error) {
	var result Result
	if err := wire(cmd, sp, &result); err != nil {
		return Result{}, err
	}

	cmd.SetArgs(argv)
	if err := cmd.Execute(); err != nil {
		return Result{}, err
	}
	return result, nil
}

func wire(cmd *cobra.Command, sp spec.Command, result *Result) error {
	if err := validatePositionals(sp.Positionals); err != nil {
		return fmt.Errorf("command %q: %w", sp.Name, err)
	}

	positionals := sp.Positionals
	cmd.Args = func(c *cobra.Command, args []string) error {
		_, err := bindPositionals(positionals, args)
		return err
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		bound, err := bindPositionals(positionals, args)
		if err != nil {
			return err
		}
		result.Path = commandPath(c)
		result.Positionals = bound
		result.Flags = bindFlags(c)
		return nil
	}

	for _, sub := range cmd.Commands() {
		for _, subSpec := range sp.Commands {
			if subSpec.Name == sub.Name() {
				if err := wire(sub, subSpec, result); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// validatePositionals checks the shape rules bindPositionals assumes: at most one
// positional may be variadic and it must be last, and no required positional may follow
// an optional one (there would be no way to tell, from a flat argument list, which
// missing argument belongs to which).
func validatePositionals(positionals []spec.Positional) error {
	seenOptional := false
	for i, p := range positionals {
		if p.Variadic && i != len(positionals)-1 {
			return fmt.Errorf("variadic positional %q must be the last positional", p.Name)
		}
		if !p.Required {
			seenOptional = true
		} else if seenOptional {
			return fmt.Errorf("required positional %q follows an optional one", p.Name)
		}
	}
	return nil
}

// bindPositionals assigns args to positionals' names in order; a trailing variadic
// positional absorbs every remaining argument.
func bindPositionals(positionals []spec.Positional, args []string) (map[string]PositionalValue, error) {
	bound := make(map[string]PositionalValue, len(positionals))

	i := 0
	for _, p := range positionals {
		if p.Variadic {
			rest := args[i:]
			if p.Required && len(rest) == 0 {
				return nil, fmt.Errorf("missing required argument %q", p.Name)
			}
			bound[p.Name] = PositionalValue{Values: append([]string(nil), rest...)}
			i = len(args)
			continue
		}
		if i >= len(args) {
			if p.Required {
				return nil, fmt.Errorf("missing required argument %q", p.Name)
			}
			bound[p.Name] = PositionalValue{}
			continue
		}
		bound[p.Name] = PositionalValue{Values: []string{args[i]}}
		i++
	}

	if i < len(args) {
		return nil, fmt.Errorf("unexpected argument(s): %v", args[i:])
	}
	return bound, nil
}

func commandPath(c *cobra.Command) []string {
	var path []string
	for ; c != nil; c = c.Parent() {
		path = append([]string{c.Name()}, path...)
	}
	return path
}

func bindFlags(cmd *cobra.Command) map[string]FlagValue {
	flags := make(map[string]FlagValue)
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		fv := FlagValue{Changed: f.Changed}
		switch f.Value.Type() {
		case "bool":
			fv.Kind = KindBool
			fv.Bool, _ = strconv.ParseBool(f.Value.String())
		case "count":
			fv.Kind = KindCount
			fv.Count, _ = strconv.Atoi(f.Value.String())
		case "stringSlice":
			fv.Kind = KindStrings
			if sv, ok := f.Value.(pflag.SliceValue); ok {
				fv.Strings = sv.GetSlice()
			}
		default:
			fv.Kind = KindString
			fv.String = f.Value.String()
		}
		flags[f.Name] = fv
	})
	return flags
}
