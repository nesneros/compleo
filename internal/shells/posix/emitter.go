package posix

import (
	"fmt"
	"io"
	"strings"
)

// Emitter is the shared POSIX behavior embedded by zsh and bash.
// StdoutFormat and StderrFormat are fmt formats with one %s for a quoted word.
// Shell is the name Name returns.
type Emitter struct {
	StdoutFormat string
	StderrFormat string
	Shell        string
}

// Name returns this shell's name.
func (e Emitter) Name() string { return e.Shell }

// EmitHelp prints text to stdout and exits 0.
func (e Emitter) EmitHelp(w io.Writer, text string) error {
	if err := e.print(w, e.StdoutFormat, strings.TrimRight(text, "\n")); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "exit %d\n", 0)
	return err
}

// EmitError prints "name: parseErr" to stderr and exits 2.
func (e Emitter) EmitError(w io.Writer, name string, parseErr error) error {
	if err := e.print(w, e.StderrFormat, fmt.Sprintf("%s: %s", name, parseErr)); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "exit %d\n", 2)
	return err
}

// PrintStderr prints text to stderr using Stderr.
func (e Emitter) PrintStderr(w io.Writer, text string) error {
	return e.print(w, e.StderrFormat, text)
}

func (e Emitter) print(w io.Writer, format, text string) error {
	_, err := fmt.Fprintf(w, format, e.Quote(text))
	return err
}
