// Package config handles loading the optional .yat.yaml project config file.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// StatusGroups defines the three semantic status groups used by the dependency graph.
// First entry in each group is the default for transitions.
type StatusGroups struct {
	Done    []string `yaml:"done"`
	Active  []string `yaml:"active"`
	Initial []string `yaml:"initial"`
}

// IsEmpty reports whether all status slices are nil or empty.
func (sg StatusGroups) IsEmpty() bool {
	return len(sg.Done) == 0 && len(sg.Active) == 0 && len(sg.Initial) == 0
}

// IsDone reports whether s belongs to the done group.
func (sg StatusGroups) IsDone(s string) bool { return containsStatus(sg.Done, s) }

// IsActive reports whether s belongs to the active group.
func (sg StatusGroups) IsActive(s string) bool { return containsStatus(sg.Active, s) }

// IsInitial reports whether s belongs to the initial group.
func (sg StatusGroups) IsInitial(s string) bool { return containsStatus(sg.Initial, s) }

func containsStatus(group []string, s string) bool {
	for _, v := range group {
		if v == s {
			return true
		}
	}
	return false
}

// DefaultDone returns the first done status (the default for completion transitions).
func (sg StatusGroups) DefaultDone() string { return sg.Done[0] }

// DefaultActive returns the first active status (the default for start transitions).
func (sg StatusGroups) DefaultActive() string { return sg.Active[0] }

// DefaultInitial returns the first initial status (the default for new items).
func (sg StatusGroups) DefaultInitial() string { return sg.Initial[0] }

// AllValid returns the union of all status groups.
func (sg StatusGroups) AllValid() []string {
	out := make([]string, 0, len(sg.Done)+len(sg.Active)+len(sg.Initial))
	out = append(out, sg.Done...)
	out = append(out, sg.Active...)
	out = append(out, sg.Initial...)
	return out
}

// Config represents the .yat.yaml project configuration.
type Config struct {
	Dir          string            `yaml:"dir"`
	ReadOnly     bool              `yaml:"readonly"`
	Statuses     StatusGroups      `yaml:"statuses"`
	Phases       []string          `yaml:"phases"`
	FieldAliases map[string]string `yaml:"field_aliases"`
}

// DefaultConfigFile is the primary config file name written by `yat init`.
const DefaultConfigFile = ".yat.yaml"

// ConfigNames lists config file variants in priority order. Local overrides
// are checked first so that per-machine settings (potentially gitignored)
// take precedence over shared project config.
var ConfigNames = []string{".yat.local.yaml", ".yat.local.yml", DefaultConfigFile, ".yat.yml"}

// Load searches for a yat config file starting from the current directory and
// walking up to the filesystem root. At each level it checks .yat.local.yaml,
// .yat.local.yml, .yat.yaml, and .yat.yml in that order. If no config file is
// found, it returns a zero Config and no error.
func Load() (Config, error) {
	return LoadWithFallback("")
}

// LoadWithFallback searches for config from the current directory first,
// then merges in config found near the items directory. This allows a
// project config (setting dir) and a spec-level config (setting statuses,
// phases, etc.) to coexist. The CWD config takes precedence for fields
// that both configs set. If fallbackDir is non-empty it is used as the
// secondary search location; otherwise the primary config's Dir field is
// used.
func LoadWithFallback(fallbackDir string) (Config, error) {
	dir, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}

	primary, _, err := loadWalk(dir)
	if err != nil {
		return Config{}, err
	}

	// Determine where to search for a secondary config.
	// CLI --dir flag takes precedence; otherwise use dir from primary config.
	secondaryDir := fallbackDir
	if secondaryDir == "" {
		secondaryDir = primary.Dir
	}

	if secondaryDir != "" {
		secondary, secondaryFound, err := loadWalk(secondaryDir)
		if err != nil {
			return Config{}, err
		}

		if secondaryFound {
			primary.MergeFrom(&secondary)
		}
	}

	primary.Defaults()

	if err := primary.Validate(); err != nil {
		return Config{}, err
	}

	return primary, nil
}

// loadWalk searches for a config file starting from startDir and walking up
// to the filesystem root.
func loadWalk(startDir string) (Config, bool, error) {
	dir := startDir

	for {
		cfg, found, err := tryLoadFromDir(dir)
		if err != nil {
			return Config{}, false, err
		}

		if found {
			return cfg, true, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return Config{}, false, nil
}

// tryLoadFromDir tries each config file name in the given directory.
// Returns the parsed config and true if found, or zero Config and false if not.
func tryLoadFromDir(dir string) (Config, bool, error) {
	for _, name := range ConfigNames {
		data, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil {
			if errors.Is(readErr, fs.ErrNotExist) {
				continue
			}

			return Config{}, false, readErr
		}

		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, false, err
		}

		expanded, expandErr := ExpandTilde(cfg.Dir)
		if expandErr != nil {
			return Config{}, false, expandErr
		}

		cfg.Dir = expanded

		return cfg, true, nil
	}

	return Config{}, false, nil
}

// canonicalFields is the set of field names that can appear as alias keys.
var canonicalFields = map[string]bool{
	"id": true, "title": true, "type": true, "priority": true,
	"points": true, "dependencies": true, "status": true, "phase": true,
}

// Defaults populates StatusGroups with the built-in defaults when the config
// omits the statuses key entirely.
func (c *Config) Defaults() {
	if c.Statuses.IsEmpty() {
		c.Statuses = StatusGroups{
			Done:    []string{"done", "complete", "completed", "closed"},
			Active:  []string{"in-progress", "active", "started"},
			Initial: []string{"todo", "draft", "backlog", "new"},
		}
	}
}

// Validate checks that the config is self-consistent.
func (c *Config) Validate() error {
	// Check no status appears in multiple groups.
	seen := make(map[string]string)
	for _, pair := range []struct {
		group string
		vals  []string
	}{
		{"done", c.Statuses.Done},
		{"active", c.Statuses.Active},
		{"initial", c.Statuses.Initial},
	} {
		for _, s := range pair.vals {
			if prev, ok := seen[s]; ok {
				return fmt.Errorf("status %q appears in both %s and %s groups", s, prev, pair.group)
			}
			seen[s] = pair.group
		}
	}

	// Check alias keys are known canonical fields.
	aliasTargets := make(map[string]string) // alias value → canonical key
	for canonical, alias := range c.FieldAliases {
		if !canonicalFields[canonical] {
			return fmt.Errorf("field_aliases key %q is not a known field", canonical)
		}
		if prev, ok := aliasTargets[alias]; ok {
			return fmt.Errorf("alias %q is used for both %q and %q", alias, prev, canonical)
		}
		aliasTargets[alias] = canonical
	}

	return nil
}

// MergeFrom fills zero-valued fields in c from other.
// ReadOnly uses OR semantics: true if either config sets it.
// FieldAliases are merged per-key with c taking precedence.
func (c *Config) MergeFrom(other *Config) {
	if c.Dir == "" {
		c.Dir = other.Dir
	}

	c.ReadOnly = c.ReadOnly || other.ReadOnly

	if c.Statuses.IsEmpty() {
		c.Statuses = other.Statuses
	}

	if c.Phases == nil {
		c.Phases = other.Phases
	}

	if len(other.FieldAliases) > 0 {
		if c.FieldAliases == nil {
			c.FieldAliases = make(map[string]string)
		}

		for k, v := range other.FieldAliases {
			if _, exists := c.FieldAliases[k]; !exists {
				c.FieldAliases[k] = v
			}
		}
	}
}

func ExpandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expanding ~: %w", err)
	}

	return filepath.Join(home, path[1:]), nil
}
