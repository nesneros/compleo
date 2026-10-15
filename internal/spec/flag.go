package spec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/carapace-sh/carapace/pkg/command"
	yaml "gopkg.in/yaml.v3"
)

// Flag mirrors carapace-sh/carapace's pkg/command.Flag (imported here as "command") — same
// fields, duplicated rather than embedded, so Type (and any future per-flag extension) lives
// directly on it, specified together with the rest of the flag's own definition, instead of in
// a side-channel map keyed by flag name alongside Flags. Keep this in sync by hand if
// command.Flag ever changes; nothing enforces it automatically.
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

	// Type coerces the flag's parsed value; cobra/pflag only ever register it as a plain
	// string/[]string/bool/count, so it's validated separately (see internal/parse). See
	// type.go. toCarapace also uses an EnumType's Values to auto-populate this flag's
	// completion, when none was explicitly declared.
	Type Type
}

// Validate reports whether s is a valid value for f. A nil Type (a zero-value Flag
// constructed outside of decoding, e.g. in a test, or a name with no entry in a FlagSet map)
// means no validation, same as StringType.
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

// toCarapace converts f into carapace's own Flag for ToCobraE to register, folding Type into
// a backtick-quoted description hint (e.g. "... (`int`)", or "... (`one|two|three`)" for an
// enum) — pflag's UnquoteUsage prefers a backtick-quoted segment in the description over the
// flag's actual (always string) type.
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

// flagKeyPattern parses a flag's shorthand-and-modifier key syntax (e.g. "-c, --count=*!")
// — duplicated from carapace's own (unexported) command.parseFlag, since compleo decodes
// into its own Flag struct instead of carapace's.
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
var validFlagObjectKeys = map[string]bool{"description": true, "nargs": true, "default": true, "type": true}

// UnmarshalYAML duplicates carapace's command.FlagSet.UnmarshalYAML (a bare string is the
// description; an object form additionally allows nargs/default), plus a "type:" key (see
// type.go's decodeType for its own shorthand-string-or-object duality).
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
