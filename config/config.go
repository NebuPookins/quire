package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the persisted application configuration.
type Config struct {
	LastSaveDir string `json:"last_save_dir"`
}

// configPath returns the path to the config file, respecting XDG_CONFIG_HOME.
func configPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "quire", "config.json")
}

// defaults returns a Config populated with safe default values.
func defaults() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return Config{LastSaveDir: filepath.Join(home, "Documents")}
}

// Load reads the config from $XDG_CONFIG_HOME/quire/config.json.
// On any error it returns a Config with safe defaults.
func Load() Config {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return defaults()
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaults()
	}
	if cfg.LastSaveDir == "" {
		cfg.LastSaveDir = defaults().LastSaveDir
	}
	return cfg
}

// Save writes cfg to $XDG_CONFIG_HOME/quire/config.json atomically.
func Save(cfg Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// Write to a temp file in the same directory then rename for atomicity.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".quire-config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
