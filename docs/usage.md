# Using compleo in a script

This is about wiring a zsh script up to compleo — the line you add to the script itself.
For what you can put in the spec (flags, positionals, types, completion), see
[docs/spec.md](./spec.md).

## 1. Add the line

Near the top of the script, before anything that uses its arguments:

```zsh
eval "$(compleo up <<'SPEC'
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

That's it — `compleo up` reads the spec, fills in the script's own name and `"$@"`, and
prints zsh that the outer `eval` runs, setting the parsed values (see step 2). `--help`, bad
arguments, and errors are all handled automatically from this one line; nothing else is
needed for those (see "What the `eval` can do on its own," below).

The heredoc is quoted (`<<'SPEC'`), so nothing in it needs escaping, not even `$` or `` ` ``
in description text. `up` is the convenient form — for finer control over the invocation
itself, use `compleo parse --name="${0:t}" -- "$@"` directly instead.

### Or: an external spec file

compleo just reads the spec from its stdin — a heredoc is only one way to supply it. A file
works too, and still doesn't need its own `name:` (`up` fills that in):

```zsh
eval "$(compleo up < "${0:A:h}/greet.spec.yaml")"
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
- **A bad argument** (unknown flag, missing required positional, a `type:`
  mismatch): prints `<name>: <error>` to stderr and exits **2** (not compleo's own generic
  exit 1 for *its* invocation failures — a script checking `$?` can tell the two apart). No
  `|| exit $?` needed after the `eval` — exiting is already part of what gets `eval`'d.
- The one gap this doesn't cover: if the `compleo` binary itself can't even run (missing, not
  executable, ...), there's nothing to `eval` at all — an explicit exit-code check on the
  `eval` is the only way to catch it. `up` re-invokes `compleo parse` internally, so in
  principle this gap applies twice; in practice, if the first call ran, the second will too.

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
holding its spec (which the completion file points at). A brand new shell picks it up for
free (`compinit` scans `$fpath` at startup); to activate it in your *current* shell too,
the command tells you exactly how:

```
To activate completion for greet in this shell, run: autoload -Uz _greet && compdef _greet greet
```

If Tab-completion for a wrapped script never seems to do anything (no error, no candidates),
the most likely causes are: `compleo generate <script>` was never run (or run before the most
recent edit — re-run it, and remember the spec it captured is frozen until you do), or
`$fpath`/`compinit` isn't wired up (see "Once per machine" above). Live completion itself
(like carapace's own completion snippets) suppresses errors, so these fail silently rather
than with a message. To actually see what went wrong, set `_COMPLEO_DEBUG_LOG` to a real file
path (not `/dev/stderr` — it won't survive the suppression) before pressing Tab again, then
check that file:

```zsh
_COMPLEO_DEBUG_LOG=/tmp/compleo-debug.log
# press Tab, then:
cat /tmp/compleo-debug.log
```

## Full example

`examples/greet` end to end:

```zsh
#!/usr/bin/env zsh

eval "$(compleo up <<'SPEC'
description: Print a greeting.
flags:
  --loud: {description: shout the greeting}
  --times=: {description: how many times to repeat, default: "1", type: {kind: int, min: 1}}
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

See `examples/extensions` for a script demonstrating every `positionals:`/`type:` combination
compleo adds on top of plain carapace-spec, `examples/macros` for a second full script
demonstrating every core carapace-spec completion macro (except `$spec`, split out into
`examples/other_spec` since it needs a dynamic path a quoted heredoc can't supply), and
[docs/spec.md](./spec.md) for everything that can go inside the spec.
