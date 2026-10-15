package cmdutil

// Env var names compleo reads or sets. A leading "_" marks internal plumbing (compleo setting
// something for compleo to read back, or a by-hand-triggerable mechanism) rather than a
// durable user-facing setting — only EnvCompletionDir lacks one.
const (
	// EnvCompletionDir is the directory completion files are read from and written to.
	EnvCompletionDir = "COMPLEO_COMPLETION_DIR"
	// EnvGenerateName is the name a generated completion is registered under. Its presence
	// also selects compleo's generate behavior beyond parsing "$@".
	EnvGenerateName = "_COMPLEO_GENERATE_NAME"
	// EnvSpecFile is the path to the spec file a completion is built from.
	EnvSpecFile = "_COMPLEO_SPEC_FILE"
	// EnvDebugLog is a file path compleo appends errors to. Must be a real file, not
	// /dev/stderr: the one scenario this exists for (a live Tab press, via carapace's own
	// generated "xargs ... 2>/dev/null") has already redirected fd 2 to /dev/null before
	// compleo starts, so /dev/stderr would resolve to that same /dev/null by inheritance.
	EnvDebugLog = "_COMPLEO_DEBUG_LOG"
)
