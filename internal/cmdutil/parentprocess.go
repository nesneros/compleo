package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

// ParentProcessName reports the name of the process that started this one. For compleo, that
// process is almost always a shell: eval "$(compleo ...)" runs compleo as a direct child of the
// invoking shell, since command substitution forks the shell and execs compleo straight into
// that forked PID, with nothing else in between. The name alone doesn't say whether compleo can
// do anything with it — callers decide what a given name means, including rejecting one they
// don't recognize.
func ParentProcessName() (string, error) {
	self, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return "", fmt.Errorf("looking up current process: %w", err)
	}
	parent, err := self.Parent()
	if err != nil {
		return "", fmt.Errorf("looking up parent process: %w", err)
	}
	name, err := parent.Name()
	if err != nil {
		return "", fmt.Errorf("reading parent process name: %w", err)
	}
	// A truncated comm falls back to argv[0], where a login shell keeps its leading "-".
	return strings.TrimPrefix(name, "-"), nil
}

// ParentInterpreter reports the name of the executable actually running the parent process,
// for when that parent is a script's own shebang interpreter (the up/parse use case) rather
// than an interactive shell directly exec'd by a terminal (ParentProcessName's use case).
// On Linux, a shebang-invoked script's reported comm name is the script's own basename, not
// the interpreter's — the kernel's binfmt_script handler never updates it — so Name() would
// report e.g. "greet" instead of "zsh". Exe() reads /proc/pid/exe instead, which always
// resolves to the actual loaded binary (the interpreter), sidestepping that. Darwin has no
// such quirk, but Exe() reports the same answer there too.
func ParentInterpreter() (string, error) {
	self, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return "", fmt.Errorf("looking up current process: %w", err)
	}
	parent, err := self.Parent()
	if err != nil {
		return "", fmt.Errorf("looking up parent process: %w", err)
	}
	if exe, err := parent.Exe(); err == nil && exe != "" {
		return filepath.Base(exe), nil
	}
	cmdline, err := parent.CmdlineSlice()
	if err != nil || len(cmdline) == 0 || cmdline[0] == "" {
		return "", fmt.Errorf("reading parent process executable: %w", err)
	}
	return strings.TrimPrefix(filepath.Base(cmdline[0]), "-"), nil
}
