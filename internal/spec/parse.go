package spec

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

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
