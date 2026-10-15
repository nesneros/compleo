package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntTypeValidateAcceptsValidValue(t *testing.T) {
	assert.NoError(t, IntType{}.Validate("3"))
}

func TestIntTypeValidateRejectsNonNumeric(t *testing.T) {
	assert.Error(t, IntType{}.Validate("abc"))
}

func TestIntTypeValidateAcceptsValueWithinMinMax(t *testing.T) {
	min, max := 1, 10
	assert.NoError(t, IntType{Min: &min, Max: &max}.Validate("5"))
}

func TestIntTypeValidateRejectsValueBelowMin(t *testing.T) {
	min := 1
	assert.Error(t, IntType{Min: &min}.Validate("0"))
}

func TestIntTypeValidateRejectsValueAboveMax(t *testing.T) {
	max := 10
	assert.Error(t, IntType{Max: &max}.Validate("11"))
}

func TestIntTypeValidateAcceptsMinAndMaxBoundaryValuesInclusive(t *testing.T) {
	min, max := 1, 10
	assert.NoError(t, IntType{Min: &min, Max: &max}.Validate("1"))
	assert.NoError(t, IntType{Min: &min, Max: &max}.Validate("10"))
}

func TestIntTypeHintWithNoBoundsIsJustTheKindName(t *testing.T) {
	assert.Equal(t, "int", IntType{}.hint())
}

func TestIntTypeHintIncludesBothBounds(t *testing.T) {
	min, max := 1, 65535
	assert.Equal(t, "int 1-65535", IntType{Min: &min, Max: &max}.hint())
}

func TestIntTypeHintIncludesMinOnly(t *testing.T) {
	min := 1
	assert.Equal(t, "int >=1", IntType{Min: &min}.hint())
}

func TestIntTypeHintIncludesMaxOnly(t *testing.T) {
	max := 65535
	assert.Equal(t, "int <=65535", IntType{Max: &max}.hint())
}

func TestFloatTypeValidateRejectsNonNumeric(t *testing.T) {
	assert.Error(t, FloatType{}.Validate("abc"))
}

func TestFloatTypeValidateAcceptsValueWithinMinMax(t *testing.T) {
	min, max := 0.0, 1.5
	assert.NoError(t, FloatType{Min: &min, Max: &max}.Validate("0.75"))
}

func TestFloatTypeValidateRejectsValueBelowMin(t *testing.T) {
	min := 0.0
	assert.Error(t, FloatType{Min: &min}.Validate("-0.1"))
}

func TestFloatTypeValidateRejectsValueAboveMax(t *testing.T) {
	max := 1.5
	assert.Error(t, FloatType{Max: &max}.Validate("1.6"))
}

func TestFloatTypeHintIncludesBothBounds(t *testing.T) {
	min, max := 0.0, 1.5
	assert.Equal(t, "float 0-1.5", FloatType{Min: &min, Max: &max}.hint())
}

func TestBoolTypeValidateRejectsNonBool(t *testing.T) {
	assert.Error(t, BoolType{}.Validate("maybe"))
}

func TestBoolTypeValidateAcceptsValidValue(t *testing.T) {
	assert.NoError(t, BoolType{}.Validate("true"))
}

func TestDurationTypeValidateRejectsInvalid(t *testing.T) {
	assert.Error(t, DurationType{}.Validate("soon"))
}

func TestDurationTypeValidateAcceptsValid(t *testing.T) {
	assert.NoError(t, DurationType{}.Validate("5m"))
}

func TestStringAndPathTypeValidateAcceptAnything(t *testing.T) {
	assert.NoError(t, StringType{}.Validate("anything at all"))
	assert.NoError(t, PathType{}.Validate("anything at all"))
}

func TestEnumTypeValidateAcceptsListedValue(t *testing.T) {
	assert.NoError(t, EnumType{Values: []string{"red", "green", "blue"}}.Validate("green"))
}

func TestEnumTypeValidateRejectsUnlistedValue(t *testing.T) {
	assert.Error(t, EnumType{Values: []string{"red", "green", "blue"}}.Validate("purple"))
}

// A spec author declaring type: enum with no values at all is a mistake worth failing loudly
// on, rather than silently accepting every value (which would hide the mistake).
func TestEnumTypeValidateWithNoValuesAlwaysRejects(t *testing.T) {
	assert.Error(t, EnumType{}.Validate("anything"))
}

func TestDecodeTypeOmittedDefaultsToStringType(t *testing.T) {
	ty, err := decodeType(nil)
	assert.NoError(t, err)
	assert.Equal(t, StringType{}, ty)
}

func TestDecodeTypeBareStringIsShorthandForKindOnlyObject(t *testing.T) {
	bare, err := decodeType("int")
	assert.NoError(t, err)
	object, err := decodeType(map[string]any{"kind": "int"})
	assert.NoError(t, err)
	assert.Equal(t, object, bare)
}

func TestDecodeTypeRejectsUnknownKind(t *testing.T) {
	_, err := decodeType("bogus")
	assert.Error(t, err)
}

func TestDecodeTypeRejectsUnknownObjectKey(t *testing.T) {
	_, err := decodeType(map[string]any{"kind": "int", "maxx": 5})
	assert.Error(t, err)
}

func TestDecodeTypeIntMinMustBeAnInt(t *testing.T) {
	_, err := decodeType(map[string]any{"kind": "int", "min": "not-a-number"})
	assert.Error(t, err)
}

func TestDecodeTypeFloatMinAcceptsWholeNumberAsInt(t *testing.T) {
	// YAML "min: 0" decodes to a Go int, not a float64 — FloatType must accept either.
	ty, err := decodeType(map[string]any{"kind": "float", "min": 0})
	assert.NoError(t, err)
	min := 0.0
	assert.Equal(t, FloatType{Min: &min}, ty)
}
