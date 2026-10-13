# compleo

A Go tool for zsh scripts. One spec file is the single source of truth for three things:

1. **Parsing** the script's command line
2. **Usage / help message**
3. **zsh completion**

The script calls compleo near the top; compleo parses `"$@"` and prints zsh code that the script `eval`s to set variables. Completion is generated from the same spec, so the three never drift apart.

## Status

Early design phase, but the core pipeline is implemented: `go.mod` at `github.com/nesneros/compleo`, `cmd/compleo/`, `internal/spec`, `internal/parse`, `internal/zsh`, mise tooling, GoReleaser + GitHub Actions CI. Spec parsing, cobra/usage building, command-line parsing, zsh emission, and completion generation (both compleo's own and a wrapped script's) all work end to end — see `examples/greet` and `examples/macros`. Treat anything under "Open questions" as undecided and ask before choosing.

## Design decisions

- **Language:** Go.
- **Target shell:** zsh only. Emit zsh syntax directly; no bash/POSIX compatibility needed. Design should stay open to extension with e.g. bash later.
- **Completion:** built on carapace, for its rich completion actions (hosts, ssh keys, brew formulae, files, ...). Spec fields for completion stay as close as possible to the carapace-spec format.
- **Macro names:** use the same names as carapace-spec/carapace-bin (e.g. `$files`, `$carapace.tools...`) — don't duplicate that naming knowledge.
- **Parsing and usage are extensions:** carapace-spec's flags have `required`/`default` but nothing for flag *types* beyond string/[]string/bool/count, and nothing structured for positionals at all. `spec.Command` adds `Positionals` (embeds its own `Type` per item — purely compleo's type, nothing to reuse) and `FlagTypes map[string]Type` (a side-channel map, not a field on each flag: carapace's flag-syntax YAML parsing is unexported with no extension hook, so compleo can't fork a custom `Flag` struct into it).
- **Result delivery:** the Go program prints a zsh script; the calling script `eval`s it. Parsed values go into an associative array.
- **Architecture:** spec is built into an in-memory `cobra.Command` at runtime via `spec.Command`'s private `toCarapace().ToCobraE()`. Not carapace's `--codegen`: that writes static per-command `.go` stubs for an offline compile step, which doesn't fit "one generic binary, arbitrary spec read fresh every invocation."
- **Parsing engine:** cobra/pflag via `ToCobraE()`, not a custom parser. Cobra/pflag only register real flags for `string`/`[]string`/`bool`/`count`; other `flagtypes:`/positional `type:` values are plain strings to them, so `internal/parse`'s `bindFlags`/`bindPositionals` validate those itself via `spec.Type.Validate` — the only place an invalid type declaration is ever caught. Positional arity/required-ness/binding is entirely `internal/parse`'s own (cobra's `Args` is always `cobra.ArbitraryArgs`). A typed flag's `--help` shows its declared type (e.g. `` (`int`) ``) appended to the description by `addFlagTypeHints`, since pflag's `UnquoteUsage` looks for a backtick-quoted segment and uses it as the displayed type instead of `flag.Value.Type()`.
- **Spec delivery at runtime:** the target script's spec is read from compleo's stdin, not a file-path argument — lets the calling script supply it as a heredoc, inline variable, or external file it cats in. Consequence: in `parse` mode, compleo's wrapped-script argv is entirely the wrapped script's own arguments.
- **compleo's own CLI vs. `parse` mode:** `parse` is a real subcommand in `cmd/compleo/spec.yaml`, like `version`/`validate`/`completion`/`activate` — flags: `--dump`; positionals: one variadic `args`, absorbing the wrapped script's entire argv. It's dispatched through the exact same generic path as every other compleo subcommand (`parse.Parse(ownCmd, ownSpec, args)`, then `switch result.Path[last]`); `run()`'s `case "parse":` pulls `result.Positionals["args"].Values` and `result.Flags["dump"].Bool` and calls `runEmit`/`runGenerate`/`runComplete` depending on `COMPLEO_MODE` (see below).
  - `"--"` is optional: it's only needed to disambiguate a wrapped script's own flag from one of compleo's own `parse`-level flags (`--dump`, cobra's auto `--help`) — e.g. `compleo parse -- --dump` makes `--dump` the wrapped script's own. With no collision, `compleo parse world` works with no `--`. This is ordinary pflag behavior (`parseArgs` stops interpreting flags at a bare `--`), not hand-rolled parsing — and it means `compleo parse --help` (before any `--`) shows `parse`'s own flag docs for free.
  - A variadic positional with zero bound values comes back as `nil` (not `[]string{}`) from `bindPositionals`; `parse.Parse` panics on a nil argv. `run()`'s `case "parse":` normalizes `nil` to `[]string{}` before dispatching, since a wrapped script with zero of its own arguments is common, not an edge case.
  - `version`/`validate`/`completion`/`activate` are subcommands on compleo's own CLI, not root flags, so they read the same as any other spec subcommand. There's no separate `help` subcommand, only cobra's own `--help`/`-h`, available on every (sub)command — compleo doesn't fight cobra's automatic `--help` (no documented per-command opt-out exists), so `compleo validate --help` works too, same as `git commit --help`.
  - `--help` is intercepted by cobra before `RunE` runs. `internal/parse`'s `helpRequested` recurses the whole command tree checking `Flags().Changed("help")` and `Parse` sets `Result.HelpRequested` from it right after `cmd.Execute()` succeeds — this can't be inferred from `Path`/`Flags` being empty, because cobra converts `flag.ErrHelp` to a `nil` error before `Execute()` returns, so success alone doesn't say whether the matched command's `RunE` actually ran. A genuine `parse.Parse` error and `HelpRequested` can never both be true.
- **Wrapped-script completion install location:** `compleo activate` prints zsh that `mkdir -p`s the completions directory and does `fpath+=(...)` — meant to run in `.zshrc` **before** `compinit` (`compinit`'s directory scan only sees what's already on `$fpath` when it runs). The directory (`completionDir()`) is `$COMPLEO_COMPLETION_DIR` if set, else `${XDG_DATA_HOME:-~/.local/share}/compleo/completions`.
  - `activate` also writes compleo's own completion to `completionDir()/_compleo` via `writeCompletionFile`, which skips the write when content is unchanged — needed because `activate` may run on every shell startup, and touching the file every time would bump the directory's mtime, which is what makes `compinit` rescan `fpath` instead of trusting its `.zcompdump` cache. This depends on `zsh.Completion`/`Emit` being deterministic for identical input (covered by `TestCompletionIsDeterministic`).
  - No extra "make this autoload" step is needed beyond the directory being on `$fpath`: `compinit` only reads each file's `#compdef` first line at scan time, and only runs the body lazily on that command's first Tab-press.
- **Quoting:** `internal/zsh.Quote` (single-quote style, escaping embedded `'` via `'\''`) is the one quoting function used everywhere zsh is emitted.
- **`internal/zsh.Emit`:** a fixed associative array named `compleo` holds every scalar flag's/positional's value (`compleo[name]=value`); a repeatable flag or variadic positional gets its own real zsh array (`compleo_<name>=(...)`) instead, since assoc-array values are scalars only. Booleans are always present as `"true"`/`"false"` (not presence/absence); count flags render as a plain integer string.
  - Assoc-array keys must stay unquoted (bareword) — `compleo['loud']=x` stores the literal key `"'loud'"`, not `"loud"`. Only the value needs `Quote()`; names come from the spec, always simple tokens.
  - A separate array's variable name is sanitized (every non-`[A-Za-z0-9_]` char, e.g. a flag's `-`, becomes `_`), not quoted — a literal hyphen in a zsh variable name breaks even referencing it later (`${my-array[1]}` parses as `${param-default}`, not a subscript). This can collide two differently-sanitized names onto one array; compleo doesn't guard against it — keep spec names distinct.
  - Iteration over `result.Flags`/`result.Positionals` is sorted (`sortedKeys`) for deterministic output, which `activate`'s skip-if-unchanged write depends on.
- **`--help` in `parse` mode:** cobra's `HelpFunc` writes straight to `OutOrStdout()` — the same stream the wrapped script evals as zsh. `runEmit` buffers the wrapped `cmd`'s output (`cmd.SetOut(&helpBuf)`), and if `HelpRequested`, emits `zsh.EmitHelp(helpBuf.String())` instead of `zsh.Emit`. `EmitHelp` wraps the text as `print -r -- <Quote(text)>\nexit 0` — `exit`, not `return`, since `return` inside `eval`'d code only exits an enclosing function if the script wraps its logic in one. `cmd.SilenceErrors`/`SilenceUsage` (set on the wrapped `cmd` too) stop cobra from also writing its own `Error:`/`Usage:` straight to the real stderr alongside compleo's own reported error.
- **Parse errors in `parse` mode are encoded into the output, not returned as a Go error:** `zsh.EmitError(name, err)` formats `"name: err"` itself, prints `print -u2 -- <Quote(...)>\nexit 2`, and returns nil. Exit 2 specifically (not `main()`'s generic exit 1), so a script checking `$?` can tell the two apart. `name` is the spec's `name:` field, not the real `$0` (compleo never sees it — `argv` is literally `"$@"`; `examples/greet` interpolates `${0:t}` into its own `name:` for this). This means a wrapped script needs only `eval "$(compleo parse -- "$@" <<SPEC ... SPEC)"`, no separate `|| exit $?` — eval'ing the output alone stops the script. Gap this doesn't cover: if compleo itself fails to even launch, there's no output to eval at all — an explicit exit-code check is the only way to catch that.
- **`internal/spec`'s API surface:** `ParseYAML` decodes and builds the cobra tree in one call — the one entry point everything uses. `decodeYAML`/`toCarapace`/`Apply` are private.
- **YAML library pin matters:** must use `gopkg.in/yaml.v3`, not `go.yaml.in/yaml/v3` — the pinned `carapace-sh/carapace v1.16.3`'s `command.FlagSet`/`command.Run` have custom `UnmarshalYAML` methods tied to that specific package's `*yaml.Node` type; the other library's decoder silently won't find them. Carapace migrates to `go.yaml.in/yaml/v3` after this tag — revisit this pin together.
- **Versioning:** follows carapace-bin's pattern — `version`/`commit`/`date` vars in `cmd/compleo/version.go`, populated by GoReleaser's default ldflags. `versionString()` appends `(date) [commit]` when `version` contains "SNAPSHOT", or when `version` is still `dev` and both `date`/`commit` are set.
- **Canonical carapace org & versions:** the project lives at `carapace-sh`; `rsteube/*` repos just redirect there. Pinned: `carapace-spec v1.9.0`, `carapace v1.16.3`, `cobra v1.10.2`, `pflag v1.0.10`.
- **Generating and serving a wrapped script's own completion:** `COMPLEO_MODE` is an env var `main()` reads once and threads through as a plain `mode string` parameter (not a hidden `os.Getenv` read deeper in the call chain). It changes what the wrapped script's existing, unconditional `eval "$(compleo parse -- "$@" <<SPEC ...)"` line does, with no mode-aware logic needed in the script itself:
  - unset (`""`): parse `"$@"` against the piped spec, as above (`runEmit`).
  - `generate`: build the wrapped script's completion snippet (`zsh.WrappedCompletion`) and write it to `completionDir()/_<name>` (skip-if-unchanged, like `activate`'s own `_compleo`), then report success as self-exiting zsh (`zsh.EmitGenerated`). Triggered by hand: `COMPLEO_MODE=generate ./greet` once, or after a spec change. User-facing (mirrors Click's `_FOO_COMPLETE=zsh_source foo`).
  - `complete`: serve one live Tab-press completion request (`runComplete`), using the same piped spec a normal run gets. Internal-only — nothing a human types by hand, only ever set by the generated completion file's own re-invocation.
  - Why: carapace's own generated snippet always re-invokes `uid.Executable() + " _carapace zsh"` — literally `compleo _carapace zsh <words>`, since `uid.Executable()` returns whatever binary is actually running, not the completed command. For a wrapped script that reaches compleo with no spec on stdin at all. `zsh.WrappedCompletion(cmd)` takes carapace's own generated snippet and replaces that one substring — `"xargs " + uid.Executable() + " _carapace zsh"` becomes `"COMPLEO_MODE=complete xargs " + cmd.Name()` — so the generated `_greet` file's Tab-press callback re-invokes `greet` itself, whose own top-of-script `eval` line runs again and re-delivers the spec via its heredoc exactly like a normal run, just with `COMPLEO_MODE=complete` set. No second spec-delivery mechanism needed. `WrappedCompletion` errors if the expected substring isn't found, rather than silently emitting a snippet whose callback still dispatches into compleo.
  - `runComplete` dispatches directly into `cmd`'s own hidden `_carapace` subcommand (`cmd.SetArgs(append([]string{"_carapace", "zsh"}, args...))`) and wraps its raw wire-format output via `zsh.EmitComplete` (same `print -r -- <Quote(...)>\nexit 0` shape as `EmitHelp`) — needed because the wrapped script's `eval` is unconditional and would otherwise try to execute carapace's wire format as zsh code instead of just reproducing it for the generated completion function's own capture.
  - A wrapped script only gets working live completion if it's resolvable on `$PATH` under the name in its `name:` field — the generated snippet's `xargs <name>` re-invocation needs to find it, and (like carapace's own snippet) suppresses errors (`2>/dev/null`), so an unresolvable name fails silently. `mise.toml`'s `_.path` adds `examples/` to `$PATH` in a mise-activated shell for exactly this reason.

## Open questions (do not decide silently)

- Whether the spec is a separate YAML file or annotations in the script.
- Whether to adopt `replace github.com/spf13/pflag => github.com/carapace-sh/carapace-pflag v1.3.0`, needed for full flag-syntax support (shorthand-only flags, non-posix single-dash flags, `nargs`-consuming flags) — without it those forms silently fail to register. Tradeoff: a pflag-fork dependency vs. degraded flag-syntax coverage.
- Nothing yet orchestrates running `COMPLEO_MODE=generate` across every wrapped script a user has — today it's a manual, per-script step. Whether `activate` (or something else) should ever discover and regenerate wrapped scripts' completions in bulk is undecided.

## Things to verify before relying on them

- The exact name and signature of carapace-spec's macro registration function.
- That current carapace-bin still generates `pkg/actions/actions_generated.go` the way v1.0.2 does.
- Whether carapace-spec's YAML decoder tolerates unknown fields — matters if the same enriched spec file (with compleo's `positionals:`/`flagtypes:` extensions) is ever meant to also be consumable directly by plain carapace-bin/carapace-spec.
- Importing `pkg/actions` links every action package; check the resulting binary size.

## Tooling: mise, not make

Use **mise** for tool versions and tasks. Do not add a Makefile.

- Tool versions (Go, golangci-lint, goreleaser, act) are pinned in `mise.toml` under `[tools]`.
- Simple tasks live in `mise.toml` under `[tasks.*]`; longer scripts are file tasks under `mise-tasks/` (e.g. `mise-tasks/ci-local`) with a `#MISE description=...` header — both run with `mise run <task>`.
- Tasks: `build`, `test`, `lint`, `fmt`, `check` (fmt check, vet, lint, test), `install`, `release-check` (`goreleaser check`, no build), `release-dry-run` (local snapshot build, no publish), `ci-local` (runs `ci.yml` locally via `act`, Docker required).
- If a task you need doesn't exist, add it rather than documenting a raw command here.
- `mise.toml`'s `[env]`: `COMPLEO_COMPLETION_DIR={{config_root}}/.completions` so a mise-activated shell (or `mise x -- ...`) writes into the project instead of the real `~/.local/share/compleo/completions` (`.completions/` is gitignored). `_.path = ["{{config_root}}/examples"]` puts the demo scripts (`greet`, `macros`) on `$PATH` by bare name, since their generated completions need that to work at all (see "Generating and serving a wrapped script's own completion" above).

## Release & CI

- `.goreleaser.yml`: builds `cmd/compleo` for linux/darwin × amd64/arm64. Relies on GoReleaser's implicit default ldflags (see "Versioning" above) rather than declaring its own.
- `.github/workflows/ci.yml`: `check-ubuntu`/`check-macos` (literal `runs-on`, **not** `strategy: matrix` — see below) run `mise run check`; `release-check` validates `.goreleaser.yml` on every PR.
- `.github/workflows/release.yml`: runs GoReleaser on `v*` tag pushes.
- `.github/dependabot.yml`: weekly `gomod` + `github-actions` update PRs.
- **act gotchas** (all handled in `mise-tasks/ci-local`, don't re-break them):
  - act doesn't read Docker contexts; `DOCKER_HOST` must be set explicitly from `docker context inspect --format '{{.Endpoints.docker.Host}}'`.
  - `runs-on: ${{ matrix.os }}` under a `strategy.matrix` races in act when multiple matrix entries evaluate the expression concurrently (a shared-YAML-node mutation bug in act's `EvaluateYamlNode`) — why `ci.yml` uses two literal jobs instead of a matrix over `os`.
  - `--container-daemon-socket -` disables bind-mounting the Docker socket into job containers — required under any VM-based runtime (Colima, Rancher Desktop), where the host socket path isn't valid inside the VM.
  - `act push` runs every workflow with a `push` trigger, ignoring `release.yml`'s `tags: ["v*"]` filter — `ci-local` scopes to `-W .github/workflows/ci.yml` so it doesn't also attempt a real `goreleaser release`.

## Conventions

- Format with `gofmt`; run `go vet` and the configured linter before finishing a change.
- Prefer separate top-level `Test...` functions over table-driven tests, even when it means similar-looking functions repeated — this project's explicit preference, not a Go-wide default.
- Use `testify` (`require`/`assert`) in tests rather than hand-rolled `if err != nil { t.Fatalf }` checks. `require` for preconditions the rest of the test depends on; `assert` for independent checks that should all get reported even if one fails.
- The emitted zsh code is the main contract. Cover it with golden-file tests, and execute the output under `zsh -f` in at least some tests.
- Quoting must be rigorous: arguments can contain spaces, quotes, `$`, backticks, and newlines. Never build emitted shell by naive string concatenation; use one tested quoting function.
- Current package split: `internal/spec` (YAML parsing, types, and building the spec's cobra/completion/usage tree — usage can't be its own package without an import cycle, since it needs `spec.Command`), `internal/parse` (binds a parsed command line to a `Result`), `internal/zsh` (emits the script). Each still testable alone.
- Keep code comments short — one line where possible. Explain the non-obvious *why*, not a paragraph of it.
