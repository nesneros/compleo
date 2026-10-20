package shells

import (
	"cmp"
	"io"
	"maps"
	"slices"

	"github.com/nesneros/compleo/internal/parse"
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
	// EmitGenerated writes the generate report, including any missing-binary warning.
	EmitGenerated(w io.Writer, name string, missing []string) error

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

	// Quote returns s quoted for this shell.
	Quote(s string) string
	// Name returns this shell's name.
	Name() string
}

// Emitter is an EvalEmitter and a CompletionEmitter.
// zsh.Emitter and bash.Emitter implement it; only zsh is installed.
type Emitter interface {
	EvalEmitter
	CompletionEmitter
}

var (
	completionEmitter CompletionEmitter
	evalEmitter       EvalEmitter
)

// SetCompletionEmitter sets the interactive-shell emitter.
func SetCompletionEmitter(e CompletionEmitter) { completionEmitter = e }

// GetCompletionEmitter returns the interactive-shell emitter.
func GetCompletionEmitter() CompletionEmitter { return completionEmitter }

// SetEvalEmitter sets the emitter for code a wrapped script evals.
func SetEvalEmitter(e EvalEmitter) { evalEmitter = e }

// GetEvalEmitter returns the emitter for code a wrapped script evals.
func GetEvalEmitter() EvalEmitter { return evalEmitter }

// SortedKeys returns m's keys sorted, for deterministic output despite map iteration order.
func SortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	return slices.Sorted(maps.Keys(m))
}
