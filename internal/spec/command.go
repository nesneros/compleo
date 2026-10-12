package spec

import (
	"github.com/carapace-sh/carapace/pkg/command"
	yaml "gopkg.in/yaml.v3"
)

// Command is compleo's superset of carapace-spec's command.Command: it embeds
// command.Command for every carapace-spec field (name, flags, persistentflags,
// completion, run, ...), decoding and behaving exactly as carapace-spec itself would, and
// adds compleo's own extensions — positional argument specs and flag value types — for
// which carapace-spec has no fields at all: it only carries completion candidates and
// display text, never a name, required-ness, arity, or type.
//
// Commands can't just be left to the embedded command.Command's own Commands field: that
// always decodes to plain command.Command, with no room for Positionals/FlagTypes on
// subcommands. It's tagged `yaml:"-"` (invisible to the generic decoder, so it can't
// collide with the embedded field's own "commands" key) and filled in by UnmarshalYAML
// below, which re-decodes the same "commands" node into []Command instead.
type Command struct {
	command.Command `yaml:",inline"`

	Commands []Command `yaml:"-"`

	Positionals []Positional `yaml:"positionals,omitempty"`

	// FlagTypes maps a flag's bare name (as in command.Flag.Name, dashes and modifiers
	// stripped) to a compleo value type. carapace-spec's own Flag only distinguishes
	// string/[]string/bool/count; a flag with no entry here is a plain string.
	FlagTypes map[string]Type `yaml:"flagtypes,omitempty"`
}

// UnmarshalYAML decodes every carapace-spec field (via the embedded command.Command) and
// compleo's own Positionals/FlagTypes normally, then separately re-decodes the "commands"
// node — already consumed by the embedded command.Command.Commands field — into this
// Command's own Commands field, so subcommands carry compleo's extensions too.
func (c *Command) UnmarshalYAML(value *yaml.Node) error {
	type alias Command // same fields, no UnmarshalYAML method, to avoid infinite recursion
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
	c.Command.Commands = nil // superseded by c.Commands; avoid two divergent copies
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

// Type is a compleo value type used to coerce a flag's or positional's parsed string
// value. carapace/cobra parse every flag as string, []string, bool, or count — Type is
// never seen by cobra, only by compleo after parsing.
type Type string

const (
	TypeString   Type = ""
	TypeInt      Type = "int"
	TypeFloat    Type = "float"
	TypeBool     Type = "bool"
	TypeDuration Type = "duration"
	TypePath     Type = "path"
)

// ToCarapace converts the spec, recursively, into carapace-spec's command.Command,
// discarding compleo's own extensions (Positionals, FlagTypes). The result is ready for
// carapace-spec's spec.Command(cmd).ToCobraE().
func (c Command) ToCarapace() command.Command {
	cmd := c.Command
	cmd.Commands = make([]command.Command, len(c.Commands))
	for i, sub := range c.Commands {
		cmd.Commands[i] = sub.ToCarapace()
	}
	return cmd
}
