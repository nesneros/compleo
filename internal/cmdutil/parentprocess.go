package cmdutil

import (
	"fmt"
	"os"
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
