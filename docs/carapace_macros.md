# Carapace and Carapace-Bin Macros Summary

## Overview

Carapace provides a rich set of completion macros that can be used in zsh completion scripts. These macros are organized into several categories:

1. **Core Macros** (from carapace-spec)
2. **Condition Macros** (from carapace-bin/pkg/conditions)
3. **Action Macros** (from carapace-bin/pkg/actions)

## Core Macros (carapace-spec)

These are the fundamental macros provided by carapace-spec:

### Shell Macros
- `$files` - File completion
- `$directories` - Directory completion  
- `$executables` - Executable file completion
- `$message` - Display a message
- `$spec` - Complete from a spec file

### Shell Execution Macros
- `$` (default shell)
- `$bash`
- `$cmd`
- `$elvish`
- `$fish`
- `$nu`
- `$osh`
- `$pwsh`
- `$sh`
- `$xonsh`
- `$zsh`

### Modifiers (can be applied to any action)
- `$chdir`
- `$list`
- `$multiparts`
- `$nospace`
- `$noprefix`
- `$prefix`
- `$shift`
- `$split`
- `$splitp`
- `$style`
- `$suffix`
- `$suppress`
- `$tag`
- `$uniquelist`
- `$usage`
- `$filter`
- `$filterargs`
- `$retain`

## Condition Macros (carapace-bin/pkg/conditions)

These are used to conditionally enable/disable completions:

- `$conditions.Arch` - Check runtime.GOARCH
- `$conditions.Os` - Check runtime.GOOS  
- `$conditions.Path` - Check if executable is in PATH
- `$conditions.Parent` - Check if parent directory contains file/directory

## Action Macros (carapace-bin/pkg/actions)

### Color Actions (pkg/actions/color/)
- `$color.256Colors`
- `$color.AnsiBackgroundColors`
- `$color.AnsiForegroundColors`
- `$color.HexColors`
- `$color.XtermColorNames`

### Environment Actions (pkg/actions/env/)
- `$env.Names`
- `$env.NameValues`
- `$env.Values`

### Filesystem Actions (pkg/actions/fs/)
- `$fs.ArFileContents`
- `$fs.BlockDevices`
- `$fs.FilenameExtensions`
- `$fs.FilesystemTypes`
- `$fs.FileModes`
- `$fs.FileModesNumeric`
- `$fs.FileModesSymbolic`
- `$fs.JarFileClasses`
- `$fs.Labels`
- `$fs.Mounts`
- `$fs.PartitionLabels`
- `$fs.PartitionUuids`
- `$fs.SubDirectories`
- `$fs.TarFileContents`
- `$fs.Uuids`
- `$fs.ZipFileContents`

### Network Actions (pkg/actions/net/)
- `$net.BaudRates`
- `$net.Ipv4Addresses`
- `$net.Ipv6Addresses`
- `$net.Subnets`

### HTTP Actions (pkg/actions/net/http/)
- `$net.http.CacheControlRequestDirectives`
- `$net.http.Charsets`
- `$net.http.ConnectionOptions`
- `$net.http.ContentEncodingTokens`
- `$net.http.EmailDomains`
- `$net.http.ETags`
- `$net.http.InstanceManipulations`
- `$net.http.MediaTypes`
- `$net.http.OpenApiPaths`
- `$net.http.PreferTokens`
- `$net.http.RangeUnits`
- `$net.http.RequestHeaderNames`
- `$net.http.RequestHeaders`
- `$net.http.RequestHeaderValues`
- `$net.http.StatusCodes`
- `$net.http.TEValues`
- `$net.http.Tags`
- `$net.http.TransferEncodingTokens`
- `$net.http.UpgradeTokens`
- `$net.http.UrlSchemes`
- `$net.http.Urls`
- `$net.http.UserAgents`
- `$net.http.WarningCodes`

### OS Actions (pkg/actions/os/)
- `$os.Commands`
- `$os.CurrentUserGroups`
- `$os.CurrentUsername`
- `$os.EnvironmentVariables`
- `$os.Gids`
- `$os.Groups`
- `$os.Hostname`
- `$os.Signals`
- `$os.Uids`
- `$os.Users`

### Process Actions (pkg/actions/ps/)
- `$ps.ProcessGroups`
- `$ps.ProcessIds`
- `$ps.Processes`

### Text Actions (pkg/actions/text/)
- `$text.Languages`
- `$text.Locales`
- `$text.Timezones`

### Time Actions (pkg/actions/time/)
- `$time.DateFormats`
- `$time.Durations`
- `$time.Months`
- `$time.Weekdays`

### Number Actions (pkg/actions/number/)
- `$number.Bases`
- `$number.Digits`

## Tools Actions (pkg/actions/tools/)

This is the largest category with 700+ actions for various tools:

### Version Control Systems
- `$tools.git.*` - Git related completions (branches, remotes, tags, etc.)
- `$tools.hg.*` - Mercurial related completions

### Container Tools
- `$tools.docker.*` - Docker (containers, images, volumes, networks, etc.)
- `$tools.docker.compose.*` - Docker Compose
- `$tools.podman.*` - Podman

### Package Managers
- `$tools.apt.*` - APT (Debian/Ubuntu)
- `$tools.brew.*` - Homebrew (macOS)
- `$tools.cargo.*` - Cargo (Rust)
- `$tools.conda.*` - Conda
- `$tools.gem.*` - RubyGems
- `$tools.go.*` - Go modules
- `$tools.mvn.*` - Maven
- `$tools.npm.*` - NPM
- `$tools.pip.*` - pip (Python)
- `$tools.yarn.*` - Yarn

### Cloud & Infrastructure
- `$tools.aws.*` - AWS CLI
- `$tools.terraform.*` - Terraform
- `$tools.kubectl.*` - kubectl (Kubernetes)
- `$tools.helm.*` - Helm

### System Tools
- `$tools.systemd.*` - systemd
- `$tools.journalctl.*` - journalctl
- `$tools.supervisor.*` - Supervisor

### Text Editors
- `$tools.helix.*` - Helix
- `$tools.vim.*` - Vim/Neovim

### Shells
- `$tools.zsh.*` - Zsh
- `$tools.fish.*` - Fish
- `$tools.bash.*` - Bash

### And many more...

For a complete list, see the full macro list in the carapace-bin repository.

## Usage in Compleo

In compleo, you can use these macros in your spec file like:

```yaml
positionals:
  - name: file
    type: string
    completion: $files

flags:
  - name: color
    type: string
    completion: $color.256Colors
```

## Macro Naming Convention

All macros follow the naming convention:
- Core macros: `$<name>` (e.g., `$files`, `$directories`)
- Nested macros: `$<category>.<name>` (e.g., `$color.256Colors`, `$tools.git.Branches`)

The CLAUDE.md document in compleo states that compleo should use the same macro names as carapace-spec/carapace-bin and should import the `MacroMap` and `MacroDescriptions` from carapace-bin's `pkg/actions/actions_generated.go` instead of hand-writing names.

