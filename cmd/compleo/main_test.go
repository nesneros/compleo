package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunWithNoArgsSucceeds(t *testing.T) {
	assert.NoError(t, run(nil))
}

func TestRunWithHelpFlagSucceeds(t *testing.T) {
	assert.NoError(t, run([]string{"--help"}))
}

func TestRunWithUnknownFlagFails(t *testing.T) {
	assert.Error(t, run([]string{"--bogus"}))
}
