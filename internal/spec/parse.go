package spec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

// validPrefix matches a legal shell-variable-name prefix, including any separator the spec
// author wants (e.g. "greet_", or "greet" for no separator at all — shells.VarName joins
// prefix and name as-is). Unlike a flag/positional name (sanitized character-by-character by
// shells.VarName, since those come from the spec author's own --flag-name/positional choices),
// a declared prefix: has no reason to need a hyphen or anything else illegal in a shell
// identifier, so a bad one is rejected here instead of silently producing unusable variables.
var validPrefix = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// decodeYAML decodes a compleo spec document into a Command, rejecting unknown fields.
func decodeYAML(data string) (Command, error) {
	var cmd Command
	decoder := yaml.NewDecoder(strings.NewReader(data))
	decoder.KnownFields(true)
	err := decoder.Decode(&cmd)
	return cmd, err
}

// ParseYAML parses a spec from data and builds its cobra command tree, with usage wired in.
// name, if non-empty, fills in a missing name: or must match a declared one (error on
// mismatch); pass "" when there's no outside name to contribute.
func ParseYAML(data string, name string) (*cobra.Command, Command, error) {
	sp, err := decodeYAML(data)
	if err != nil {
		return nil, Command{}, err
	}
	if name != "" {
		if sp.Name != "" && sp.Name != name {
			return nil, Command{}, fmt.Errorf("spec declares name: %q, but %q was given — these must match", sp.Name, name)
		}
		sp.Name = name
	}
	if sp.Prefix != "" && !validPrefix.MatchString(sp.Prefix) {
		return nil, Command{}, fmt.Errorf("prefix %q: must match %s", sp.Prefix, validPrefix)
	}
	cmd, err := sp.toCarapace().ToCobraE()
	if err != nil {
		return nil, Command{}, err
	}
	// Without a name:, cmd.Use would be "", rendering as a stray blank in "Usage:" — compleo
	// has no other way to know the invoking name (never sees $0), so <script> fills the gap.
	if sp.Name == "" {
		cmd.Use = "<script>"
	}
	applySpec(cmd, sp)
	return cmd, sp, nil
}
