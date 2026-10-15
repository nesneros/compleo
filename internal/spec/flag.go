package spec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/carapace-sh/carapace/pkg/command"
	yaml "gopkg.in/yaml.v3"
)

// Flag is a compleo spec's flag definition, with the same fields as carapace-sh/carapace's
// pkg/command.Flag (imported here as "command"), plus Type.
//
// https://github.com/carapace-sh/carapace/blob/v1.16.3/pkg/command/flag.go
type Flag struct {
	Longhand    string
	Shorthand   string
	Description string

	NameAsShorthand bool
	Repeatable      bool
	Optarg          bool
	Value           bool
	Hidden          bool
	Required        bool
	Persistent      bool

	Nargs   int
	Default string

	// Type validates the flag's value and describes it beyond pflag's own
	// string/[]string/bool/count.
	Type Type

	// Completion is this flag's own completion action expression (e.g. ["$files"], or a
	// plain list of literal values), folded into carapace's name-keyed completion map by
	// Command.toCarapace. An EnumType flag with no Completion falls back to its own Values.
	Completion []string
}

// Validate reports whether s is a valid value for f. A nil Type means no validation,
// equivalent to StringType.
func (f Flag) Validate(s string) error {
	if f.Type == nil {
		return nil
	}
	return f.Type.Validate(s)
}

// Name returns Longhand if set, else Shorthand — mirrors command.Flag.Name().
func (f Flag) Name() string {
	if f.Longhand != "" {
		return f.Longhand
	}
	return f.Shorthand
}

// toCarapace converts f into carapace's own Flag, appending Type's hint to the description as
// a backtick-quoted segment (e.g. "... (`int`)", or "... (`one|two|three`)" for an enum).
func (f Flag) toCarapace() command.Flag {
	desc := f.Description
	if f.Type != nil {
		if hint := f.Type.hint(); hint != "" {
			desc += " (`" + hint + "`)"
		}
	}
	return command.Flag{
		Longhand:        f.Longhand,
		Shorthand:       f.Shorthand,
		Description:     desc,
		NameAsShorthand: f.NameAsShorthand,
		Repeatable:      f.Repeatable,
		Optarg:          f.Optarg,
		Value:           f.Value,
		Hidden:          f.Hidden,
		Required:        f.Required,
		Persistent:      f.Persistent,
		Nargs:           f.Nargs,
		Default:         f.Default,
	}
}

// FlagSet mirrors carapace-sh/carapace's pkg/command.FlagSet.
//
// https://github.com/carapace-sh/carapace/blob/v1.16.3/pkg/command/flagset.go
type FlagSet map[string]Flag

// toCarapace converts every flag in fs.
func (fs FlagSet) toCarapace() command.FlagSet {
	if fs == nil {
		return nil
	}
	out := make(command.FlagSet, len(fs))
	for name, f := range fs {
		out[name] = f.toCarapace()
	}
	return out
}

// flagKeyPattern parses a flag's shorthand-and-modifier key syntax, e.g. "-c, --count=*!".
var flagKeyPattern = regexp.MustCompile(`^(?P<shorthand>-[^-][^ =*?&!]*)?(, )?(?P<longhand>-[-]?[^ =*?&!]*)?(?P<modifier>[=*?&!]*)$`)

// parseFlagKey parses s (a FlagSet map key) into a Flag with everything but
// Description/Nargs/Default/Type filled in.
func parseFlagKey(s string) (Flag, error) {
	match := flagKeyPattern.FindStringSubmatch(s)
	if match == nil {
		return Flag{}, fmt.Errorf("flag syntax invalid: %v", s)
	}
	groups := make(map[string]string, len(match))
	for i, name := range flagKeyPattern.SubexpNames() {
		if name != "" {
			groups[name] = match[i]
		}
	}

	longhand := groups["longhand"]
	modifier := groups["modifier"]
	optarg := strings.Contains(modifier, "?")
	f := Flag{
		Longhand:        strings.TrimLeft(longhand, "-"),
		Shorthand:       strings.TrimPrefix(groups["shorthand"], "-"),
		NameAsShorthand: longhand != "" && !strings.HasPrefix(longhand, "--"),
		Repeatable:      strings.Contains(modifier, "*"),
		Optarg:          optarg,
		Value:           optarg || strings.Contains(modifier, "="),
		Hidden:          strings.Contains(modifier, "&"),
		Required:        strings.Contains(modifier, "!"),
	}

	if f.Longhand == "" && f.Shorthand == "" {
		return Flag{}, fmt.Errorf("malformed flag: %q", s)
	}
	return f, nil
}

// validFlagObjectKeys are the only keys recognized in a flag's object-form value.
var validFlagObjectKeys = map[string]bool{"description": true, "nargs": true, "default": true, "type": true, "completion": true}

// UnmarshalYAML decodes a flags: entry. A bare string is the description; an object form
// additionally accepts nargs, default, and type.
func (fs *FlagSet) UnmarshalYAML(value *yaml.Node) error {
	m := make(map[string]any)
	if err := value.Decode(&m); err != nil {
		return err
	}

	flagSet := make(FlagSet, len(m))
	for k, v := range m {
		f, err := parseFlagKey(k)
		if err != nil {
			return err
		}
		f.Type = StringType{}

		switch v := v.(type) {
		case string:
			f.Description = v
		case map[string]any:
			for key := range v {
				if !validFlagObjectKeys[key] {
					return fmt.Errorf("flag %q: unknown key %q", k, key)
				}
			}
			f.Description, _ = v["description"].(string)
			f.Nargs, _ = v["nargs"].(int)
			switch d := v["default"].(type) {
			case string:
				f.Default = d
			case nil:
			default:
				f.Default = fmt.Sprint(d)
			}
			t, err := decodeType(v["type"])
			if err != nil {
				return fmt.Errorf("flag %q: %w", k, err)
			}
			f.Type = t
			if c, ok := v["completion"]; ok {
				completion, err := decodeStringList(c)
				if err != nil {
					return fmt.Errorf("flag %q: completion: %w", k, err)
				}
				f.Completion = completion
			}
		default:
			return fmt.Errorf("flag %q: invalid value", k)
		}

		flagSet[f.Name()] = f
	}
	*fs = flagSet
	return nil
}

// decodeStringList converts a generically-decoded YAML sequence (v is []any, each element
// already decoded to its own Go type) into a []string, erroring on any non-string element.
func decodeStringList(v any) ([]string, error) {
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected a list, got %T", v)
	}
	strs := make([]string, len(list))
	for i, elem := range list {
		s, ok := elem.(string)
		if !ok {
			return nil, fmt.Errorf("element %d: expected a string, got %T", i, elem)
		}
		strs[i] = s
	}
	return strs, nil
}
