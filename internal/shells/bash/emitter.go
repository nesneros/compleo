package bash

import (
	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/shells/posix"
)

// Emitter is the bash implementation of shells.Emitter.
// Quote, EmitHelp, and EmitError come from the embedded posix.Emitter.
type Emitter struct {
	posix.Emitter
}

// Em is the bash emitter.
var Em = Emitter{
	StdoutFormat: "printf '%%s\\n' %s\n",
	StderrFormat: "printf '%%s\\n' %s >&2\n",
	Shell:        "bash",
}

var _ shells.Emitter = Em
