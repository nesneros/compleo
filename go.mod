module github.com/nesneros/compleo

go 1.27.2

require (
	github.com/carapace-sh/carapace v1.16.3
	github.com/carapace-sh/carapace-spec v1.9.0
	github.com/shirou/gopsutil/v3 v3.24.5
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/stretchr/testify v1.12.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/carapace-sh/carapace-shlex v1.1.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20260802145828-341c2f0c90b5 // indirect
	github.com/power-devops/perfstat v0.0.0-20260916203055-22a1a467d9f0 // indirect
	github.com/shoenig/go-m1cpu v0.2.4 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

// Full flag-syntax support (shorthand-only flags, non-posix single-dash flags, nargs-consuming
// flags): without this, carapace-spec's addFlagTo silently drops those forms instead of
// registering them — see CLAUDE.md's "Full flag-syntax support" design decision.
replace github.com/spf13/pflag => github.com/carapace-sh/carapace-pflag v1.3.0
