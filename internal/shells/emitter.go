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
	"github.com/nesneros/compleo/internal/shells/posix"
	"github.com/spf13/cobra"
)

// EvalEmitter writes code a wrapped script evals.
type EvalEmitter interface {
	// EmitResult writes the parsed flag and positional values.
	EmitResult(w io.Writer, result parse.Result) error
	// EmitHelp writes text the script prints, then exits 0.
	EmitHelp(w io.Writer, text string) error
	// EmitError writes the parse error to stderr and exits 2.
	EmitError(w io.Writer, name string, parseErr error) error
	// EmitUp writes the compleo parse invocation the script evals.
	EmitUp(w io.Writer, rawSpec string) error
	// EmitGenerated writes the generate report: hint (from the completion-target shell's own
	// ActivationHint, which may be a different shell than this one — see CompletionEmitter)
	// plus any missing-binary warning, in this shell's own syntax, then exits 0.
	EmitGenerated(w io.Writer, hint string, missing []string) error

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
	EmitActivate(w io.Writer, dir string) error
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
// zsh.Emitter and bash.Emitter both implement and register it.
type Emitter interface {
	EvalEmitter
	CompletionEmitter
}

func RegisterEmitter(e any) {
	if ee, ok := e.(EvalEmitter); ok {
		registeredEvalEmitters[ee.Name()] = ee
	}
	if ce, ok := e.(CompletionEmitter); ok {
		registeredCompletionEmitters[ce.Name()] = ce
	}
}

var (
	registeredEvalEmitters       map[string]EvalEmitter
	registeredCompletionEmitters map[string]CompletionEmitter
)

func init() {
	registeredEvalEmitters = make(map[string]EvalEmitter)
	registeredCompletionEmitters = make(map[string]CompletionEmitter)
}

func CompletionEmitterForShell(shell string) (CompletionEmitter, error) {
	emitter := registeredCompletionEmitters[shell]
	if emitter == nil {
		return nil, fmt.Errorf("unsupported shell: %q", shell)
	}
	return emitter, nil
}

// GetCompletionEmitter returns the interactive-shell emitter named by EnvCompletionShell.
func GetCompletionEmitter() CompletionEmitter {
	shell := os.Getenv(cmdutil.EnvCompletionShell)
	emitter, err := CompletionEmitterForShell(shell)
	if err != nil {
		cmdutil.Panic(fmt.Sprintf("Is compleo activated in the current shell? [%s]", err.Error()))
	}
	return emitter
}

// SortedKeys returns m's keys sorted, for deterministic output despite map iteration order.
func SortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	return slices.Sorted(maps.Keys(m))
}

// EvalEmitterForShell returns the registered EvalEmitter for shell, or nil if none is registered.
func EvalEmitterForShell(shell string) EvalEmitter {
	return registeredEvalEmitters[shell]
}

// ResolveEvalEmitter returns the EvalEmitter for shell, or, when shell is empty, for the
// wrapping script's own interpreter (cmdutil.ParentInterpreter — the direct parent of a
// "parse"/"up" invocation, always that script's shell: eval "$(...)" forks it straight into
// compleo, same argument as cmdutil.ParentProcessName's own doc comment). Returns an error
// describing what went wrong (detection failure or an unregistered shell name) rather than
// an EvalEmitter when resolution fails — never guesses zsh as a silent default, since that's
// exactly the wrong-shell-output bug this exists to avoid.
func ResolveEvalEmitter(shell string) (EvalEmitter, error) {
	if shell == "" {
		detected, err := cmdutil.ParentInterpreter()
		if err != nil {
			return nil, fmt.Errorf("detecting the wrapping script's shell: %w", err)
		}
		shell = detected
	}
	em := EvalEmitterForShell(shell)
	if em == nil {
		return nil, fmt.Errorf("unsupported shell: %q", shell)
	}
	return em, nil
}

// EmitUndetectedShellError writes a POSIX-portable payload (no zsh/bash-only syntax) that
// reports msg to stderr and exits 2 — for when the wrapping script's shell itself couldn't be
// resolved, so no shell-specific EvalEmitter exists yet to encode the error properly. Unlike a
// returned Go error, this still halts the script: an ordinary error has nothing on stdout, so
// eval "$(...)" would silently eval an empty string and the script would carry on unaware.
func EmitUndetectedShellError(w io.Writer, msg string) error {
	_, err := fmt.Fprintf(w, "printf '%%s\\n' %s >&2\nexit 2\n", posix.Emitter{}.Quote(msg))
	return err
}
