package cmd

// AgentsCmd prints the bundled agent skill. It is an alias for `yat skill show`,
// kept so existing prompts that run `yat agents` keep working.
type AgentsCmd struct{}

// Run prints the skill, exactly as `yat skill show` does.
func (a *AgentsCmd) Run(env *SkillEnv) error {
	return (&SkillShowCmd{}).Run(env)
}
