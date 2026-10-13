# Spec file format

A compleo spec is a [carapace-spec](https://carapace-sh.github.io/carapace-spec/) YAML
document — the same format carapace-bin's completers use — with two additions: `positionals:`
and `flagtypes:`. Everything else (`name`, `description`, `flags`, `persistentflags`,
`commands`, `completion`, `aliases`, `group`, `hidden`, ...) is plain carapace-spec; see its
docs for the full field reference.

For completion *actions* specifically (`$files`, `$directories`, and the rest of the
`$macro(...)` syntax used under `completion:`), see carapace-spec's own
[core macros documentation](https://carapace-sh.github.io/carapace-spec/carapace-spec/macros.html) —
these all work out of the box, just by compleo importing carapace-spec (see `examples/macros`
for a runnable demo of every one). carapace-bin additionally ships a much larger set of
tool-specific macros (`$carapace.tools.git.branches`, and ~130 more); compleo doesn't link
those in directly, but they still work transparently at Tab-press time by shelling out to a
separately-installed `carapace` binary — see
[carapace-bin's macro documentation](https://carapace-sh.github.io/carapace-bin/spec/macros.html)
for that list. Macro names are never compleo's own invention — compleo uses exactly the same
names either way.

## What carapace-spec gives you

- `name`, `description` — shown in `--help` and used for zsh's `#compdef`.
- `flags` / `persistentflags` — carapace-spec's shorthand-and-modifier syntax
  (`-c, --count=`, `--tag*=`, `--optarg?`, ...). Flags only ever carry a `string`, `[]string`
  (repeatable), `bool`, or `count` value at this layer — see `flagtypes:` below to declare
  anything more specific.
- `commands` — subcommands, each a full `Command` in its own right (so `positionals:` and
  `flagtypes:` both nest into subcommands too).
- `completion:` — `flag`/`positional`/`positionalany`/`dash`/`dashany` completion actions,
  keyed by flag/position, using carapace-bin's macros.
- `run:` — carapace-spec's own field for making a spec directly runnable. compleo **ignores
  it**: compleo parses a command line and emits zsh, it never executes the spec itself.

## What compleo adds

### `positionals:`

carapace-spec has no structured concept of positional arguments at all — no name, arity, or
type, just whatever's left over after flags. compleo adds a `positionals:` list per command:

```yaml
positionals:
  - name: name       # required: becomes a key in the emitted $compleo[...] / $compleo_... array
    description: who to greet
    required: true   # missing => a parse error (see below)
    variadic: false  # true => a real zsh array ($compleo_name), not a scalar; must be last
    type: string      # see Types below; defaults to string if omitted
```

Rules compleo enforces (in `internal/parse`, not carapace-spec): at most one `variadic:
true` positional, and it must be last; a `required: true` positional may not follow an
optional one.

This is separate from, and not automatically linked to, carapace-spec's own
`completion.positional`/`completion.positionalany` (which complete by *index*, not name) —
you declare both independently if you want a positional both parsed *and* completed.

### `flagtypes:`

carapace-spec flags are always `string`/`[]string`/`bool`/`count` as far as parsing goes.
`flagtypes:` maps a flag's bare name (no dashes) to a compleo `Type`, validated after
parsing (cobra/pflag never see it — an invalid value is still just a string to them):

```yaml
flags:
  --times=: {description: how many times to repeat, default: "1"}
flagtypes:
  times: int
```

A declared type also gets appended to the flag's `--help` description (e.g. `` (`int`) ``),
so `--times int` reads correctly even though pflag's underlying registered type is `string`.

### Types

| YAML value | Meaning                                    |
| ---------- | ------------------------------------------- |
| *(omitted)*, `string` | no extra validation (the default) |
| `int`      | must parse as a Go `int` (`strconv.Atoi`)   |
| `float`    | must parse as a `float64`                   |
| `bool`     | must parse as a bool (`true`/`false`/`1`/`0`/...) |
| `duration` | must parse as a Go duration (`"1h30m"`, ...) |
| `path`     | accepted like `string` — no extra validation yet |

`type:` works the same way on a `positionals:` entry as it does in `flagtypes:`.

## Full example

From `examples/greet`:

```yaml
name: ${0:t}
description: Print a greeting.
flags:
  --loud: {description: shout the greeting}
  --times=: {description: how many times to repeat, default: "1"}
flagtypes:
  times: int
positionals:
  - name: name
    description: who to greet
    required: true
```

See `examples/greet` (parsing/positionals/flagtypes) and `examples/macros` (every core
carapace-spec completion macro) for complete, runnable scripts.
