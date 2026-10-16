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

func versionString() string {
	if strings.Contains(version, "SNAPSHOT") || (version == "dev" && date != "" && commit != "") {
		return fmt.Sprintf("%s: %s [%v]", version, commit, date)
	}
	return version
}
