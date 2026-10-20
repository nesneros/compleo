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

	shells.SetCompletionEmitter(zsh.Em)
	shells.SetEvalEmitter(nil)
	assert.Equal(t, zsh.Em, shells.GetCompletionEmitter())
	assert.Nil(t, shells.GetEvalEmitter())

	shells.SetEvalEmitter(zsh.Em)
	shells.SetCompletionEmitter(nil)
	assert.Nil(t, shells.GetCompletionEmitter())
	assert.Equal(t, zsh.Em, shells.GetEvalEmitter())
}
