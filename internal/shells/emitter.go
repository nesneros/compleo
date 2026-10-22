package shells

import (
	"cmp"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/parse"
	"github.com/spf13/cobra"
)

// EvalEmitter writes code a wrapped script evals, to the stream it was constructed with.
type EvalEmitter interface {
	// EmitResult writes the parsed flag and positional values.
	EmitResult(result parse.Result) error
	// EmitHelp writes text the script prints, then exits 0.
	EmitHelp(text string) error
	// EmitError writes the parse error to stderr and exits 2.
	EmitError(name string, parseErr error) error
	// EmitUp writes the compleo parse invocation the script evals.
	EmitUp(rawSpec string) error
	// EmitGenerated writes the generate report: hint (from the completion-target shell's own
	// ActivationHint, which may be a different shell than this one — see CompletionEmitter)
	// plus any missing-binary warning, in this shell's own syntax, then exits 0.
	EmitGenerated(hint string, missing []string) error

	// Quote returns s quoted for this shell.
	Quote(s string) string
	// Name returns this shell's name.
	Name() string
}

// CompletionEmitter writes the interactive shell, where Tab is pressed.
type CompletionEmitter interface {
	// WrappedCompletion returns a wrapped script's completion snippet, with specFilePath embedded for Tab presses.
	WrappedCompletion(cmd *cobra.Command, name string, specFilePath string) (string, error)
	// EmitActivate wires the completions directory into the current shell.
	EmitActivate(dir string) error
	// ActivationHint is the human-readable instruction for activating name's completion in
	// this shell right now (e.g. zsh's "autoload -Uz _name && compdef ..."), for EmitGenerated
	// to report — in the script's own shell's syntax, which may differ from this one.
	ActivationHint(name string) string

	// Quote returns s quoted for this shell.
	Quote(s string) string
	// Name returns this shell's name.
	Name() string
}

// Emitter is an EvalEmitter and a CompletionEmitter.
// zsh.Emitter and bash.Emitter both implement it and register a factory for it.
type Emitter interface {
	EvalEmitter
	CompletionEmitter
}

// EmitterFactory constructs an Emitter bound to out (what every Emit* method writes the
// shell code to be eval'd to) and err (the real stderr, for a method that needs to bypass
// eval — see posix.Emitter's own doc comment; unused by any current method).
type EmitterFactory func(out, err io.Writer) Emitter

var registeredFactories map[string]EmitterFactory

func init() {
	registeredFactories = make(map[string]EmitterFactory)
}

// RegisterEmitterFactory registers factory under name (what Name() returns on the Emitter it
// constructs), for later lookup by EvalEmitterForShell/CompletionEmitterForShell.
func RegisterEmitterFactory(name string, factory EmitterFactory) {
	registeredFactories[name] = factory
}

// EvalEmitterForShell constructs the registered Emitter for shell, bound to out/err, or
// returns nil if no shell by that name is registered.
func EvalEmitterForShell(shell string, out, err io.Writer) EvalEmitter {
	factory := registeredFactories[shell]
	if factory == nil {
		return nil
	}
	return factory(out, err)
}

// CompletionEmitterForShell constructs the registered Emitter for shell, bound to out/err.
func CompletionEmitterForShell(shell string, out, err io.Writer) (CompletionEmitter, error) {
	factory := registeredFactories[shell]
	if factory == nil {
		return nil, fmt.Errorf("unsupported shell: %q", shell)
	}
	return factory(out, err), nil
}

// GetCompletionEmitter returns the interactive-shell emitter named by EnvCompletionShell,
// bound to out/err.
func GetCompletionEmitter(out, err io.Writer) CompletionEmitter {
	shell := os.Getenv(cmdutil.EnvCompletionShell)
	emitter, resolveErr := CompletionEmitterForShell(shell, out, err)
	if resolveErr != nil {
		cmdutil.Panic(fmt.Sprintf("Is compleo activated in the current shell? [%s]", resolveErr.Error()))
	}
	return emitter
}

// SortedKeys returns m's keys sorted, for deterministic output despite map iteration order.
func SortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	return slices.Sorted(maps.Keys(m))
}

// ResolveEvalEmitter returns the EvalEmitter for shell (bound to out/err), or, when shell is
// empty, for the wrapping script's own interpreter (cmdutil.ParentInterpreter — the direct
// parent of a "parse"/"up" invocation, always that script's shell: eval "$(...)" forks it
// straight into compleo, same argument as cmdutil.ParentProcessName's own doc comment).
// Returns an error describing what went wrong (detection failure or an unregistered shell
// name) rather than an EvalEmitter when resolution fails — never guesses zsh as a silent
// default, since that's exactly the wrong-shell-output bug this exists to avoid.
func ResolveEvalEmitter(shell string, out, err io.Writer) (EvalEmitter, error) {
	if shell == "" {
		detected, detectErr := cmdutil.ParentInterpreter()
		if detectErr != nil {
			return nil, fmt.Errorf("detecting the wrapping script's shell: %w", detectErr)
		}
		shell = detected
	}
	em := EvalEmitterForShell(shell, out, err)
	if em == nil {
		return nil, fmt.Errorf("unsupported shell: %q", shell)
	}
	return em, nil
}

// EmitUndetectedShellError reports msg directly to errOut (the real stderr) and writes a bare
// "exit 2" to out — for when the wrapping script's shell itself couldn't be resolved, so no
// shell-specific EvalEmitter exists yet to encode the error in that shell's own syntax. msg
// bypasses eval entirely (a direct write, not shell-embedded text), so it needs no quoting;
// "exit 2" alone is valid under any shell, which is the one thing that still has to go through
// eval: an ordinary returned Go error has nothing on stdout, so eval "$(...)" would silently
// eval an empty string and the script would carry on unaware, instead of actually halting.
func EmitUndetectedShellError(out, errOut io.Writer, msg string) error {
	if _, err := fmt.Fprintln(errOut, msg); err != nil {
		return err
	}
	_, err := fmt.Fprintln(out, "exit 2")
	return err
}
