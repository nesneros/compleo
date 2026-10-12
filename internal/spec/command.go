package spec

import (
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
	TypeString   Type = ""
	TypeInt      Type = "int"
	TypeFloat    Type = "float"
	TypeBool     Type = "bool"
	TypeDuration Type = "duration"
	TypePath     Type = "path"
)

// toCarapace converts c, recursively, into carapace-spec's Command (so ParseYAML can
// call .ToCobraE() on it directly), dropping Positionals/FlagTypes.
func (c Command) toCarapace() carapacespec.Command {
	cmd := c.Command
	cmd.Commands = make([]command.Command, len(c.Commands))
	for i, sub := range c.Commands {
		cmd.Commands[i] = command.Command(sub.toCarapace())
	}
	return carapacespec.Command(cmd)
}
