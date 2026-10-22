# Spec file format

A compleo spec is a [carapace-spec](https://carapace-sh.github.io/carapace-spec/) YAML
document — the same format carapace-bin's completers use — with three additions:
`positionals:`, a `type:` key, and a `completion:` key co-located on a flag's (or positional's)
own definition, instead of plain carapace-spec's separate, name/index-keyed top-level
`completion:` section. Everything else (`name`, `description`, `flags`, `persistentflags`,
`commands`, `aliases`, `group`, `hidden`, ...) is plain carapace-spec; see its docs for the full
field reference. A command's top-level `completion:` section (`positionalany`/`dash`/`dashany`
in plain carapace-spec) is gone entirely — compleo found no realistic case for declaring it by
hand once completion lives on a flag's/positional's own definition instead; see below.

For completion *actions* specifically (`$files`, `$directories`, and the rest of the
`$macro(...)` syntax used in a flag's or positional's own `completion:`), see carapace-spec's
own
[core macros documentation](https://carapace-sh.github.io/carapace-spec/carapace-spec/macros.html) —
these all work out of the box, just by compleo importing carapace-spec (see `examples/macros`
for a runnable demo of every one except `$spec`, which needs its own separate spec file to
delegate to — see `examples/other_spec`). carapace-bin additionally ships a much larger set of
tool-specific macros (`$carapace.tools.git.LocalBranches`, and ~130 more); compleo doesn't link
those in directly, but they still work transparently at Tab-press time by shelling out to a
separately-installed `carapace` binary — see
[carapace-bin's macro documentation](https://carapace-sh.github.io/carapace-bin/spec/macros.html)
for that list.

compleo also registers one macro of its own, `$compleo.scripts` — like `$executables`, but
narrowed to executables on `$PATH` that look like they actually wrap compleo (a shebang, plus
the word "compleo" somewhere in the file). Used for `compleo generate <TAB>`; available in any
spec compleo parses, not just its own. Any other `$name.macro` is this same pattern — the
currently-running binary's own name, namespacing its own macros, same as `$carapace.*`.

## What carapace-spec gives you

- `name`, `description` — shown in `--help` and used for zsh's `#compdef`.
- `flags` / `persistentflags` — carapace-spec's shorthand-and-modifier syntax
  (`-c, --count=`, `--tag*=`, `--optarg?`, ...). Flags only ever carry a `string`, `[]string`
  (repeatable), `bool`, or `count` value at this layer — see `type:` below to declare anything
  more specific.
- `commands` — subcommands, each a full `Command` in its own right (so `positionals:` and a
  flag's `type:` both nest into subcommands too).
- `completion:` — completion actions, using carapace-bin's macros. Plain carapace-spec keys
  these by flag name/position under a separate top-level `flag`/`positional` map; compleo moves
  `flag`/`positional` entries onto the flag/positional itself instead (see below), and drops the
  rest of that top-level section (`positionalany`/`dash`/`dashany`) rather than keep it as a
  spec-author-facing feature — see below.
- `run:` — carapace-spec's own field for making a spec directly runnable. compleo **ignores
  it**: compleo parses a command line and emits zsh, it never executes the spec itself.

## What compleo adds

### `positionals:`

carapace-spec has no structured concept of positional arguments at all — no name, arity, or
type, just whatever's left over after flags. compleo adds a `positionals:` list per command:

```yaml
positionals:
  - name: name       # required: emitted as the scalar $compleo_name
    description: who to greet
    required: true   # missing => a parse error (see below)
    variadic: false  # true => the array $compleo_name, not a scalar; must be last
    type: string      # see Types below; defaults to string if omitted
    completion: ["$files"]  # optional: this positional's own completion action, see below
```

Rules compleo enforces (in `internal/parse`, not carapace-spec): at most one `variadic:
true` positional, and it must be last; a `required: true` positional may not follow an
optional one.

A positional's own `completion:` works the same way a flag's does — see the next section.

### `completion:` on a flag or positional

carapace-spec normally keys a flag's or positional's completion by name/index in a separate,
command-level `completion:` section, disconnected from the flag's own `flags:`/`positionals:`
entry — see "Why carapace separates them" below for why that's carapace's own natural shape.
compleo instead lets a `completion:` key live directly on the flag or positional it completes:

```yaml
flags:
  --file=: {description: input file, completion: ["$files"]}
positionals:
  - name: target
    completion: ["$compleo.scripts"]
```

This is purely where the same action expression is written — the value (a list of literal
candidates, or a `["$macro(...)"]` call, exactly as plain carapace-spec's `completion:` section
accepts) is unchanged, and `toCarapace()` gathers these back into carapace-spec's own
name-keyed/index-keyed shape before handing off to `ToCobraE()`. Nothing about how an action
expression is written or resolved is compleo-specific; only where you declare it is.

Two behaviors are specific to positionals, since carapace-spec's own positional completion is
index-keyed, not name-keyed:

- A positional with no `completion:` of its own contributes nothing to the index-keyed list —
  it isn't padded with an empty placeholder — *unless* a later positional does declare one, in
  which case the gap has to be kept to preserve the later one's index (same limitation plain
  carapace-spec's own `completion.positional` always had; compleo just works out the indices
  for you instead of you counting them by hand).
- A trailing **variadic** positional's own `completion:` only ever covers its first word —
  carapace's index-keyed list has no way to say "and every word after this one too" — so it's
  also used as the command's `completion.positionalany` fallback (the only way left to
  influence it; see below).

If nothing above applies — no positionals declared at all, or none of them have a
`completion:`, and there's no trailing variadic positional to fall back to — Tab-completion for
that position explicitly completes to nothing (`$compleo.none`), rather than being left to
whatever carapace's own fallback does for a position nobody declared (observed to be the cwd's
files). There's deliberately no way to declare a command-level `completion.positionalany`/`dash`/
`dashany` yourself: every case we could construct for wanting one either already has a better,
named equivalent via a variadic `positionals:` entry, or describes a position compleo's own
parser (`internal/parse`) would reject as an arity error anyway — completion for an argument
that's guaranteed to fail isn't useful, it's misleading. `dash`/`dashany` specifically would also
create drift between parsing and completion: compleo's parser doesn't distinguish an argument
before vs. after a literal `--` (pflag strips it before `internal/parse` ever sees the args), so
giving it different completion than an ordinary positional would promise something parsing
doesn't honor — exactly what compleo's one-spec-is-the-source-of-truth design exists to prevent.

For an `enum` *flag or positional* (`type: {kind: enum, values: [...]}`, see Types below), a
`completion:` also gets auto-filled from the same `values:` if you don't declare one yourself
— see the Types section for the full rationale.

#### Why carapace separates them

carapace's primary job is retrofitting completion onto a command *that already exists* — a CLI
built with its own `pflag.FlagSet`, which has no completion field at all (pflag predates
carapace). Completion can only be bolted on afterward as a separate, name-keyed registration,
mirroring how cobra itself does shell completion (`RegisterFlagCompletionFunc`, also a side map
keyed by name, not a method on the flag). carapace-spec's YAML `completion:` section is a
fairly direct serialization of that same Go API shape. compleo doesn't have that constraint —
it builds the cobra tree itself from its own `Flag`/`Positional` structs rather than retrofitting
someone else's — so co-locating is a real option for compleo in a way it never was for carapace.

### `type:` on a flag

carapace-spec flags are always `string`/`[]string`/`bool`/`count` as far as parsing goes. A
flag's object-form value accepts one more key, `type:`, alongside `description`/`nargs`/
`default`, naming a compleo `Type` validated after parsing (cobra/pflag never see it — an
invalid value is still just a string to them):

```yaml
flags:
  --times=: {description: how many times to repeat, default: "1", type: int}
```

A declared type also gets appended to the flag's `--help` description (e.g. `` (`int`) ``),
so `--times int` reads correctly even though pflag's underlying registered type is `string`.

Plain carapace-spec/carapace-bin tolerates this fine if the same file is ever consumed
directly by them: their `FlagSet` decoder already accepts the object form and simply ignores
any key besides `description`/`nargs`/`default` — compleo's own decoder is the one that
additionally reads (and, on an unrecognized key, rejects a typo in) `type:` and `completion:`.
A plain carapace-spec/carapace-bin consumer of the same file would still see the flag's
description and parsing behavior correctly, but silently get no completion for it at all.

### Types

A bare string (e.g. `type: int`) is shorthand for the kind alone, with no further attributes.
Some kinds accept more, as a nested object with a `kind:` key instead:

| `type:` | Meaning                                    |
| ---------- | ------------------------------------------- |
| *(omitted)*, `string` | no extra validation (the default) |
| `int`, or `{kind: int, min: N, max: N}` | must parse as a Go `int` (`strconv.Atoi`); `min`/`max` (either optional) bound it, inclusive |
| `float`, or `{kind: float, min: N, max: N}` | must parse as a `float64`; `min`/`max` as above |
| `bool`     | must parse as a bool (`true`/`false`/`1`/`0`/...) |
| `duration` | must parse as a Go duration (`"1h30m"`, ...) |
| `path`     | accepted like `string` — no extra validation yet |
| `{kind: enum, values: [...]}` | must be one of `values:` — always needs the object form, since there's nothing meaningful to validate with just the bare word `enum` |

`type:` works the same way on a `positionals:` entry as it does on a flag.

```yaml
flags:
  --port=: {description: a port, type: {kind: int, min: 1, max: 65535}}
  --color=: {description: a color, type: {kind: enum, values: [red, green, blue]}}
```

`--help`'s type hint shows the bounds too when either is set, not just the bare kind name —
`--port`'s declared above renders as `--port int 1-65535` (only one of `min`/`max` shows as
`>=1` or `<=65535` instead).

For an `enum` flag *or positional*, this also auto-fills its own `completion:` with the same
values, unless one is already explicitly declared there (see the `completion:` section above)
— so the list only needs writing once to get both validation and Tab-completion. `int`/`float`'s
`min`/`max` have no such completion-derivation — there's no core carapace-spec macro for a
bounded range that doesn't need the separately-installed `carapace` binary (see
`examples/carapace_bin_macros`), so unlike `enum`'s free literal-values completion, deriving one
here would trade a dependency-free feature for one with a hidden runtime dependency; declare
`completion:` yourself if you want it.

## Full example

From `examples/greet`:

```yaml
name: ${0:t}
description: Print a greeting.
flags:
  --loud: {description: shout the greeting}
  --times=: {description: how many times to repeat, default: "1", type: {kind: int, min: 1}}
positionals:
  - name: name
    description: who to greet
    required: true
```

See `examples/greet` (parsing/positionals/a typed flag), `examples/extensions` (every
`positionals:`/`type:` combination — required/optional/variadic, `int`/`float`/`bool`/
`duration`/`path`/`enum` — in one script), `examples/macros` (every core carapace-spec
completion macro except `$spec`), and `examples/other_spec` (`$spec`, which needs a dynamic
path a quoted heredoc can't supply on its own) for complete, runnable scripts.
