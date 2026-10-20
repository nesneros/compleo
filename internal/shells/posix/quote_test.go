package posix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuotePlainString(t *testing.T) {
	assert.Equal(t, `'hello'`, Emitter{}.Quote("hello"))
}

func TestQuoteEmptyString(t *testing.T) {
	assert.Equal(t, `''`, Emitter{}.Quote(""))
}

func TestQuoteEscapesSingleQuote(t *testing.T) {
	assert.Equal(t, `'it'\''s'`, Emitter{}.Quote("it's"))
}
