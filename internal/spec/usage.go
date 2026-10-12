package spec

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// applySpec wires a synopsis line and "Arguments:" help section onto cmd and every
// descendant, from sp's Positionals. cmd must be built from sp (e.g. via Parse).
//
// Subcommands are matched by name, not position: cobra sorts Commands() alphabetically.
//
// Children are wired before their parent: UsageFunc() falls back to a parent's func for
// any command without its own, so wiring bottom-up guarantees that fallback only ever
// reaches cobra's true default, never a sibling's/parent's closure.
func applySpec(cmd *cobra.Command, sp Command) {
	for _, sub := range cmd.Commands() {
		for _, subSpec := range sp.Commands {
			if subSpec.Name == sub.Name() {
				applySpec(sub, subSpec)
				break
			}
		}
	}
	wire(cmd, sp.Positionals)
}

func wire(cmd *cobra.Command, positionals []Positional) {
	if len(positionals) > 0 {
		cmd.Use = synopsisLine(cmd, positionals)
	}

	defaultUsage := cmd.UsageFunc()
	cmd.SetUsageFunc(func(c *cobra.Command) error {
		if err := defaultUsage(c); err != nil {
			return err
		}
		if len(positionals) == 0 {
			return nil
		}
		fmt.Fprint(c.OutOrStderr(), "\nArguments:\n"+formatPositionals(positionals))
		return nil
	})
}

func synopsisLine(cmd *cobra.Command, positionals []Positional) string {
	parts := []string{cmd.Name()}
	if cmd.HasAvailableFlags() {
		parts = append(parts, "[flags]")
	}
	for _, p := range positionals {
		parts = append(parts, placeholder(p))
	}
	return strings.Join(parts, " ")
}

// placeholder: <name> required, [name] optional, trailing "..." if variadic.
func placeholder(p Positional) string {
	switch {
	case p.Required && p.Variadic:
		return "<" + p.Name + ">..."
	case p.Required:
		return "<" + p.Name + ">"
	case p.Variadic:
		return "[" + p.Name + "...]"
	default:
		return "[" + p.Name + "]"
	}
}

// formatPositionals renders the "Arguments:" body, names padded like cobra pads flags.
func formatPositionals(positionals []Positional) string {
	width := 0
	for _, p := range positionals {
		if len(p.Name) > width {
			width = len(p.Name)
		}
	}

	var b strings.Builder
	for _, p := range positionals {
		fmt.Fprintf(&b, "  %-*s   %s\n", width, p.Name, p.Description)
	}
	return strings.TrimRight(b.String(), "\n")
}
