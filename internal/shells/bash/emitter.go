package bash

import (
	"io"

	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/shells/posix"
)

// Emitter is the bash implementation of shells.Emitter.
// Quote, EmitHelp, and EmitError come from the embedded posix.Emitter.
type Emitter struct {
	posix.Emitter
}

// Em is the bash emitter, unbound to any stream — only its Quote/Name (and, for tests,
// WrappedCompletion/ActivationHint, which don't write anywhere) are meaningful to call on it
// directly. New returns one bound to real streams.
var Em = Emitter{
	StdoutFormat: "printf '%%s\\n' %s\n",
	StderrFormat: "printf '%%s\\n' %s >&2\n",
	Shell:        "bash",
}

var _ shells.Emitter = Em

// New returns a bash Emitter whose Emit* methods write to out (and, if ever needed, err).
func New(out, err io.Writer) shells.Emitter {
	e := Em
	e.Out = out
	e.Err = err
	return e
}

func init() {
	shells.RegisterEmitterFactory("bash", New)
}
