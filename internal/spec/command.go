package spec

import (
	"fmt"

	carapacespec "github.com/carapace-sh/carapace-spec"
	"github.com/carapace-sh/carapace/pkg/command"
	yaml "gopkg.in/yaml.v3"
)

// Command is a compleo spec's command definition, with the same field names and YAML tags as
// carapace-sh/carapace's pkg/command.Command (imported here as "command"), plus Positionals.
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
	Commands        []Command   `yaml:"commands,omitempty"`

	Documentation Documentation     `yaml:"documentation,omitempty"`
	Examples      map[string]string `yaml:"examples,omitempty"`

	Positionals []Positional `yaml:"positionals,omitempty"`
}

// Documentation holds extra help text for a command, keyed by flag name or positional index.
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

	// Completion is this positional's own completion action expression (e.g. ["$files"]),
	// folded into carapace's index-keyed completion list by Command.toCarapace.
	Completion []string
}

// Validate reports whether s is a valid value for p. A nil Type means no validation,
// equivalent to StringType.
func (p Positional) Validate(s string) error {
	if p.Type == nil {
		return nil
	}
	return p.Type.Validate(s)
}

// validPositionalKeys are the only keys recognized on a positionals: entry.
var validPositionalKeys = map[string]bool{
	"name": true, "description": true, "required": true, "variadic": true, "type": true, "completion": true,
}

// UnmarshalYAML decodes a positionals: entry, rejecting any key not in validPositionalKeys.
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

	if c, ok := m["completion"]; ok {
		completion, err := decodeStringList(c)
		if err != nil {
			return fmt.Errorf("positional %q: completion: %w", p.Name, err)
		}
		p.Completion = completion
	}
	return nil
}

// toCarapace recursively converts c into carapace-spec's Command. Positionals has no
// carapace-spec equivalent and is not included.
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
	out.Completion.Flag = flagCompletions(c.Flags, c.PersistentFlags)
	positionalCompletion, variadicFallback := positionalCompletions(c.Positionals)
	out.Completion.Positional = positionalCompletion
	// A trailing variadic positional's own completion is the only way a spec can still
	// influence this; otherwise left unset, carapace falls through to a cobra-compat bridge
	// path whose completion is effectively undefined for a position the spec never declared
	// — offer nothing explicitly rather than risk a surprise (e.g. file candidates).
	out.Completion.PositionalAny = variadicFallback
	if out.Completion.PositionalAny == nil {
		out.Completion.PositionalAny = noPositionalCompletion
	}
	out.Documentation.Command = c.Documentation.Command
	out.Documentation.Flag = c.Documentation.Flag
	out.Documentation.Positional = c.Documentation.Positional
	out.Documentation.PositionalAny = c.Documentation.PositionalAny
	out.Documentation.Dash = c.Documentation.Dash
	out.Documentation.DashAny = c.Documentation.DashAny
	return carapacespec.Command(out)
}

// noPositionalCompletion is the action expression used to explicitly offer nothing for a
// position no completion was declared for (see $compleo.none, internal/spec/macros.go).
var noPositionalCompletion = []string{"$compleo.none"}

// flagCompletions gathers each flag's own Completion across flagSets into carapace's
// name-keyed completion map. An enum flag with no Completion of its own falls back to its
// own Values — so declaring the list once gets both validation and Tab-completion.
func flagCompletions(flagSets ...FlagSet) map[string][]string {
	var out map[string][]string
	for _, fs := range flagSets {
		for name, f := range fs {
			completion := f.Completion
			if completion == nil {
				if et, ok := f.Type.(EnumType); ok && len(et.Values) > 0 {
					completion = et.Values
				}
			}
			if completion == nil {
				continue
			}
			if out == nil {
				out = make(map[string][]string, len(fs))
			}
			out[name] = completion
		}
	}
	return out
}

// positionalCompletions builds carapace's index-keyed completion list from each positional's
// own Completion (falling back to an enum positional's own Values, same as a flag), and
// separately reports the trailing variadic positional's own completion, if any, as a
// candidate completion.positionalany fallback.
//
// The returned list is trimmed of trailing entries with no completion (nil, not just empty),
// returning nil outright if none exist: carapace's own getPositional treats any list entry at
// all — even one with zero actions — as an actual completion for that index, shadowing
// completion.positionalany/the command's default for it; a command with positionals declared
// but none completed must not silently shadow positionalany/the default for every index just
// because positionals: exists. A gap *before* a later declared completion still can't be
// avoided (same as plain carapace-spec's own completion.positional has always required) — only
// a trailing run of undeclared positionals is trimmed.
func positionalCompletions(positionals []Positional) (indexed [][]string, variadicFallback []string) {
	if len(positionals) == 0 {
		return nil, nil
	}

	completions := make([][]string, len(positionals))
	lastDeclared := -1
	for i, p := range positionals {
		completion := p.Completion
		if completion == nil {
			if et, ok := p.Type.(EnumType); ok && len(et.Values) > 0 {
				completion = et.Values
			}
		}
		completions[i] = completion
		if completion != nil {
			lastDeclared = i
		}
	}

	// A variadic positional's own completion.positional[i] entry only ever covers its first
	// word — carapace has no concept of "repeat this index's action" — so without also using
	// it as positionalany, every word after the first would silently get no completion at all.
	if positionals[len(positionals)-1].Variadic {
		variadicFallback = completions[len(positionals)-1]
	}

	if lastDeclared == -1 {
		return nil, variadicFallback
	}
	return completions[:lastDeclared+1], variadicFallback
}
