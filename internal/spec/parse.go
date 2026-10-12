package spec

import (
	"io"

	yaml "gopkg.in/yaml.v3"
)

// ParseYAML decodes a compleo spec file. It uses the same YAML library as the pinned
// carapace-sh/carapace v1.16.3 (gopkg.in/yaml.v3 — carapace only migrated to the
// go.yaml.in/yaml/v3 fork after this tag), which command.FlagSet and command.Run rely on
// for their custom UnmarshalYAML methods — a different library's decoder would not find
// them, since those methods are tied to that library's own *yaml.Node type.
func ParseYAML(r io.Reader) (Command, error) {
	var cmd Command
	err := yaml.NewDecoder(r).Decode(&cmd)
	return cmd, err
}
