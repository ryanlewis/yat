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

// Config represents the .yat.yaml project configuration.
type Config struct {
	Dir string `yaml:"dir"`
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
	dir, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}

	for {
		for _, name := range ConfigNames {
			data, readErr := os.ReadFile(filepath.Join(dir, name))
			if readErr != nil {
				if errors.Is(readErr, fs.ErrNotExist) {
					continue
				}

				return Config{}, readErr
			}

			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return Config{}, err
			}

			expanded, expandErr := expandTilde(cfg.Dir)
			if expandErr != nil {
				return Config{}, expandErr
			}

			cfg.Dir = expanded

			return cfg, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return Config{}, nil
}

func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expanding ~ in config dir: %w", err)
	}

	return filepath.Join(home, path[1:]), nil
}
