// Package usage generates the positional-argument parts of a command's usage synopsis
// and help text. carapace-spec's ToCobraE has no concept of named positional arguments
// at all (only completion candidates), and cobra's own default usage/help rendering has
// no section for them either — it only ever prints whatever free text happens to be in
// Use. This package is what actually makes spec.Command's Positionals visible to a user
// running --help or getting a usage error.
package usage

import (
	"fmt"
	"strings"

	"github.com/nesneros/compleo/internal/spec"
	"github.com/spf13/cobra"
)

// Apply wires a generated synopsis line and an "Arguments:" help section onto cmd and
// every descendant, from sp's (and its subcommands') Positionals. cmd must have been
// built from sp, e.g. via spec.Command(sp.ToCarapace()).ToCobraE().
//
// Subcommands are matched to sp.Commands by name rather than by position: cobra sorts
// Commands() alphabetically by default (EnableCommandSorting), so it generally won't
// match sp.Commands' declaration order.
//
// Descendants are wired before their parent (post-order). Each wire call captures
// cmd.UsageFunc() as "the real default" to wrap — but UsageFunc() falls back to a
// parent's func for any command that doesn't have its own set yet. Wiring children first
// guarantees that fallback can only ever reach cobra's true built-in default, never a
// sibling's or parent's closure (which would print the wrong command's arguments).
func Apply(cmd *cobra.Command, sp spec.Command) {
	for _, sub := range cmd.Commands() {
		for _, subSpec := range sp.Commands {
			if subSpec.Name == sub.Name() {
				Apply(sub, subSpec)
				break
			}
		}
	}
	wire(cmd, sp.Positionals)
}

func wire(cmd *cobra.Command, positionals []spec.Positional) {
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

func synopsisLine(cmd *cobra.Command, positionals []spec.Positional) string {
	parts := []string{cmd.Name()}
	if cmd.HasAvailableFlags() {
		parts = append(parts, "[flags]")
	}
	for _, p := range positionals {
		parts = append(parts, placeholder(p))
	}
	return strings.Join(parts, " ")
}

// placeholder renders one positional's synopsis notation: <name> for a required
// argument, [name] for an optional one, with a trailing "..." for a variadic one
// (<name>... / [name...]) — the same angle-bracket convention used by git and docker.
func placeholder(p spec.Positional) string {
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

// formatPositionals renders the "Arguments:" section body: one line per positional, its
// bare name (not its bracketed placeholder) padded to align the descriptions, the same
// way cobra pads flag names in its "Flags:" section.
func formatPositionals(positionals []spec.Positional) string {
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
