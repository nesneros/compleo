package spec

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	carapacespec "github.com/carapace-sh/carapace-spec"
	"github.com/carapace-sh/carapace/pkg/command"
)

// Command mirrors carapace-sh/carapace's pkg/command.Command (imported here as "command") —
// same field names/YAML tags, duplicated rather than embedded, so a compleo extension (Type on
// Flag, Positionals here) lives directly on the field it extends, not in a side-channel map
// keyed by name alongside it. toCarapace converts back to the real command.Command, just before
// building the cobra tree — the one place that still needs it. Keep this in sync by hand if
// command.Command ever changes; nothing enforces it automatically.
//
// https://github.com/carapace-sh/carapace/blob/v1.16.3/pkg/command/command.go
type Command struct {
	Name        string          `yaml:"name"`
	Aliases     []string        `yaml:"aliases,omitempty"`
	Description string          `yaml:"description,omitempty"`
	Group       string          `yaml:"group,omitempty"`
	Hidden      bool            `yaml:"hidden,omitempty"`
	Parsing     command.Parsing `yaml:"parsing,omitempty"`

	Flags           FlagSet     `yaml:"flags,omitempty"`
	PersistentFlags FlagSet     `yaml:"persistentflags,omitempty"`
	ExclusiveFlags  [][]string  `yaml:"exclusiveflags,omitempty"`
	Run             command.Run `yaml:"run,omitempty"`
	Completion      Completion  `yaml:"completion,omitempty"`
	Commands        []Command   `yaml:"commands,omitempty"`

	Documentation Documentation     `yaml:"documentation,omitempty"`
	Examples      map[string]string `yaml:"examples,omitempty"`

	Positionals []Positional `yaml:"positionals,omitempty"`
}

// Completion mirrors command.Command's own anonymous Completion field shape — Go can't
// reference another package's anonymous struct type from outside it, so this is a plain copy.
//
// https://github.com/carapace-sh/carapace/blob/v1.16.3/pkg/command/command.go
type Completion struct {
	Flag          map[string][]string `yaml:"flag,omitempty"`
	Positional    [][]string          `yaml:"positional,omitempty"`
	PositionalAny []string            `yaml:"positionalany,omitempty"`
	Dash          [][]string          `yaml:"dash,omitempty"`
	DashAny       []string            `yaml:"dashany,omitempty"`
}

// Documentation mirrors command.Command's own anonymous Documentation field shape (see
// Completion above — same reason this is a plain copy, not a reference).
//
// https://github.com/carapace-sh/carapace/blob/v1.16.3/pkg/command/command.go
type Documentation struct {
	Command       string            `yaml:"command,omitempty"`
	Flag          map[string]string `yaml:"flag,omitempty"`
	Positional    []string          `yaml:"positional,omitempty"`
	PositionalAny string            `yaml:"positionalany,omitempty"`
	Dash          []string          `yaml:"dash,omitempty"`
	DashAny       string            `yaml:"dashany,omitempty"`
}

// Positional describes one named positional argument.
type Positional struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
	Variadic    bool   `yaml:"variadic,omitempty"`
	Type        Type   `yaml:"type,omitempty"`

	// Values is the allowed set for Type == typeEnum; meaningless otherwise. Unlike Flag's
	// Values, this never feeds completion (carapace-spec's own positional completion is
	// index-keyed, not name-keyed, so there's no single natural slot to derive it into).
	Values []string `yaml:"values,omitempty"`
}

// Validate reports whether s is a valid value for p: Type.Validate for everything but enum,
// which needs Values too (Type alone doesn't carry the allowed set).
func (p Positional) Validate(s string) error {
	if p.Type == typeEnum {
		return validateEnum(s, p.Values)
	}
	return p.Type.Validate(s)
}

// Type coerces a flag's or positional's parsed string value; cobra never sees it.
type Type string

const (
	typeString   Type = ""
	typeInt      Type = "int"
	typeFloat    Type = "float"
	typeBool     Type = "bool"
	typeDuration Type = "duration"
	typePath     Type = "path"
	// typeEnum alone doesn't carry the allowed set — see Flag.Values/Positional.Values and
	// the Flag.Validate/Positional.Validate methods that combine the two.
	typeEnum Type = "enum"
)

// validateEnum reports whether s is one of values — shared by Flag.Validate and
// Positional.Validate, since typeEnum means the same thing on both.
func validateEnum(s string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("type enum declared with no values")
	}
	if !slices.Contains(values, s) {
		return fmt.Errorf("invalid value %q: must be one of %s", s, strings.Join(values, ", "))
	}
	return nil
}

// Validate reports whether s is a valid t: cobra only ever parses these as plain strings.
// typeEnum is intentionally absent here — it needs Values too, which Type alone never
// carries; use Flag.Validate/Positional.Validate instead, which have access to both.
func (t Type) Validate(s string) error {
	switch t {
	case typeInt:
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("invalid int value %q", s)
		}
	case typeFloat:
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("invalid float value %q", s)
		}
	case typeBool:
		if _, err := strconv.ParseBool(s); err != nil {
			return fmt.Errorf("invalid bool value %q", s)
		}
	case typeDuration:
		if _, err := time.ParseDuration(s); err != nil {
			return fmt.Errorf("invalid duration value %q", s)
		}
	}
	return nil
}

// toCarapace converts c, recursively, into carapace-spec's Command so ParseYAML can call
// .ToCobraE() on it directly. Positionals has no carapace-spec equivalent and is dropped here
// (it's consumed separately by applySpec).
func (c Command) toCarapace() carapacespec.Command {
	sub := make([]command.Command, len(c.Commands))
	for i, s := range c.Commands {
		sub[i] = command.Command(s.toCarapace())
	}

	out := command.Command{
		Name:            c.Name,
		Aliases:         c.Aliases,
		Description:     c.Description,
		Group:           c.Group,
		Hidden:          c.Hidden,
		Parsing:         c.Parsing,
		Flags:           c.Flags.toCarapace(),
		PersistentFlags: c.PersistentFlags.toCarapace(),
		ExclusiveFlags:  c.ExclusiveFlags,
		Run:             c.Run,
		Commands:        sub,
		Examples:        c.Examples,
	}
	out.Completion.Flag = withEnumCompletions(c.Completion.Flag, c.Flags, c.PersistentFlags)
	out.Completion.Positional = c.Completion.Positional
	out.Completion.PositionalAny = c.Completion.PositionalAny
	out.Completion.Dash = c.Completion.Dash
	out.Completion.DashAny = c.Completion.DashAny
	out.Documentation.Command = c.Documentation.Command
	out.Documentation.Flag = c.Documentation.Flag
	out.Documentation.Positional = c.Documentation.Positional
	out.Documentation.PositionalAny = c.Documentation.PositionalAny
	out.Documentation.Dash = c.Documentation.Dash
	out.Documentation.DashAny = c.Documentation.DashAny
	return carapacespec.Command(out)
}

// withEnumCompletions copies completion (may be nil) and adds an entry, from Values, for
// every enum flag across flagSets that doesn't already have one explicitly declared — an
// explicit completion (e.g. styled or described values) always wins over a derived one.
func withEnumCompletions(completion map[string][]string, flagSets ...FlagSet) map[string][]string {
	var out map[string][]string
	for _, fs := range flagSets {
		for name, f := range fs {
			if f.Type != typeEnum || len(f.Values) == 0 {
				continue
			}
			if _, declared := completion[name]; declared {
				continue
			}
			if out == nil {
				out = make(map[string][]string, len(completion))
				maps.Copy(out, completion)
			}
			out[name] = f.Values
		}
	}
	if out != nil {
		return out
	}
	return completion
}
