package cmdutil

// Env var names compleo and wrapped scripts read/set. Centralized so the Go code setting one
// (e.g. generate's subprocess env) and the code reading it (or, for EnvSpecFile, the zsh
// compleo itself emits) can't drift apart.
const (
	// EnvCompletionDir overrides CompletionDir's default location.
	EnvCompletionDir = "COMPLEO_COMPLETION_DIR"
	// EnvMode selects compleo parse's behavior beyond parsing "$@" (e.g. "generate").
	EnvMode = "COMPLEO_MODE"
	// EnvGenerateName is the real, invokable name "generate <script>" runs its target under.
	EnvGenerateName = "COMPLEO_GENERATE_NAME"
	// EnvSpecFile points a live Tab-press at the companion spec file WrappedCompletion embeds.
	EnvSpecFile = "COMPLEO_SPEC_FILE"
)
