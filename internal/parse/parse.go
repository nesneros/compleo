// Package parse executes a spec's cobra command tree against a command line and returns
// the result as a plain Go struct. Positional binding/arity validation is this package's
// job: cobra's Args is always cobra.ArbitraryArgs, carapace-spec gives it nothing here.
package parse

import (
	"fmt"
	stdmaps "maps"
	"slices"
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

	// HelpRequested reports whether --help was invoked; Path, Flags, and Positionals are
	// zero-valued when this is true.
	HelpRequested bool

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

// FlagValue is one flag's parsed value; only the field named by Kind is meaningful.
// Changed reports whether the flag was present on the command line, vs. a default.
type FlagValue struct {
	Kind    FlagKind
	Changed bool
	String  string
	Strings []string
	Bool    bool
	Count   int
}

// PositionalValue is one positional's value(s). Variadic reports whether it should be
// treated as a list rather than a scalar.
type PositionalValue struct {
	Values   []string
	Variadic bool
}

// Parse executes cmd (built via sp.ToCobraE()) against argv and returns the result of
// whichever (sub)command argv invokes. Any spec run: is overridden — compleo parses, it
// doesn't execute. Subcommands are matched by name, not position (cobra sorts them).
func Parse(cmd *cobra.Command, sp spec.Command, argv []string) (Result, error) {
	// nil argv makes cobra fall back to os.Args — it can't tell "no args" from "unset".
	if argv == nil {
		panic("parse.Parse: argv must not be nil (use an empty slice for \"no arguments\")")
	}

	var result Result
	if err := wire(cmd, sp, &result); err != nil {
		return Result{}, err
	}

	cmd.SetArgs(argv)
	if err := cmd.Execute(); err != nil {
		return Result{}, err
	}
	result.HelpRequested = helpRequested(cmd)
	return result, nil
}

// helpRequested reports whether --help was set anywhere in cmd's command tree.
func helpRequested(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("help") || slices.ContainsFunc(cmd.Commands(), helpRequested)
}

func wire(cmd *cobra.Command, sp spec.Command, result *Result) error {
	return wireCommand(cmd, sp, result, nil)
}

// wireCommand wires sp onto cmd and recurses into matching subcommands. inherited carries
// flags declared as PersistentFlags by an ancestor, merged into each subcommand's own
// effective flag set.
func wireCommand(cmd *cobra.Command, sp spec.Command, result *Result, inherited map[string]spec.Flag) error {
	if err := validatePositionals(sp.Positionals); err != nil {
		return fmt.Errorf("command %q: %w", sp.Name, err)
	}

	positionals := sp.Positionals
	flags := mergeFlagMaps(inherited, sp.Flags, sp.PersistentFlags)
	cmd.Args = func(c *cobra.Command, args []string) error {
		_, err := bindPositionals(positionals, args)
		return err
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		bound, err := bindPositionals(positionals, args)
		if err != nil {
			return err
		}
		fv, err := bindFlags(c, flags)
		if err != nil {
			return err
		}
		result.Path = commandPath(c)
		result.Positionals = bound
		result.Flags = fv
		return nil
	}

	childInherited := mergeFlagMaps(inherited, sp.PersistentFlags)
	for _, sub := range cmd.Commands() {
		for _, subSpec := range sp.Commands {
			if subSpec.Name == sub.Name() {
				if err := wireCommand(sub, subSpec, result, childInherited); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// mergeFlagMaps unions maps (a map and any number of spec.FlagSets) left to right; a later
// one's entry wins on a name collision.
func mergeFlagMaps(base map[string]spec.Flag, flagSets ...spec.FlagSet) map[string]spec.Flag {
	size := len(base)
	for _, fs := range flagSets {
		size += len(fs)
	}
	merged := make(map[string]spec.Flag, size)
	stdmaps.Copy(merged, base)
	for _, fs := range flagSets {
		stdmaps.Copy(merged, fs)
	}
	return merged
}

// validatePositionals: at most one variadic, and it must be last; no required
// positional may follow an optional one.
func validatePositionals(positionals []spec.Positional) error {
	optional := ""
	for i, p := range positionals {
		if p.Variadic && i != len(positionals)-1 {
			return fmt.Errorf("variadic positional %q must be the last positional", p.Name)
		}
		if !p.Required {
			optional = p.Name
		} else if optional != "" {
			return fmt.Errorf("required positional %q follows optional %q", p.Name, optional)
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
			for _, v := range rest {
				if err := p.Validate(v); err != nil {
					return nil, fmt.Errorf("argument %q: %w", p.Name, err)
				}
			}
			bound[p.Name] = PositionalValue{Values: append([]string(nil), rest...), Variadic: true}
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
		if err := p.Validate(args[i]); err != nil {
			return nil, fmt.Errorf("argument %q: %w", p.Name, err)
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

// bindFlags collects each of cmd's registered flags' parsed values, validating each one
// against the corresponding spec.Flag's declared Type.
func bindFlags(cmd *cobra.Command, specFlags map[string]spec.Flag) (map[string]FlagValue, error) {
	flags := make(map[string]FlagValue)
	var firstErr error
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if firstErr != nil {
			return
		}
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
			for _, v := range fv.Strings {
				if err := specFlags[f.Name].Validate(v); err != nil {
					firstErr = fmt.Errorf("--%s: %w", f.Name, err)
					return
				}
			}
		default:
			fv.Kind = KindString
			fv.String = f.Value.String()
			// An unset flag with no declared default still registers as "" (pflag's own
			// zero value for a string-kind flag) — only validate that if the user actually
			// typed it, or the spec itself declared a default (worth catching if that
			// default is itself invalid); otherwise every optional typed flag would require
			// a default just to be leavable unset.
			if f.Changed || specFlags[f.Name].Default != "" {
				if err := specFlags[f.Name].Validate(fv.String); err != nil {
					firstErr = fmt.Errorf("--%s: %w", f.Name, err)
					return
				}
			}
		}
		flags[f.Name] = fv
	})
	if firstErr != nil {
		return nil, firstErr
	}
	return flags, nil
}
