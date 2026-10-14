# Using compleo in a script

This is about wiring a zsh script up to compleo — the line you add to the script itself.
For what you can put in the spec (flags, positionals, types, completion), see
[docs/spec.md](./spec.md).

## 1. Add the line

Near the top of the script, before anything that uses its arguments:

```zsh
eval "$(compleo parse -- "$@" <<SPEC
name: ${0:t}
description: Print a greeting.
flags:
  --loud: {description: shout the greeting}
positionals:
  - name: name
    description: who to greet
    required: true
SPEC
)"
```

That's it — compleo parses `"$@"` against the embedded spec and prints zsh that the `eval`
runs, setting the parsed values (see step 2). `--help`, bad arguments, and errors are all
handled automatically from this one line; nothing else is needed for those (see "What the
`eval` can do on its own," below).

One thing about the heredoc specifically: **it's unquoted** (`<<SPEC`, not `<<'SPEC'`) on
purpose, so `${0:t}` expands — `${0:t}` is zsh for "basename of `$0`," i.e. whatever the
script was actually invoked as (`greet`, not `./bin/greet` or a symlink's target), used as the
spec's own `name:`. The cost of leaving it unquoted: *any* literal `$` or `` ` `` in the spec —
including in plain description text, not just macros — needs escaping (`\$`, `` \` ``), or it
silently expands to nothing. See `examples/macros` for a spec with many escaped
`\$macro(...)` calls.

The alternative is to quote the delimiter (`<<'SPEC'`): nothing in the spec needs escaping
then, but `${0:t}` (or any other `$`-expansion) no longer expands either — `name:` needs a
hardcoded string instead of `${0:t}` (or can be left out: `compleo generate <script>` fills it
in, though `--help`/error text won't know the script's name until it's there).

Best of both: drop `name:` from the spec and pass `--name` on the command line instead, where
`${0:t}` still expands normally (it's not inside the heredoc):

```zsh
eval "$(compleo parse --name "${0:t}" -- "$@" <<'SPEC'
description: Print a greeting.
flags:
  --loud: {description: shout the greeting}
positionals:
  - name: name
    description: who to greet
    required: true
SPEC
)"
```

### Or: an external spec file

compleo just reads the spec from its stdin — a heredoc is only one way to supply it. A file
works too:

```zsh
eval "$(compleo parse -- "$@" < "${0:A:h}/greet.spec.yaml")"
```

## 2. Read the parsed values

After the `eval`, values are in a `compleo` associative array, keyed by flag/positional name
(the part after `--`/`-`, no dashes):

```zsh
greeting="Hello, ${compleo[name]}!"
[[ $compleo[loud] == true ]] && greeting=${(U)greeting}
```

- A plain flag or non-variadic positional is a scalar: `$compleo[name]`.
- A repeatable flag (`--tag*=`) or a `variadic: true` positional instead gets its own real
  zsh array, `$compleo_<name>` (not `$compleo[name]`) — assoc-array values can only be
  scalars. Non-alphanumeric characters in the name (e.g. the `-` in `dry-run`) become `_` in
  the array's variable name, since a literal `-` in a zsh variable name breaks referencing it.
- Booleans are always present as the literal string `"true"` or `"false"` — compare with
  `==`, not by checking whether the key exists.
- A count flag (`-v` repeated) is a plain integer string.

## What the `eval` can do on its own

Everything below happens from that one `eval` line — no extra code needed in the script:

- **`--help`**: prints the usage text and exits 0.
- **A bad argument** (unknown flag, missing required positional, a `flagtypes:`/`type:`
  mismatch): prints `<name>: <error>` to stderr and exits **2** (not compleo's own generic
  exit 1 for *its* invocation failures — a script checking `$?` can tell the two apart). No
  `|| exit $?` needed after the `eval` — exiting is already part of what gets `eval`'d.
- The one gap this doesn't cover: if the `compleo` binary itself can't even run (missing, not
  executable, ...), there's nothing to `eval` at all. That's the one case where an explicit
  exit-code check on the `eval` itself is the only way to catch a failure.

## 3. Enable completion

Two separate one-time steps — one per machine, one per script:

**Once per machine**, in `.zshrc`, *before* `compinit` runs:

```zsh
eval "$(compleo activate)"
```

This adds compleo's completions directory to `$fpath` (and writes compleo's own completion
there). `compinit`'s own `$fpath` scan only sees what's already there when it runs, hence
"before `compinit`." Re-running it later in an already-open shell also works — it activates
everything already in the directory immediately, the same trick `generate`'s hint uses.

**Once per script** (and again whenever its spec changes) — this is only needed for the
`generate` step itself; once generated, the script's own spec is saved alongside the
completion file, so Tab-completion afterward never re-runs the script and doesn't need
`$PATH` at all. `generate`'s argument can be a bare name resolved via `$PATH`, or a path
(`compleo generate ./greet`) that skips `$PATH` entirely:

```zsh
compleo generate greet
```

This writes `greet`'s completion file next to compleo's own, plus a hidden companion file
holding its spec (which the completion file points at). The command prints a one-line
confirmation plus a hint for activating it in your *current* shell immediately (a brand new
shell picks it up for free, since `compinit` scans `$fpath` at startup):

```
greet: generated completion at ~/.local/share/compleo/completions/_greet (new shells pick it
up automatically; to use it in this one now: autoload -Uz _greet && compdef _greet greet)
```

If Tab-completion for a wrapped script never seems to do anything (no error, no candidates),
the most likely causes are: `compleo generate <script>` was never run (or run before the most
recent edit — re-run it, and remember the spec it captured is frozen until you do), or
`$fpath`/`compinit` isn't wired up (see "Once per machine" above). Live completion itself
(like carapace's own completion snippets) suppresses errors, so these fail silently rather
than with a message.

## Full example

`examples/greet` end to end:

```zsh
#!/usr/bin/env zsh

eval "$(compleo parse -- "$@" <<SPEC
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
SPEC
)"

greeting="Hello, ${compleo[name]}!"
[[ $compleo[loud] == true ]] && greeting=${(U)greeting}

for ((i = 0; i < compleo[times]; i++)); do
  print -r -- "$greeting"
done
```

```sh
$ greet world --times 2 --loud
HELLO, WORLD!
HELLO, WORLD!
```

See `examples/macros` for a second full script demonstrating every core carapace-spec
completion macro, and [docs/spec.md](./spec.md) for everything that can go inside the spec.
