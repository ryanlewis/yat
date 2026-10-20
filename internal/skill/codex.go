package skill

type codexAgent struct{}

const codexName = "codex"

func (codexAgent) Name() string { return codexName }

// DefaultDir resolves the skill directory inside the Codex CLI's config
// directory. Codex relocates that directory via $CODEX_HOME, so honor it
// when set and fall back to ~/.codex.
func (codexAgent) DefaultDir() (string, error) {
	return resolveAgentDir("CODEX_HOME", verbatimTilde, ".codex")
}

func (codexAgent) Files() map[string][]byte { return sharedFiles }
