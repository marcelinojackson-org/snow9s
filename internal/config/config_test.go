package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("SNOWFLAKE_ACCOUNT", "envacct")
	t.Setenv("SNOWFLAKE_USER", "envuser")
	t.Setenv("SNOWFLAKE_PASSWORD", "envpass")
	t.Setenv("SNOWFLAKE_PRIVATE_KEY_PATH", "")
	t.Setenv("SNOWFLAKE_SCHEMA", "PUBLIC")
	t.Setenv("SNOWFLAKE_DATABASE", "DB")
	t.Setenv("SNOWFLAKE_WAREHOUSE", "WH")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNOW9S_CONFIG", cfgPath)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Account != "envacct" || cfg.User != "envuser" || cfg.Password != "envpass" {
		t.Fatalf("env vars not mapped: %+v", cfg)
	}
}

func TestLoadConfigWithContextFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
contexts:
  dev:
    account: acct1
    user: user1
    password: pass1
    schema: PUBLIC
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNOW9S_CONFIG", path)
	t.Setenv("SNOWFLAKE_ACCOUNT", "acct1")
	t.Setenv("SNOWFLAKE_USER", "user1")
	t.Setenv("SNOWFLAKE_PASSWORD", "pass1")
	t.Setenv("SNOWFLAKE_PRIVATE_KEY_PATH", "")
	t.Setenv("SNOWFLAKE_SCHEMA", "PUBLIC")

	cfg, err := LoadConfig("dev")
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if cfg.Account != "acct1" || cfg.User != "user1" || cfg.Password != "pass1" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env")
	if err := os.WriteFile(envFile, []byte("SNOWFLAKE_ACCOUNT=envfile\nSNOWFLAKE_USER=envuser\nSNOWFLAKE_PASSWORD=envpass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	t.Setenv("SNOW9S_CONFIG", cfgPath)
	t.Setenv("SNOWFLAKE_ACCOUNT", "")
	t.Setenv("SNOWFLAKE_USER", "")
	t.Setenv("SNOWFLAKE_PASSWORD", "")
	t.Setenv("SNOWFLAKE_PRIVATE_KEY_PATH", "")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Account != "envfile" || cfg.User != "envuser" || cfg.Password != "envpass" {
		t.Fatalf("env overrides not loaded: %+v", cfg)
	}
}

func TestEnvTemplateCreated(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	t.Setenv("SNOW9S_CONFIG", cfgPath)
	t.Setenv("SNOWFLAKE_ACCOUNT", "")
	t.Setenv("SNOWFLAKE_USER", "")
	t.Setenv("SNOWFLAKE_PASSWORD", "")
	t.Setenv("SNOWFLAKE_PRIVATE_KEY_PATH", "")

	if _, err := LoadConfig(""); err != nil {
		t.Fatalf("load config: %v", err)
	}
	envPath := filepath.Join(filepath.Dir(cfgPath), "env")
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf("env template not created: %v", err)
	}
}

func TestValidateWithPrivateKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.p8")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Account: "acct", User: "user", PrivateKeyPath: keyPath}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validation success with private key: %v", err)
	}
}

func TestMergeOverrides(t *testing.T) {
	base := Config{Account: "a", User: "u", Password: "p", Schema: "public"}
	over := Config{Account: "x", Debug: true}
	merged := MergeOverrides(base, over)
	if merged.Account != "x" || !merged.Debug {
		t.Fatalf("merge failed: %+v", merged)
	}
}
