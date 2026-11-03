// Package nu completes in an interactive Nushell and emits a nuon record a wrapped
// script reads with `from nuon`. Nu resolves `source` before the script runs, so
// generated code cannot be sourced in the same run. The process name is "nu";
// carapace calls the snippet shell "nushell".
package nu

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/shells"
)

// Emitter writes the nuon record a script reads, and nu completion wiring.
// Nushell is not a POSIX shell, so this does not embed posix.Emitter.
type Emitter struct {
	out io.Writer
}

// Em is the nu emitter, unbound to any stream. New returns one bound to real streams.
var Em Emitter

var _ shells.Emitter = Em

// Name returns "nu", the process name ParentProcessName reports.
func (Emitter) Name() string { return "nu" }

// nuQuote escapes Nushell double-quoted strings. Replace does not mutate it.
var nuQuote = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`", "\n", `\n`, "\r", `\r`)

// Quote returns s as a Nushell double-quoted string.
func (Emitter) Quote(s string) string {
	return `"` + nuQuote.Replace(s) + `"`
}

// New returns a nu emitter whose output methods write to out.
func New(out, _ io.Writer) shells.Emitter {
	return Emitter{out: out}
}

func init() {
	shells.RegisterEmitterFactory("nu", New)
	// Carapace's own name for this shell. Same emitter; Name() stays "nu".
	shells.RegisterCompletionEmitterFactory("nushell", func(out, err io.Writer) shells.CompletionEmitter {
		return New(out, err)
	})
}

// CompletionFileName is "<command>.nu" in the shared completion directory.
func (Emitter) CompletionFileName(name string) string { return name + ".nu" }

// ActivationHint is the command that loads name's completion into this session.
// Nushell has one external completer, so this registers name; activate installs the dispatcher that calls it.
func (e Emitter) ActivationHint(name string) string {
	path := filepath.Join(cmdutil.CompletionDir(), e.CompletionFileName(name))
	return "source " + e.Quote(path)
}

// activateTemplate exports the completion shell and installs one external completer.
// Nushell's `source` only accepts a parse-time constant path, so this cannot loop over
// the directory. The dispatcher reads each command's spec file instead. Sourcing one
// "<name>.nu" (the activation hint) still registers that command's closure.
const activateTemplate = `$env.%[1]s = %[2]s
$env._COMPLEO_COMPLETION_DIR = %[3]s
$env._COMPLEO_NU_EXE = %[4]s
def --env compleo [...args] { ^compleo ...$args }
do {
  let already = (try { $env._COMPLEO_NU_DISPATCHER } catch { false })
  if not $already {
    $env._COMPLEO_NU_DISPATCHER = true
    let prev_completer = (try { $env.config.completions.external.completer } catch { null })
    let dispatcher = {|place|
      let spans = (if ($place | describe) =~ "record" { $place.command } else { $place })
      let cmd = (try { $spans | get 0 } catch { "" })
      let registered = (try { $env._COMPLEO_NU_COMPLETERS } catch { {} })
      if ($cmd != "") and ($cmd in ($registered | columns)) {
        do ($registered | get $cmd) $place
      } else {
        let spec = ($env._COMPLEO_COMPLETION_DIR | path join $".($cmd).spec.yaml")
        if ($cmd == "compleo") or (($cmd != "") and ($spec | path exists)) {
          if ($spec | path exists) {
            with-env { %[5]s: $spec } { ^($env._COMPLEO_NU_EXE) _carapace nushell ...$spans | from json }
          } else {
            ^($env._COMPLEO_NU_EXE) _carapace nushell ...$spans | from json
          }
        } else if $prev_completer != null {
          do $prev_completer $place
        } else {
          null
        }
      }
    }
    mut current = (($env | default {} config).config | default {} completions)
    $current.completions = ($current.completions | default {} external)
    # A bare closure is run by upsert; the block returns it so the closure is stored.
    $current.completions.external = ($current.completions.external
      | default true enable
      | upsert completer { $dispatcher })
    $env.config = $current
  }
}
`

// EmitActivate writes nu that records the completion shell and installs the dispatcher.
func (e Emitter) EmitActivate(dir string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("nu activate: %w", err)
	}
	_, err = fmt.Fprintf(e.out, activateTemplate,
		cmdutil.EnvCompletionShell, e.Quote(e.Name()), e.Quote(dir), e.Quote(exe), cmdutil.EnvSpecFile)
	return err
}
