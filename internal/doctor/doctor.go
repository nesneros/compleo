// Package doctor collects facts about a compleo installation's environment for diagnosis.
// Collect gathers data; Format and Severity (format.go) render it and judge severity. Collect
// never panics and never mutates anything — every lookup turns a failure into a Report field
// rather than a returned error, so a broken setup (not activated, carapace missing, a spec that
// no longer parses, ...) shows up as a finding instead of crashing doctor itself.
package doctor

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nesneros/compleo/internal/cmdutil"
	"github.com/nesneros/compleo/internal/generate"
	"github.com/nesneros/compleo/internal/shells"
	"github.com/nesneros/compleo/internal/shells/bash"
	"github.com/nesneros/compleo/internal/spec"
)

// Deps are Collect's dependencies, injected so it's testable (its own parent process is never
// a real shell under "go test") and so every check fails softly instead of calling os.Exit or
// panicking — unlike shells.GetCompletionEmitter, which Collect must never call.
type Deps struct {
	Getenv            func(string) string
	LookPath          func(string) (string, error)
	ParentProcessName func() (string, error)
	ParentInterpreter func() (string, error)
	CompletionDir     func() string
	ReadDir           func(string) ([]os.DirEntry, error)
	ReadFile          func(string) ([]byte, error)
	RunCommand        func(path string, args ...string) (string, error)
	Executable        func() (string, error)
}

// DefaultDeps wires Deps to the real OS, process table, and cmdutil.
func DefaultDeps() Deps {
	return Deps{
		Getenv:            os.Getenv,
		LookPath:          exec.LookPath,
		ParentProcessName: cmdutil.ParentProcessName,
		ParentInterpreter: cmdutil.ParentInterpreter,
		CompletionDir:     cmdutil.CompletionDir,
		ReadDir:           os.ReadDir,
		ReadFile:          os.ReadFile,
		RunCommand:        runCommand,
		Executable:        os.Executable,
	}
}

// runCommand runs path with args and returns its trimmed stdout. Bounded by a timeout since
// it runs an external binary (carapace, bash) doctor doesn't control.
func runCommand(path string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, args...).Output()
	return strings.TrimSpace(string(out)), err
}

// Report is plain data about the environment — no formatting or severity judgment baked in.
// Format and Severity (format.go) derive text and warning/error counts from it.
type Report struct {
	// CompleoVersion is set by the caller (main package owns versionString()), not by Collect.
	CompleoVersion string

	Shell            ShellInfo
	Env              []EnvVar
	CompletionDir    string
	CompletionDirErr error
	Completions      []CompletionEntry
	Carapace         CarapaceInfo
	Bash             BashInfo
}

// ShellInfo reports both of compleo's shell-detection mechanisms (see AGENTS.md's "Two
// detection mechanisms") as doctor itself observes them — doctor's own parent is the real
// interactive shell, so Interactive and Interpreter normally agree here even though a script's
// own "parse"/"up" invocation is exactly the case where they can differ.
type ShellInfo struct {
	Interactive           string
	InteractiveErr        error
	InteractiveRegistered bool
	Interpreter           string
	InterpreterErr        error
	InterpreterRegistered bool
	// CompletionShellEnv is the raw _COMPLEO_COMPLETION_SHELL value; "" means activate hasn't
	// exported it into this shell (or this isn't the shell it was exported into).
	CompletionShellEnv string
}

// EnvVar is one compleo-recognized environment variable and its current value.
type EnvVar struct {
	Name  string
	Value string
	Set   bool
}

// CompletionEntry describes one wrapped script's generated completion, by name.
type CompletionEntry struct {
	Name              string
	HasCompletionFile bool
	HasSpecFile       bool
	// ScriptPath is where Name resolves on $PATH; empty means it wasn't found there.
	ScriptPath string
	// ScriptShell is the shebang interpreter read from ScriptPath; empty if undetermined.
	ScriptShell string
	// SpecParseErr is non-nil if the companion spec file no longer parses under this compleo
	// binary (e.g. after an upgrade that tightened spec validation).
	SpecParseErr error
	// MissingMacroBins are binaries the spec's completion macros would exec but aren't on $PATH.
	MissingMacroBins []string
}

// CarapaceInfo is the external carapace binary many carapace-bin macros ($carapace.*) exec at
// Tab time — a separate, runtime dependency from the carapace-spec/carapace Go modules
// compiled into compleo itself.
type CarapaceInfo struct {
	Found      bool
	Path       string
	Version    string
	VersionErr error
}

// BashInfo is only meaningful when Relevant: compleo's bash completion needs bash 4.3
// (complete -o noquote, plus carapace's mapfile). A script's own eval only needs bash 3.2.
type BashInfo struct {
	// Relevant is true when the completion-target shell is bash, or some generated
	// completion's own script shebang is bash — i.e. this check matters here at all.
	Relevant bool
	Found    bool
	Path     string
	// Major is 0 when bash was found but its version couldn't be read. Minor is the
	// BASH_VERSINFO minor (0 is a real minor, as in bash 4.0).
	Major int
	Minor int
}

// Collect gathers a Report by querying d. It never panics, exits, or writes anything.
func Collect(d Deps) Report {
	var r Report

	r.Shell = collectShell(d)

	for _, name := range []string{
		cmdutil.EnvCompletionDir, cmdutil.EnvCompletionShell, cmdutil.EnvGenerateName,
		cmdutil.EnvScriptFile, cmdutil.EnvSpecFile, cmdutil.EnvDebugLog,
	} {
		v := d.Getenv(name)
		r.Env = append(r.Env, EnvVar{Name: name, Value: v, Set: v != ""})
	}

	r.CompletionDir = d.CompletionDir()
	entries, err := d.ReadDir(r.CompletionDir)
	r.CompletionDirErr = err
	if err == nil {
		r.Completions = collectCompletions(d, r.CompletionDir, entries)
	}

	r.Carapace = collectCarapace(d)
	r.Bash = collectBash(d, r.Shell, r.Completions)

	return r
}

func collectShell(d Deps) ShellInfo {
	var s ShellInfo
	registered := make(map[string]bool)
	for _, n := range shells.RegisteredNames() {
		registered[n] = true
	}

	s.Interactive, s.InteractiveErr = d.ParentProcessName()
	s.InteractiveRegistered = s.InteractiveErr == nil && registered[s.Interactive]

	s.Interpreter, s.InterpreterErr = d.ParentInterpreter()
	s.InterpreterRegistered = s.InterpreterErr == nil && registered[s.Interpreter]

	s.CompletionShellEnv = d.Getenv(cmdutil.EnvCompletionShell)
	return s
}

// collectCompletions pairs up completion files ("_name") and companion spec files
// (".name.spec.yaml") in dir by name. "_compleo" (compleo's own completion) is excluded: it has
// no companion spec file by design, so it would otherwise always show as mispaired.
func collectCompletions(d Deps, dir string, entries []os.DirEntry) []CompletionEntry {
	hasCompletion := make(map[string]bool)
	hasSpec := make(map[string]bool)
	for _, e := range entries {
		n := e.Name()
		switch {
		case strings.HasPrefix(n, "_"):
			name := strings.TrimPrefix(n, "_")
			if name == "compleo" {
				continue
			}
			hasCompletion[name] = true
		case strings.HasPrefix(n, ".") && strings.HasSuffix(n, ".spec.yaml"):
			hasSpec[strings.TrimSuffix(strings.TrimPrefix(n, "."), ".spec.yaml")] = true
		}
	}

	names := make(map[string]bool, len(hasCompletion)+len(hasSpec))
	for n := range hasCompletion {
		names[n] = true
	}
	for n := range hasSpec {
		names[n] = true
	}
	sortedNames := make([]string, 0, len(names))
	for n := range names {
		sortedNames = append(sortedNames, n)
	}
	sort.Strings(sortedNames)

	self := "compleo"
	if exe, err := d.Executable(); err == nil {
		self = filepath.Base(exe)
	}

	out := make([]CompletionEntry, 0, len(sortedNames))
	for _, name := range sortedNames {
		c := CompletionEntry{Name: name, HasCompletionFile: hasCompletion[name], HasSpecFile: hasSpec[name]}
		if path, err := d.LookPath(name); err == nil {
			c.ScriptPath = path
			if content, err := d.ReadFile(path); err == nil {
				if shell, err := shells.ShebangShell(bytes.NewReader(content)); err == nil {
					c.ScriptShell = shell
				}
			}
		}
		if c.HasSpecFile {
			content, err := d.ReadFile(filepath.Join(dir, "."+name+".spec.yaml"))
			if err != nil {
				c.SpecParseErr = err
			} else if _, sp, perr := spec.ParseYAML(string(content), ""); perr != nil {
				c.SpecParseErr = perr
			} else {
				c.MissingMacroBins = generate.MissingMacroBins(sp, self)
			}
		}
		out = append(out, c)
	}
	return out
}

func collectCarapace(d Deps) CarapaceInfo {
	var c CarapaceInfo
	path, err := d.LookPath("carapace")
	if err != nil {
		return c
	}
	c.Found = true
	c.Path = path
	c.Version, c.VersionErr = d.RunCommand(path, "--version")
	return c
}

func collectBash(d Deps, s ShellInfo, completions []CompletionEntry) BashInfo {
	var b BashInfo
	b.Relevant = s.CompletionShellEnv == "bash"
	for _, c := range completions {
		if c.ScriptShell == "bash" {
			b.Relevant = true
		}
	}
	if !b.Relevant {
		return b
	}

	path, err := d.LookPath("bash")
	if err != nil {
		return b
	}
	b.Found = true
	b.Path = path
	if out, err := d.RunCommand(path, "-c", bash.VersionCommand); err == nil {
		if maj, min, err := bash.ParseVersion(out); err == nil {
			b.Major, b.Minor = maj, min
		}
	}
	return b
}
