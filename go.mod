module github.com/nesneros/compleo

go 1.27.1

require (
	github.com/carapace-sh/carapace v1.16.3
	github.com/carapace-sh/carapace-spec v1.9.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/stretchr/testify v1.12.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/carapace-sh/carapace-shlex v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

// Full flag-syntax support (shorthand-only flags, non-posix single-dash flags, nargs-consuming
// flags): without this, carapace-spec's addFlagTo silently drops those forms instead of
// registering them — see CLAUDE.md's "Full flag-syntax support" design decision.
replace github.com/spf13/pflag => github.com/carapace-sh/carapace-pflag v1.3.0
