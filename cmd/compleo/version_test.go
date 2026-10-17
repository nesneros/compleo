package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionStringDefaultWithoutStampReturnsDev(t *testing.T) {
	withVersion(t, "dev", "", "")
	assert.Equal(t, "dev", versionString())
}

func TestVersionStringDefaultWithStampIncludesDateAndCommit(t *testing.T) {
	withVersion(t, "dev", "9e9877ba", "2026-10-12T21:18:04Z")
	assert.Equal(t, "dev: 9e9877ba [2026-10-12T21:18:04Z]", versionString())
}

func TestVersionStringDefaultWithOnlyCommitReturnsDev(t *testing.T) {
	withVersion(t, "dev", "9e9877ba", "")
	assert.Equal(t, "dev", versionString())
}

func TestVersionStringSnapshotIncludesDateAndCommit(t *testing.T) {
	withVersion(t, "1.2.3-SNAPSHOT-9e9877b", "9e9877ba", "2026-10-12T21:18:04Z")
	assert.Equal(t, "1.2.3-SNAPSHOT-9e9877b: 9e9877ba [2026-10-12T21:18:04Z]", versionString())
}

func TestVersionStringReleaseReturnsVersion(t *testing.T) {
	withVersion(t, "1.2.3", "9e9877ba", "2026-10-12T21:18:04Z")
	assert.Equal(t, "1.2.3", versionString())
}

func withVersion(t *testing.T, v, c, d string) {
	t.Helper()
	prevVersion, prevCommit, prevDate := version, commit, date
	version, commit, date = v, c, d
	t.Cleanup(func() {
		version, commit, date = prevVersion, prevCommit, prevDate
	})
}
