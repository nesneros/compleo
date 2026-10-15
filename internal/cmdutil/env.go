package cmdutil

// Env var names compleo reads or sets.
const (
	// EnvCompletionDir is the directory completion files are read from and written to.
	EnvCompletionDir = "COMPLEO_COMPLETION_DIR"
	// EnvGenerateName is the name a generated completion is registered under. Its presence
	// also selects compleo's generate behavior beyond parsing "$@".
	EnvGenerateName = "_COMPLEO_GENERATE_NAME"
	// EnvSpecFile is the path to the spec file a completion is built from.
	EnvSpecFile = "_COMPLEO_SPEC_FILE"
	// EnvDebugLog is a file path compleo appends errors to.
	EnvDebugLog = "_COMPLEO_DEBUG_LOG"
)
