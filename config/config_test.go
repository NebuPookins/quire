package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "quire", "config.json")
}

func TestLoad_Defaults_WhenNoFile(t *testing.T) {
	withTempConfig(t)
	cfg := Load()
	if cfg.LastSaveDir == "" {
		t.Fatal("expected a non-empty default LastSaveDir")
	}
}

func TestLoad_Defaults_WhenMalformed(t *testing.T) {
	path := withTempConfig(t)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("not json{{{{"), 0o644)

	cfg := Load()
	if cfg.LastSaveDir == "" {
		t.Fatal("expected defaults on malformed JSON")
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	withTempConfig(t)
	want := Config{LastSaveDir: "/tmp/my-scans"}

	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got := Load()
	if got.LastSaveDir != want.LastSaveDir {
		t.Fatalf("got %q, want %q", got.LastSaveDir, want.LastSaveDir)
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	path := withTempConfig(t)

	if err := Save(Config{LastSaveDir: "/tmp/x"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}

func TestLoad_EmptyLastSaveDir_UsesDefault(t *testing.T) {
	path := withTempConfig(t)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"last_save_dir":""}`), 0o644)

	cfg := Load()
	if cfg.LastSaveDir == "" {
		t.Fatal("expected default to fill in empty LastSaveDir")
	}
}
