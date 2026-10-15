package spec

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Type coerces a flag's or positional's parsed string value; cobra never sees it. One
// concrete implementation per kind, each owning only the attributes that kind needs —
// deliberately not a single flat struct with every kind's fields bolted on regardless of
// which kind is actually in use (an enum's Values next to an int's Min/Max, etc.).
type Type interface {
	// Validate reports whether s is a valid value of this type: cobra/pflag only ever
	// parse it as a plain string, so this is the only place an invalid value is caught.
	Validate(s string) error

	// hint is what --help's backtick-quoted type hint shows (e.g. "int", "red|green|blue"),
	// or "" for the default (StringType), which shows no hint at all.
	hint() string
}

// StringType is the default: no extra validation, no --help hint.
type StringType struct{}

func (StringType) Validate(string) error { return nil }
func (StringType) hint() string          { return "" }

// IntType requires strconv.Atoi to succeed, optionally bounded by Min/Max (either may be nil).
type IntType struct {
	Min, Max *int
}

func (t IntType) Validate(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("invalid int value %q", s)
	}
	if err := checkRange(n, t.Min, t.Max); err != nil {
		return fmt.Errorf("invalid int value %d: %w", n, err)
	}
	return nil
}
func (t IntType) hint() string { return rangeHint("int", t.Min, t.Max) }

// FloatType requires strconv.ParseFloat to succeed, optionally bounded by Min/Max.
type FloatType struct {
	Min, Max *float64
}

func (t FloatType) Validate(s string) error {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("invalid float value %q", s)
	}
	if err := checkRange(n, t.Min, t.Max); err != nil {
		return fmt.Errorf("invalid float value %g: %w", n, err)
	}
	return nil
}
func (t FloatType) hint() string { return rangeHint("float", t.Min, t.Max) }

// checkRange reports whether v is within [min, max] (either may be nil, meaning unbounded) —
// shared by IntType and FloatType, whose Validate otherwise differs only in how the string is
// parsed and how a value prints in an error message.
func checkRange[T cmp.Ordered](v T, min, max *T) error {
	if min != nil && v < *min {
		return fmt.Errorf("must be >= %v", *min)
	}
	if max != nil && v > *max {
		return fmt.Errorf("must be <= %v", *max)
	}
	return nil
}

// rangeHint renders kind plus any bounds (e.g. "int 1-65535", "int >=1", "int" alone when
// neither is set) — shared by IntType and FloatType's own hint().
func rangeHint[T cmp.Ordered](kind string, min, max *T) string {
	switch {
	case min != nil && max != nil:
		return fmt.Sprintf("%s %v-%v", kind, *min, *max)
	case min != nil:
		return fmt.Sprintf("%s >=%v", kind, *min)
	case max != nil:
		return fmt.Sprintf("%s <=%v", kind, *max)
	default:
		return kind
	}
}

// BoolType requires strconv.ParseBool to succeed.
type BoolType struct{}

func (BoolType) Validate(s string) error {
	if _, err := strconv.ParseBool(s); err != nil {
		return fmt.Errorf("invalid bool value %q", s)
	}
	return nil
}
func (BoolType) hint() string { return "bool" }

// DurationType requires time.ParseDuration to succeed.
type DurationType struct{}

func (DurationType) Validate(s string) error {
	if _, err := time.ParseDuration(s); err != nil {
		return fmt.Errorf("invalid duration value %q", s)
	}
	return nil
}
func (DurationType) hint() string { return "duration" }

// PathType is accepted like a string — no extra validation yet (room to grow: e.g. Exists,
// Must — see CLAUDE.md's open questions).
type PathType struct{}

func (PathType) Validate(string) error { return nil }
func (PathType) hint() string          { return "path" }

// EnumType requires s to be one of Values.
type EnumType struct {
	Values []string
}

func (t EnumType) Validate(s string) error {
	if len(t.Values) == 0 {
		return fmt.Errorf("type enum declared with no values")
	}
	if !slices.Contains(t.Values, s) {
		return fmt.Errorf("invalid value %q: must be one of %s", s, strings.Join(t.Values, ", "))
	}
	return nil
}
func (t EnumType) hint() string { return strings.Join(t.Values, "|") }

// validTypeObjectKeys are the only keys recognized in a "type:" object-form value.
var validTypeObjectKeys = map[string]bool{"kind": true, "min": true, "max": true, "values": true}

// decodeType builds a Type from a flag's or positional's own "type:" value, as already
// generically decoded (so v is nil, a string, or a map[string]any). A bare string (e.g.
// "int") is shorthand for the object form with no extra attributes ({kind: int}); the object
// form additionally allows kind-specific attributes (min/max, values).
func decodeType(v any) (Type, error) {
	switch v := v.(type) {
	case nil:
		return StringType{}, nil
	case string:
		return typeFromKind(v, nil)
	case map[string]any:
		for key := range v {
			if !validTypeObjectKeys[key] {
				return nil, fmt.Errorf("type: unknown key %q", key)
			}
		}
		kind, _ := v["kind"].(string)
		return typeFromKind(kind, v)
	default:
		return nil, fmt.Errorf("invalid type: %#v", v)
	}
}

// typeFromKind builds the concrete Type for kind, reading any further attributes from attrs
// (nil for the bare-string shorthand, equivalent to {kind: <kind>} with nothing else).
func typeFromKind(kind string, attrs map[string]any) (Type, error) {
	switch kind {
	case "", "string":
		return StringType{}, nil
	case "int":
		min, err := intAttr(attrs, "min")
		if err != nil {
			return nil, err
		}
		max, err := intAttr(attrs, "max")
		if err != nil {
			return nil, err
		}
		return IntType{Min: min, Max: max}, nil
	case "float":
		min, err := floatAttr(attrs, "min")
		if err != nil {
			return nil, err
		}
		max, err := floatAttr(attrs, "max")
		if err != nil {
			return nil, err
		}
		return FloatType{Min: min, Max: max}, nil
	case "bool":
		return BoolType{}, nil
	case "duration":
		return DurationType{}, nil
	case "path":
		return PathType{}, nil
	case "enum":
		var values []string
		if raw, ok := attrs["values"]; ok {
			v, err := decodeStringList(raw)
			if err != nil {
				return nil, fmt.Errorf("values: %w", err)
			}
			values = v
		}
		return EnumType{Values: values}, nil
	default:
		return nil, fmt.Errorf("unknown type kind %q", kind)
	}
}

func intAttr(attrs map[string]any, key string) (*int, error) {
	raw, ok := attrs[key]
	if !ok {
		return nil, nil
	}
	n, ok := raw.(int)
	if !ok {
		return nil, fmt.Errorf("%s: expected an int, got %T", key, raw)
	}
	return &n, nil
}

func floatAttr(attrs map[string]any, key string) (*float64, error) {
	raw, ok := attrs[key]
	if !ok {
		return nil, nil
	}
	switch v := raw.(type) {
	case float64:
		return &v, nil
	case int:
		f := float64(v)
		return &f, nil
	default:
		return nil, fmt.Errorf("%s: expected a number, got %T", key, raw)
	}
}
