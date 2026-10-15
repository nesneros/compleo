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

	// HelpRequested: cobra intercepts --help before RunE runs, so Path/Flags/Positionals
	// stay zero-valued whenever this is true — check it before relying on them.
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

// PositionalValue is one positional's value(s); Variadic tells callers like zsh.Emit to render it as a list.
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

// helpRequested reports whether --help was changed anywhere in cmd's tree: cobra
// intercepts --help before RunE runs, so there's no other way to detect it per
// resolved command.
func helpRequested(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("help") || slices.ContainsFunc(cmd.Commands(), helpRequested)
}

func wire(cmd *cobra.Command, sp spec.Command, result *Result) error {
	return wireCommand(cmd, sp, result, nil)
}

// wireCommand recurses with inherited: a subcommand's effective flag types are its own
// Flags/PersistentFlags plus whatever its ancestors declared as PersistentFlags — cobra
// merges ancestors' persistent flags into a subcommand's own cmd.Flags() the same way.
func wireCommand(cmd *cobra.Command, sp spec.Command, result *Result, inherited map[string]spec.Type) error {
	if err := validatePositionals(sp.Positionals); err != nil {
		return fmt.Errorf("command %q: %w", sp.Name, err)
	}

	positionals := sp.Positionals
	flagTypes := mergeFlagTypes(inherited, flagTypeMap(sp.Flags), flagTypeMap(sp.PersistentFlags))
	cmd.Args = func(c *cobra.Command, args []string) error {
		_, err := bindPositionals(positionals, args)
		return err
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		bound, err := bindPositionals(positionals, args)
		if err != nil {
			return err
		}
		flags, err := bindFlags(c, flagTypes)
		if err != nil {
			return err
		}
		result.Path = commandPath(c)
		result.Positionals = bound
		result.Flags = flags
		return nil
	}

	childInherited := mergeFlagTypes(inherited, flagTypeMap(sp.PersistentFlags))
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

// flagTypeMap extracts each flag's declared Type, keyed by its bare name.
func flagTypeMap(fs spec.FlagSet) map[string]spec.Type {
	m := make(map[string]spec.Type, len(fs))
	for name, f := range fs {
		m[name] = f.Type
	}
	return m
}

// mergeFlagTypes unions maps left to right; a later map's entry wins on a name collision.
func mergeFlagTypes(maps ...map[string]spec.Type) map[string]spec.Type {
	size := 0
	for _, m := range maps {
		size += len(m)
	}
	merged := make(map[string]spec.Type, size)
	for _, m := range maps {
		stdmaps.Copy(merged, m)
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
				if err := p.Type.Validate(v); err != nil {
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
		if err := p.Type.Validate(args[i]); err != nil {
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

// bindFlags also validates against flagTypes: cobra/pflag only ever parse a compleo
// Type as a plain string, so this is the only place an invalid value is caught.
func bindFlags(cmd *cobra.Command, flagTypes map[string]spec.Type) (map[string]FlagValue, error) {
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
				if err := flagTypes[f.Name].Validate(v); err != nil {
					firstErr = fmt.Errorf("--%s: %w", f.Name, err)
					return
				}
			}
		default:
			fv.Kind = KindString
			fv.String = f.Value.String()
			if err := flagTypes[f.Name].Validate(fv.String); err != nil {
				firstErr = fmt.Errorf("--%s: %w", f.Name, err)
				return
			}
		}
		flags[f.Name] = fv
	})
	if firstErr != nil {
		return nil, firstErr
	}
	return flags, nil
}
