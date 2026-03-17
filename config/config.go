package config

// Config holds the persisted application configuration.
type Config struct {
	LastSaveDir string `json:"last_save_dir"`
}

// Load reads the config from $XDG_CONFIG_HOME/quire/config.json.
// On any error it returns a Config with safe defaults.
func Load() Config {
	panic("not implemented")
}

// Save writes cfg to $XDG_CONFIG_HOME/quire/config.json.
func Save(cfg Config) error {
	panic("not implemented")
}
