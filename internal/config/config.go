// Package config handles loading the optional .yat.yaml project config file.
package config

import (
	"errors"
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

// Load reads .yat.yaml from the current directory. If the file does not exist,
// it returns a zero Config and no error.
func Load() (Config, error) {
	data, err := os.ReadFile(".yat.yaml")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Config{}, nil
		}

		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	cfg.Dir = expandTilde(cfg.Dir)

	return cfg, nil
}

func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(home, path[1:])
}
