package doctor

import (
	"errors"
	"os"
	"testing"

	_ "github.com/nesneros/compleo/internal/shells/bash" // registers the bash emitter
	_ "github.com/nesneros/compleo/internal/shells/zsh"  // registers the zsh emitter
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubDeps returns a Deps where everything fails/is-unset by default, so a test only has to
// override what it cares about — matching the real "nothing configured yet" environment.
func stubDeps() Deps {
	return Deps{
		Getenv:            func(string) string { return "" },
		LookPath:          func(string) (string, error) { return "", errors.New("not found") },
		ParentProcessName: func() (string, error) { return "", errors.New("no parent") },
		ParentInterpreter: func() (string, error) { return "", errors.New("no parent") },
		CompletionDir:     func() string { return "/nonexistent" },
		ReadDir:           func(string) ([]os.DirEntry, error) { return nil, errors.New("not found") },
		ReadFile:          func(string) ([]byte, error) { return nil, errors.New("not found") },
		RunCommand:        func(string, ...string) (string, error) { return "", errors.New("not found") },
		Executable:        func() (string, error) { return "/usr/local/bin/compleo", nil },
	}
}

func TestCollectReportsShellDetectionErrors(t *testing.T) {
	d := stubDeps()
	r := Collect(d)
	require.Error(t, r.Shell.InteractiveErr)
	require.Error(t, r.Shell.InterpreterErr)
	assert.False(t, r.Shell.InteractiveRegistered)
	assert.False(t, r.Shell.InterpreterRegistered)
}

func TestCollectFlagsUnregisteredInteractiveShell(t *testing.T) {
	d := stubDeps()
	d.ParentProcessName = func() (string, error) { return "fish", nil }
	r := Collect(d)
	assert.Equal(t, "fish", r.Shell.Interactive)
	assert.False(t, r.Shell.InteractiveRegistered)
}

func TestCollectAcceptsRegisteredInterpreter(t *testing.T) {
	d := stubDeps()
	d.ParentInterpreter = func() (string, error) { return "zsh", nil }
	r := Collect(d)
	assert.True(t, r.Shell.InterpreterRegistered)
}

func TestCollectReadsAllEnvVars(t *testing.T) {
	d := stubDeps()
	d.Getenv = func(name string) string {
		if name == "_COMPLEO_COMPLETION_SHELL" {
			return "zsh"
		}
		return ""
	}
	r := Collect(d)
	require.Len(t, r.Env, 6)
	for _, v := range r.Env {
		if v.Name == "_COMPLEO_COMPLETION_SHELL" {
			assert.True(t, v.Set)
			assert.Equal(t, "zsh", v.Value)
		} else {
			assert.False(t, v.Set)
		}
	}
}

func TestCollectReportsUnreadableCompletionDir(t *testing.T) {
	r := Collect(stubDeps())
	require.Error(t, r.CompletionDirErr)
	assert.Empty(t, r.Completions)
}

func TestCollectExcludesCompleoOwnCompletionFromPairing(t *testing.T) {
	d := stubDeps()
	d.ReadDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{fakeDirEntry{"_compleo"}}, nil
	}
	r := Collect(d)
	assert.Empty(t, r.Completions)
}

func TestCollectPairsCompletionAndSpecFiles(t *testing.T) {
	d := stubDeps()
	d.ReadDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{
			fakeDirEntry{"_greet"},
			fakeDirEntry{".greet.spec.yaml"},
			fakeDirEntry{"_orphan"},
		}, nil
	}
	d.LookPath = func(name string) (string, error) {
		if name == "greet" {
			return "/usr/local/bin/greet", nil
		}
		return "", errors.New("not found")
	}
	d.ReadFile = func(path string) ([]byte, error) {
		switch path {
		case "/some/dir/.greet.spec.yaml":
			return []byte("name: greet\nflags:\n  --loud: shout\n"), nil
		case "/usr/local/bin/greet":
			return []byte("#!/bin/zsh\n"), nil
		}
		return nil, errors.New("not found")
	}
	d.CompletionDir = func() string { return "/some/dir" }

	r := Collect(d)
	require.Len(t, r.Completions, 2)

	greet := r.Completions[0]
	assert.Equal(t, "greet", greet.Name)
	assert.True(t, greet.HasCompletionFile)
	assert.True(t, greet.HasSpecFile)
	assert.Equal(t, "/usr/local/bin/greet", greet.ScriptPath)
	assert.Equal(t, "zsh", greet.ScriptShell)
	assert.NoError(t, greet.SpecParseErr)

	orphan := r.Completions[1]
	assert.Equal(t, "orphan", orphan.Name)
	assert.True(t, orphan.HasCompletionFile)
	assert.False(t, orphan.HasSpecFile)
	assert.Empty(t, orphan.ScriptPath)
}

func TestCollectDetectsSpecThatNoLongerParses(t *testing.T) {
	d := stubDeps()
	d.ReadDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{fakeDirEntry{".broken.spec.yaml"}}, nil
	}
	d.ReadFile = func(string) ([]byte, error) {
		return []byte("name: broken\nunknownField: true\n"), nil
	}
	r := Collect(d)
	require.Len(t, r.Completions, 1)
	assert.Error(t, r.Completions[0].SpecParseErr)
}

func TestCollectReportsMissingMacroBinaries(t *testing.T) {
	d := stubDeps()
	d.Executable = func() (string, error) { return "/usr/local/bin/compleo", nil }
	d.ReadDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{fakeDirEntry{".tool.spec.yaml"}}, nil
	}
	d.ReadFile = func(string) ([]byte, error) {
		return []byte("name: tool\nflags:\n  --x=: {completion: [\"$definitelynotarealbinary123.foo\"]}\n"), nil
	}
	r := Collect(d)
	require.Len(t, r.Completions, 1)
	assert.Contains(t, r.Completions[0].MissingMacroBins, "definitelynotarealbinary123")
}

func TestCollectReportsCarapaceNotFound(t *testing.T) {
	r := Collect(stubDeps())
	assert.False(t, r.Carapace.Found)
}

func TestCollectReportsCarapaceVersion(t *testing.T) {
	d := stubDeps()
	d.LookPath = func(name string) (string, error) {
		if name == "carapace" {
			return "/opt/homebrew/bin/carapace", nil
		}
		return "", errors.New("not found")
	}
	d.RunCommand = func(path string, args ...string) (string, error) {
		return "v1.2.5", nil
	}
	r := Collect(d)
	assert.True(t, r.Carapace.Found)
	assert.Equal(t, "/opt/homebrew/bin/carapace", r.Carapace.Path)
	assert.Equal(t, "v1.2.5", r.Carapace.Version)
}

func TestCollectBashNotRelevantByDefault(t *testing.T) {
	r := Collect(stubDeps())
	assert.False(t, r.Bash.Relevant)
}

func TestCollectBashRelevantFromCompletionShellEnv(t *testing.T) {
	d := stubDeps()
	d.Getenv = func(name string) string {
		if name == "_COMPLEO_COMPLETION_SHELL" {
			return "bash"
		}
		return ""
	}
	r := Collect(d)
	assert.True(t, r.Bash.Relevant)
}

func TestCollectBashRelevantFromScriptShebang(t *testing.T) {
	d := stubDeps()
	d.ReadDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{fakeDirEntry{"_tool"}}, nil
	}
	d.LookPath = func(name string) (string, error) {
		if name == "tool" {
			return "/usr/local/bin/tool", nil
		}
		return "", errors.New("not found")
	}
	d.ReadFile = func(string) ([]byte, error) { return []byte("#!/bin/bash\n"), nil }
	r := Collect(d)
	assert.True(t, r.Bash.Relevant)
}

func TestCollectReadsBashVersion(t *testing.T) {
	d := stubDeps()
	d.Getenv = func(name string) string {
		if name == "_COMPLEO_COMPLETION_SHELL" {
			return "bash"
		}
		return ""
	}
	d.LookPath = func(name string) (string, error) {
		if name == "bash" {
			return "/bin/bash", nil
		}
		return "", errors.New("not found")
	}
	d.RunCommand = func(path string, args ...string) (string, error) { return "3 2", nil }
	r := Collect(d)
	assert.True(t, r.Bash.Found)
	assert.Equal(t, 3, r.Bash.Major)
	assert.Equal(t, 2, r.Bash.Minor)
}

// fakeDirEntry is a minimal os.DirEntry for tests that only need Name().
type fakeDirEntry struct{ name string }

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return false }
func (f fakeDirEntry) Type() os.FileMode          { return 0 }
func (f fakeDirEntry) Info() (os.FileInfo, error) { return nil, errors.New("not implemented") }
