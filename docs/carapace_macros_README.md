# Carapace-Bin Macros Example

This directory contains examples demonstrating how to use **carapace-bin's 900+ completion macros** with **compleo**.

## Overview

**Carapace** is a powerful completion library that provides rich, context-aware completions for zsh and other shells. **Carapace-bin** is the companion binary that provides hundreds of pre-built completion actions (macros) for:

- Filesystem operations
- Network configuration
- Environment variables
- Colors and styles
- Package managers (npm, cargo, pip, etc.)
- Version control systems (git, hg)
- Container tools (docker, podman)
- Cloud platforms (aws, terraform)
- And much more...

**Compleo** is a Go tool that uses carapace under the hood to provide:
- Command-line argument parsing
- Help/usage message generation
- zsh completion

All from a single YAML spec file.

## Files in This Directory

| File | Purpose |
|------|---------|
| `carapace_macros.md` | Complete documentation of all macro categories |
| `carapace_macros_list.txt` | Raw list of core/action macros |
| `carapace_macros_example.yaml` | Example compleo spec using many macros |
| `carapace_macros_example.sh` | Example shell script using the spec |
| `carapace_macros_README.md` | This file |

## Quick Start

### 1. Install compleo

```bash
# Install from source
go install github.com/nesneros/compleo@latest

# Or download a release from GitHub
```

### 2. Try the example

```bash
# Make the example script executable
chmod +x carapace_macros_example.sh

# Source it for completion to work in your shell
source carapace_macros_example.sh

# Try it!
./carapace_macros_example.sh --help

# Test completion (press TAB after --)
./carapace_macros_example.sh --fg-color <TAB>
./carapace_macros_example.sh --git-branch <TAB>
./carapace_macros_example.sh --http-method <TAB>
```

## Macro Categories Demonstrated

### Core Macros (carapace-spec)
- `$files` - File completion
- `$directories` - Directory completion
- `$executables` - Executable file completion
- Shell execution macros (`$bash`, `$zsh`, etc.)

### Filesystem Macros (pkg/actions/fs/)
- `$fs.FilenameExtensions`
- `$fs.FilesystemTypes`
- `$fs.FileModesSymbolic`
- `$fs.BlockDevices`
- `$fs.Mounts`

### Environment Macros (pkg/actions/env/)
- `$env.Names` - Environment variable names
- `$env.Values` - Environment variable values

### Color Macros (pkg/actions/color/)
- `$color.AnsiForegroundColors`
- `$color.AnsiBackgroundColors`
- `$color.HexColors`
- `$color.XtermColorNames`

### Network Macros (pkg/actions/net/)
- `$net.Ports`, `$net.KnownPorts`
- `$net.Protocols`
- `$net.Ipv4Addresses`, `$net.Ipv6Addresses`
- `$net.Subnets`, `$net.Hosts`
- `$net.ssh.*` - SSH-specific completions
- `$net.http.*` - HTTP-specific completions (methods, status codes, media types, etc.)

### OS Macros (pkg/actions/os/)
- `$os.Users`, `$os.Groups`
- `$os.Commands`, `$os.Signals`
- `$os.Shells`
- `$os.Displays`, `$os.Locales`

### Process Macros (pkg/actions/ps/)
- `$ps.ProcessIds`, `$ps.ProcessExecutables`
- `$ps.KillSignals`

### Text Macros (pkg/actions/text/)
- `$text.Languages`
- `$text.Timezones`

### Time Macros (pkg/actions/time/)
- `$time.Months`, `$time.Weekdays`
- `$time.Durations`

### Number Macros (pkg/actions/number/)
- `$number.Bases`
- `$number.Digits`

### Tool-Specific Macros (pkg/actions/tools/)
- `$tools.git.*` - Branches, remotes, tags, etc.
- `$tools.docker.*` - Containers, images, volumes
- `$tools.npm.*` - Scripts, packages
- `$tools.python.*` - Packages, modules
- `$tools.systemd.*` - Units, services

### Shell Macros (pkg/actions/shell/)
- `$shell.Aliases`, `$shell.Builtins`, `$shell.Functions`

## Creating Your Own Spec

To create your own script with carapace macros:

1. **Create a YAML spec file** (e.g., `my_script.yaml`):
   ```yaml
   name: my-script
   description: My awesome script
   
   flags:
     - name: input
       type: string
       description: Input file
       completion: $files
       
     - name: color
       type: string
       description: Text color
       completion: $color.256Colors
   
   positionals:
     - name: args
       type: string
       variadic: true
       description: Additional arguments
   ```

2. **Create your shell script** (e.g., `my_script.sh`):
   ```bash
   #!/usr/bin/env zsh
   SPEC=$(cat "${0:h}/my_script.yaml")
   eval "$(compleo parse -- "$@" <<< "$SPEC")"
   
   # Access parsed values via $compleo_<name>
   echo "Input file: $compleo_input"
   echo "Color: $compleo_color"
   ```

3. **Make it executable**:
   ```bash
   chmod +x my_script.sh
   source my_script.sh  # For completion
   ```

## Advanced Usage

### Modifiers

Carapace provides **modifiers** that can transform completion behavior:

```yaml
flags:
  - name: unique-tags
    type: string
    completion: $tag
    modifiers: $uniquelist
```

Available modifiers:
- `$chdir` - Change directory before completion
- `$list` - List format
- `$multiparts` - Multi-part completion
- `$nospace` - No space after completion
- `$noprefix` - No prefix
- `$prefix` - Add prefix
- `$shift` - Shift arguments
- `$split`, `$splitp` - Split handling
- `$style` - Style completion
- `$suffix` - Add suffix
- `$suppress` - Suppress completion
- `$tag` - Tag-based completion
- `$uniquelist` - Unique list
- `$usage` - Usage help
- `$filter`, `$filterargs` - Filtering
- `$retain` - Retain context

### Multiple Completions

You can chain multiple completions together:

```yaml
flags:
  - name: git-ref
    type: string
    completion: $tools.git.Branches ||| $tools.git.Tags ||| $tools.git.Remotes
```

### Conditional Completions

Use condition macros to enable/disable completions based on context:

```yaml
flags:
  - name: arch-specific
    type: string
    completion: $files
    when: $conditions.Os darwin linux
```

## Complete Macro List

For a complete list of all 900+ macros, see:
- `carapace_macros.md` - Organized by category with descriptions
- `carapace_macros_list.txt` - Raw alphabetical list

## References

- **Carapace**: https://github.com/carapace-sh/carapace
- **Carapace-Bin**: https://github.com/carapace-sh/carapace-bin
- **Compleo**: https://github.com/nesneros/compleo

## License

These examples are provided for educational purposes. See the individual project licenses for carapace, carapace-bin, and compleo.
