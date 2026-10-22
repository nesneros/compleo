package posix

import (
	"fmt"
	"io"
	"strings"
)

// Emitter is the shared POSIX behavior embedded by zsh and bash.
// StdoutFormat and StderrFormat are fmt formats with one %s for a quoted word.
// Shell is the name Name returns. Out is the stream every Emit* method writes the shell code
// to be eval'd to (bound once per invocation by the shell package's own New constructor); Err
// is the real stderr for a method that needs to bypass eval entirely (none currently do — see
// shells.EmitUndetectedShellError for the one case that needs exactly that, kept as a
// standalone function since it runs when no Emitter could be resolved at all).
type Emitter struct {
	StdoutFormat string
	StderrFormat string
	Shell        string
	Out          io.Writer
	Err          io.Writer
}

// Name returns this shell's name.
func (e Emitter) Name() string { return e.Shell }

// EmitHelp prints text to stdout and exits 0.
func (e Emitter) EmitHelp(text string) error {
	if err := e.print(e.StdoutFormat, strings.TrimRight(text, "\n")); err != nil {
		return err
	}
	_, err := fmt.Fprintf(e.Out, "exit %d\n", 0)
	return err
}

// EmitError prints "name: parseErr" to stderr and exits 2.
func (e Emitter) EmitError(name string, parseErr error) error {
	if err := e.print(e.StderrFormat, fmt.Sprintf("%s: %s", name, parseErr)); err != nil {
		return err
	}
	_, err := fmt.Fprintf(e.Out, "exit %d\n", 2)
	return err
}

// PrintStderr prints text to stderr using Stderr.
func (e Emitter) PrintStderr(text string) error {
	return e.print(e.StderrFormat, text)
}

func (e Emitter) print(format, text string) error {
	_, err := fmt.Fprintf(e.Out, format, e.Quote(text))
	return err
}
