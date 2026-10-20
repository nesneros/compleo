package shells

import (
	"bufio"
	"errors"
	"io"
	"path/filepath"
	"strings"
)

// ShebangShell reads r's first line and returns the shell it names.
// "#!/bin/zsh" and "#!/usr/bin/env zsh" both return "zsh".
func ShebangShell(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	// 256 characters, plus one byte so the newline fits in the buffer.
	scanner.Buffer(make([]byte, 257), 256)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
	}
	return shellFromShebang(scanner.Text())
}

// shellFromShebang returns the shell named by one shebang line.
func shellFromShebang(line string) (string, error) {
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "#!") {
		return "", errors.New("no shebang")
	}
	fields := strings.Fields(line[2:])
	if len(fields) == 0 {
		return "", errors.New("shebang names no interpreter")
	}
	// Shell shebangs do not pass env flags, so the next word is the program.
	if filepath.Base(fields[0]) == "env" {
		if len(fields) < 2 {
			return "", errors.New("shebang names no interpreter")
		}
		return filepath.Base(fields[1]), nil
	}
	return filepath.Base(fields[0]), nil
}
