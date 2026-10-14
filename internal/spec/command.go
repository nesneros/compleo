package spec

import (
	"fmt"
	"strconv"
	"time"

	carapacespec "github.com/carapace-sh/carapace-spec"
	"github.com/carapace-sh/carapace/pkg/command"
	yaml "gopkg.in/yaml.v3"
)

// Command is compleo's superset of carapace-spec's command.Command, adding Positionals
// and FlagTypes, which carapace-spec has no fields for at all.
type Command struct {
	command.Command `yaml:",inline"`

	// yaml:"-": decoded separately by UnmarshalYAML, so subcommands get Positionals too.
	Commands []Command `yaml:"-"`

	Positionals []Positional `yaml:"positionals,omitempty"`

	// FlagTypes maps a flag's bare name to a compleo value type (carapace-spec flags are
	// only ever string/[]string/bool/count).
	FlagTypes map[string]Type `yaml:"flagtypes,omitempty"`
}

// UnmarshalYAML re-decodes "commands" into []Command (not command.Command.Commands'
// plain []command.Command), so subcommands carry Positionals/FlagTypes too.
func (c *Command) UnmarshalYAML(value *yaml.Node) error {
	type alias Command // no UnmarshalYAML method, to avoid infinite recursion
	if err := value.Decode((*alias)(c)); err != nil {
		return err
	}

	for i := 0; i+1 < len(value.Content); i += 2 {
		if value.Content[i].Value == "commands" {
			if err := value.Content[i+1].Decode(&c.Commands); err != nil {
				return err
			}
			break
		}
	}
	c.Command.Commands = nil // superseded by c.Commands
	return nil
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

// toCarapace converts c, recursively, into carapace-spec's Command (so ParseYAML can
// call .ToCobraE() on it directly), dropping Positionals (FlagTypes is consumed into
// flag description hints, see addFlagTypeHints, then dropped too).
func (c Command) toCarapace() carapacespec.Command {
	cmd := c.Command
	cmd.Flags = addFlagTypeHints(cmd.Flags, c.FlagTypes)
	cmd.Commands = make([]command.Command, len(c.Commands))
	for i, sub := range c.Commands {
		cmd.Commands[i] = command.Command(sub.toCarapace())
	}
	return carapacespec.Command(cmd)
}

// addFlagTypeHints appends a backtick-quoted type name to each typed flag's description, so
// usage shows e.g. "--times int" — pflag.UnquoteUsage prefers a backtick-quoted segment over
// the flag's actual registered type.
func addFlagTypeHints(flags command.FlagSet, flagTypes map[string]Type) command.FlagSet {
	if len(flagTypes) == 0 {
		return flags
	}
	hinted := make(command.FlagSet, len(flags))
	for name, f := range flags {
		if t, ok := flagTypes[name]; ok && t != typeString {
			f.Description += " (`" + string(t) + "`)"
		}
		hinted[name] = f
	}
	return hinted
}
