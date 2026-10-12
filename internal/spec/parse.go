package spec

import (
	"io"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

// decodeYAML decodes a compleo spec file. Must use gopkg.in/yaml.v3, matching the pinned
// carapace-sh/carapace v1.16.3 — command.FlagSet/Run's custom UnmarshalYAML won't be
// found by a different library's decoder.
func decodeYAML(r io.Reader) (Command, error) {
	var cmd Command
	err := yaml.NewDecoder(r).Decode(&cmd)
	return cmd, err
}

// ParseYAML reads a spec from r and builds its cobra command tree, with usage wired in.
func ParseYAML(r io.Reader) (*cobra.Command, Command, error) {
	sp, err := decodeYAML(r)
	if err != nil {
		return nil, Command{}, err
	}
	cmd, err := sp.toCarapace().ToCobraE()
	if err != nil {
		return nil, Command{}, err
	}
	applySpec(cmd, sp)
	return cmd, sp, nil
}
