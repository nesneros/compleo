package bash

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// MinMajor and MinMinor are the oldest bash version compleo's generated completion works on:
// WrappedCompletion injects into carapace's own snippet, which ends in "unset COMPREPLY[-1]" —
// negative array subscripts, added in bash 4.3. (mapfile/compopt elsewhere in that snippet only
// need 4.0.) EmitResult itself needs nothing newer than bash 3.2.
const (
	MinMajor = 4
	MinMinor = 3
)

// VersionCommand is the "bash -c" script that reads BASH_VERSINFO without needing bash 4's
// associative arrays, so it also runs fine under the old bash it might be reporting on.
const VersionCommand = `printf '%s %s' "${BASH_VERSINFO[0]}" "${BASH_VERSINFO[1]}"`

// VersionSupported reports whether major.minor is at least MinMajor.MinMinor.
func VersionSupported(major, minor int) bool {
	return major > MinMajor || (major == MinMajor && minor >= MinMinor)
}

// ParseVersion parses VersionCommand's "<major> <minor>" output.
func ParseVersion(output string) (major, minor int, err error) {
	parts := strings.Fields(output)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected bash version output: %q", output)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing bash major version: %w", err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing bash minor version: %w", err)
	}
	return major, minor, nil
}

// DetectVersion runs the bash binary at path and reports its major.minor version. Bounded by a
// timeout since it execs a binary compleo doesn't control.
func DetectVersion(path string) (major, minor int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "-c", VersionCommand).Output()
	if err != nil {
		return 0, 0, err
	}
	return ParseVersion(string(out))
}
