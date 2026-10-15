package spec

import (
	"fmt"
	"strconv"
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
)

// Validate reports whether s is a valid t: cobra only ever parses these as plain strings.
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
	out.Completion.Flag = c.Completion.Flag
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
