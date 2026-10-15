package main

import (
	"fmt"
	"strings"
)

// version, commit, date are populated via linker flags at build time.
var (
	version = "dev"
	commit  string
	date    string
)

// versionString appends (date) [commit] for SNAPSHOT builds, and for the default dev version once both are set.
func versionString() string {
	if strings.Contains(version, "SNAPSHOT") || (version == "dev" && date != "" && commit != "") {
		return fmt.Sprintf("%s (%v) [%v]", version, date, commit)
	}
	return version
}
