package spec

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

// decodeYAML decodes a compleo spec file. Must use gopkg.in/yaml.v3, matching the pinned
// carapace-sh/carapace v1.16.3 — command.FlagSet/Run's custom UnmarshalYAML won't be
// found by a different library's decoder.
func decodeYAML(r io.Reader) (Command, error) {
	var cmd Command
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)
	err := decoder.Decode(&cmd)
	return cmd, err
}

// ParseYAML reads a spec from r and builds its cobra command tree, with usage wired in.
// name, if non-empty, fills in a missing name: or must match a declared one (error on
// mismatch); pass "" when there's no outside name to contribute.
func ParseYAML(r io.Reader, name string) (*cobra.Command, Command, error) {
	sp, err := decodeYAML(r)
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
	// Without a name:, cmd.Use would be "" — cobra's Name() reads "" as the first word of
	// Use, which renders as a stray blank in --help's "Usage:" line. <script> fills that
	// gap honestly: compleo has no other way to know the invoking name (never sees $0).
	if sp.Name == "" {
		cmd.Use = "<script>"
	}
	applySpec(cmd, sp)
	return cmd, sp, nil
}
