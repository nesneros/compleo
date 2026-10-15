package spec

import (
	"fmt"
	"maps"

	carapacespec "github.com/carapace-sh/carapace-spec"
	"github.com/carapace-sh/carapace/pkg/command"
	yaml "gopkg.in/yaml.v3"
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

// Positional describes one named positional argument. See type.go for Type.
type Positional struct {
	Name        string
	Description string
	Required    bool
	Variadic    bool
	Type        Type
}

// Validate reports whether s is a valid value for p. A nil Type (a zero-value Positional
// constructed outside of decoding, e.g. in a test) means no validation, same as StringType.
func (p Positional) Validate(s string) error {
	if p.Type == nil {
		return nil
	}
	return p.Type.Validate(s)
}

// validPositionalKeys are the only keys recognized on a positionals: entry.
var validPositionalKeys = map[string]bool{
	"name": true, "description": true, "required": true, "variadic": true, "type": true,
}

// UnmarshalYAML decodes into a map first (like FlagSet does), rather than relying on plain
// struct tags, so an unrecognized key is still caught (see type.go's decodeType) and so Type
// — an interface, with no natural YAML shape of its own — can be built explicitly.
func (p *Positional) UnmarshalYAML(value *yaml.Node) error {
	m := make(map[string]any)
	if err := value.Decode(&m); err != nil {
		return err
	}
	for key := range m {
		if !validPositionalKeys[key] {
			return fmt.Errorf("positional: unknown key %q", key)
		}
	}

	p.Name, _ = m["name"].(string)
	p.Description, _ = m["description"].(string)
	p.Required, _ = m["required"].(bool)
	p.Variadic, _ = m["variadic"].(bool)

	t, err := decodeType(m["type"])
	if err != nil {
		return fmt.Errorf("positional %q: %w", p.Name, err)
	}
	p.Type = t
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
			et, ok := f.Type.(EnumType)
			if !ok || len(et.Values) == 0 {
				continue
			}
			if _, declared := completion[name]; declared {
				continue
			}
			if out == nil {
				out = make(map[string][]string, len(completion))
				maps.Copy(out, completion)
			}
			out[name] = et.Values
		}
	}
	if out != nil {
		return out
	}
	return completion
}
