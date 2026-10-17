package bash

import "github.com/nesneros/compleo/internal/shells"

// Emitter is the bash implementation of shells.Emitter.
type Emitter struct{}

var _ shells.Emitter = Emitter{}
