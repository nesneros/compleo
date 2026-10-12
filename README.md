# compleo

A Go tool for zsh scripts. One spec file is the single source of truth for
parsing a script's command line, its usage/help message, and its zsh
completion (built on [carapace](https://carapace.sh)).

See [CLAUDE.md](./CLAUDE.md) for design decisions and open questions. Early
design phase — no functionality yet.

## Development

Tool versions and tasks are managed with [mise](https://mise.jdx.dev):

```sh
mise install
mise run build
mise run test
mise run check
```
