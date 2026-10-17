package shells_test

import (
	"testing"

	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/shells/zsh"
	"github.com/stretchr/testify/assert"
)

func TestCompletionAndEvalEmittersAreIndependent(t *testing.T) {
	t.Cleanup(func() {
		shells.SetCompletionEmitter(nil)
		shells.SetEvalEmitter(nil)
	})

	shells.SetCompletionEmitter(zsh.Emitter{})
	shells.SetEvalEmitter(nil)
	assert.Equal(t, zsh.Emitter{}, shells.CompletionEmitter())
	assert.Nil(t, shells.EvalEmitter())

	shells.SetEvalEmitter(zsh.Emitter{})
	shells.SetCompletionEmitter(nil)
	assert.Nil(t, shells.CompletionEmitter())
	assert.Equal(t, zsh.Emitter{}, shells.EvalEmitter())
}
