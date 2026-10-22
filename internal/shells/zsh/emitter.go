package zsh

import (
	"io"

	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/shells/posix"
)

// Emitter is the zsh implementation of shells.Emitter.
// Quote, EmitHelp, and EmitError come from the embedded posix.Emitter.
type Emitter struct {
	posix.Emitter
}

// Em is the zsh emitter, unbound to any stream — only its Quote/Name (and, for tests,
// WrappedCompletion/ActivationHint, which don't write anywhere) are meaningful to call on it
// directly. New returns one bound to real streams.
var Em = Emitter{
	StdoutFormat: "print -r -- %s\n",
	StderrFormat: "print -u2 -- %s\n",
	Shell:        "zsh",
}

var _ shells.Emitter = Em

// New returns a zsh Emitter whose Emit* methods write to out (and, if ever needed, err).
func New(out, err io.Writer) shells.Emitter {
	e := Em
	e.Out = out
	e.Err = err
	return e
}

func init() {
	shells.RegisterEmitterFactory("zsh", New)
}
