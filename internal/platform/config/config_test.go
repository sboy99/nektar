package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSecretsFromEnvFile(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
eventbus:
  provider: inmemory
storage:
  provider: inmemory
cache:
  provider: inmemory
gmail:
  default_query: "newer_than:1d"
gemini:
  model: gemini-2.0-flash
`), 0o600); err != nil {
		t.Fatal(err)
	}

	envBody := "" +
		"NEKTAR_GMAIL_CLIENT_ID=id-from-env\n" +
		"NEKTAR_GMAIL_CLIENT_SECRET=secret-from-env\n" +
		"NEKTAR_GEMINI_API_KEY=gemini-key\n" +
		"NEKTAR_DISCORD_WEBHOOK_URL=https://discord.example/hook\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envBody), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, k := range []string{
		"NEKTAR_GMAIL_CLIENT_ID",
		"NEKTAR_GMAIL_CLIENT_SECRET",
		"NEKTAR_GEMINI_API_KEY",
		"NEKTAR_DISCORD_WEBHOOK_URL",
		"NEKTAR_ENV_FILE",
	} {
		t.Cleanup(func(key string) func() {
			v, ok := os.LookupEnv(key)
			return func() {
				if ok {
					_ = os.Setenv(key, v)
				} else {
					_ = os.Unsetenv(key)
				}
			}
		}(k))
		_ = os.Unsetenv(k)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Gmail.ClientID != "id-from-env" {
		t.Fatalf("client_id = %q", cfg.Gmail.ClientID)
	}
	if cfg.Gmail.ClientSecret != "secret-from-env" {
		t.Fatalf("client_secret = %q", cfg.Gmail.ClientSecret)
	}
	if cfg.Gemini.APIKey != "gemini-key" {
		t.Fatalf("api_key = %q", cfg.Gemini.APIKey)
	}
	if cfg.Discord.WebhookURL != "https://discord.example/hook" {
		t.Fatalf("webhook = %q", cfg.Discord.WebhookURL)
	}
}

func TestLoadOSEnvWinsOverDotEnv(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("eventbus:\n  provider: inmemory\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("NEKTAR_GEMINI_API_KEY=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("NEKTAR_GEMINI_API_KEY", "from-os")
	t.Setenv("NEKTAR_ENV_FILE", "")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Gemini.APIKey != "from-os" {
		t.Fatalf("api_key = %q, want from-os", cfg.Gemini.APIKey)
	}
}
