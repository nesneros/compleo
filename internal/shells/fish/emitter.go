// Package fish completes in an interactive fish shell. It is not an eval shell:
// a wrapped script is still zsh or bash.
package fish

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/shells/posix"
)

// Emitter writes fish completion wiring. Fish is not a POSIX shell, so this does not
// embed posix.Emitter (that would also expose EmitHelp and EmitError).
type Emitter struct {
	out io.Writer
}

// Em is the fish emitter, unbound to any stream. New returns one bound to real streams.
var Em Emitter

var _ shells.CompletionEmitter = Em

// Name returns "fish".
func (Emitter) Name() string { return "fish" }

// Quote returns s as a single-quoted word. Fish has no escape inside single quotes;
// an embedded ' is closed, escaped, and reopened, which fish parses as concatenation.
func (Emitter) Quote(s string) string { return posix.Quote(s) }

// New returns a fish emitter whose EmitActivate writes to out.
func New(out, _ io.Writer) shells.CompletionEmitter {
	return Emitter{out: out}
}

func init() {
	shells.RegisterCompletionEmitterFactory("fish", New)
}

// CompletionFileName is the name fish autoloads: "<command>.fish" on fish_complete_path.
func (Emitter) CompletionFileName(name string) string { return name + ".fish" }

// ActivationHint is the command that loads name's completion in this fish session.
func (e Emitter) ActivationHint(name string) string {
	path := filepath.Join(cmdutil.CompletionDir(), e.CompletionFileName(name))
	return "source " + e.Quote(path)
}

// activateTemplate exports the interactive shell, defines a compleo function, and adds dir
// to fish_complete_path so fish autoloads <command>.fish from it.
const activateTemplate = `set -gx %[2]s %[3]s
function compleo
  command compleo $argv
end
set -l __compleo_dir %[1]s
if not contains -- $__compleo_dir $fish_complete_path
  set -gp fish_complete_path $__compleo_dir
end
`

// EmitActivate writes fish that records the completion shell and searches dir.
func (e Emitter) EmitActivate(dir string) error {
	_, err := fmt.Fprintf(e.out, activateTemplate, e.Quote(dir), cmdutil.EnvCompletionShell, e.Quote(e.Name()))
	return err
}
