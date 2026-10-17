package zsh

import "github.com/nesneros/compleo/internal/shells"

// Emitter is the zsh implementation of shells.Emitter.
type Emitter struct{}

var _ shells.Emitter = Emitter{}
