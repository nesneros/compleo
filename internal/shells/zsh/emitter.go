package zsh

import (
	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/shells/posix"
)

// Emitter is the zsh implementation of shells.Emitter.
// Quote, EmitHelp, and EmitError come from the embedded posix.Emitter.
type Emitter struct {
	posix.Emitter
}

// Em is the zsh emitter.
var Em = Emitter{
	StdoutFormat: "print -r -- %s\n",
	StderrFormat: "print -u2 -- %s\n",
	Shell:        "zsh",
}

var _ shells.Emitter = Em
