package shells

import (
	"io"

	"github.com/nesneros/compleo/internal/parse"
	"github.com/spf13/cobra"
)

// Emitter writes one shell's syntax. zsh.Emitter implements it; a bash emitter would too.
type Emitter interface {
	// Used through EvalEmitter: code a wrapped script evals.

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

	// Used through CompletionEmitter: the interactive shell, where Tab is pressed.

	// WrappedCompletion returns a wrapped script's completion snippet, with specFilePath embedded for Tab presses.
	WrappedCompletion(cmd *cobra.Command, name string, specFilePath string) (string, error)
	// EmitActivate wires the completions directory into the current shell.
	EmitActivate(w io.Writer, dir string) error

	// Used by both.

	// Quote returns s quoted for this shell.
	Quote(s string) string
}

// CompletionEmitter is the interactive shell, where Tab is pressed.
// EvalEmitter is the shell that evals a wrapped script.
var (
	completionEmitter Emitter
	evalEmitter       Emitter
)

// SetCompletionEmitter sets the interactive-shell emitter.
func SetCompletionEmitter(e Emitter) { completionEmitter = e }

// CompletionEmitter returns the interactive-shell emitter.
func CompletionEmitter() Emitter { return completionEmitter }

// SetEvalEmitter sets the emitter for code a wrapped script evals.
func SetEvalEmitter(e Emitter) { evalEmitter = e }

// EvalEmitter returns the emitter for code a wrapped script evals.
func EvalEmitter() Emitter { return evalEmitter }
