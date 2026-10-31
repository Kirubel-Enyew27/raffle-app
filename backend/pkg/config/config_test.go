package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// chdir moves into dir for the duration of the test and resets viper's global
// state afterwards (Load mutates the package-level viper instance).
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prev)
		viper.Reset()
	})
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadReadsDotEnv(t *testing.T) {
	viper.Reset()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "GEMINI_API_KEY=from-dotenv\nTELE_VERIFY_KEY=verify-from-dotenv\n")
	chdir(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GeminiAPIKey != "from-dotenv" {
		t.Errorf("GeminiAPIKey = %q, want %q", cfg.GeminiAPIKey, "from-dotenv")
	}
	if cfg.TeleVerifyKey != "verify-from-dotenv" {
		t.Errorf("TeleVerifyKey = %q, want %q", cfg.TeleVerifyKey, "verify-from-dotenv")
	}
}

func TestLoadPrefersEnvironmentOverDotEnv(t *testing.T) {
	viper.Reset()
	t.Setenv("GEMINI_API_KEY", "from-environment")

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "GEMINI_API_KEY=from-dotenv\n")
	chdir(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GeminiAPIKey != "from-environment" {
		t.Errorf("GeminiAPIKey = %q, want the environment value %q", cfg.GeminiAPIKey, "from-environment")
	}
}

func TestLoadFindsBackendDotEnvFromRepoRoot(t *testing.T) {
	viper.Reset()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "backend", ".env"), "GEMINI_API_KEY=from-repo-root\n")
	chdir(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GeminiAPIKey != "from-repo-root" {
		t.Errorf("GeminiAPIKey = %q, want %q", cfg.GeminiAPIKey, "from-repo-root")
	}
}

func TestLoadWithoutDotEnvUsesDefaults(t *testing.T) {
	viper.Reset()
	t.Setenv("GEMINI_API_KEY", "")
	chdir(t, t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GeminiAPIKey != "" {
		t.Errorf("GeminiAPIKey = %q, want empty when no .env and no environment value", cfg.GeminiAPIKey)
	}
	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q, want the development default", cfg.AppEnv)
	}
	if cfg.Database.MaxOpenConns != 25 || cfg.Database.MaxIdleConns != 5 {
		t.Errorf("database pool defaults = (%d open, %d idle), want (25 open, 5 idle)", cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("database connection lifetime = %s, want 5m", cfg.Database.ConnMaxLifetime)
	}
}

func TestLoadDatabasePoolSettingsFromEnvironment(t *testing.T) {
	viper.Reset()
	t.Setenv("DB_MAX_OPEN_CONNS", "17")
	t.Setenv("DB_MAX_IDLE_CONNS", "3")
	t.Setenv("DB_CONN_MAX_LIFETIME", "2m")
	chdir(t, t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Database.MaxOpenConns != 17 || cfg.Database.MaxIdleConns != 3 {
		t.Errorf("database pool = (%d open, %d idle), want (17 open, 3 idle)", cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime != 2*time.Minute {
		t.Errorf("database connection lifetime = %s, want 2m", cfg.Database.ConnMaxLifetime)
	}
}

func TestDevelopmentShortcutRequiresExplicitEnvironment(t *testing.T) {
	previousValue, previouslySet := os.LookupEnv("APP_ENV")
	if err := os.Unsetenv("APP_ENV"); err != nil {
		t.Fatalf("unset APP_ENV: %v", err)
	}
	t.Cleanup(func() {
		if previouslySet {
			_ = os.Setenv("APP_ENV", previousValue)
		} else {
			_ = os.Unsetenv("APP_ENV")
		}
		viper.Reset()
	})

	viper.Reset()
	viper.SetDefault("APP_ENV", "development")
	if IsDevelopmentEnvironment() {
		t.Fatal("development shortcut enabled from the default APP_ENV without an explicit environment setting")
	}
}

func TestLoadRejectsMalformedDotEnv(t *testing.T) {
	viper.Reset()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "this line has no assignment\n")
	chdir(t, dir)

	if _, err := Load(); err == nil {
		t.Fatal("Load succeeded on a malformed .env, want an error")
	}
}
