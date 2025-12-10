package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the Snowflake connection and app settings.
type Config struct {
	Account        string `mapstructure:"account"`
	User           string `mapstructure:"user"`
	Password       string `mapstructure:"password"`
	PrivateKeyPath string `mapstructure:"private_key_path"`
	Database       string `mapstructure:"database"`
	Schema         string `mapstructure:"schema"`
	Warehouse      string `mapstructure:"warehouse"`
	Context        string `mapstructure:"context"`
	Debug          bool   `mapstructure:"debug"`
}

// LoadConfig reads configuration from env vars and the optional config file.
// Context names align with the kubeconfig style: contexts.<name>.
func LoadConfig(contextName string) (Config, error) {
	cfgPath := configFilePath()

	if err := ensureConfigDir(cfgPath); err != nil {
		return Config{}, err
	}
	loadEnvOverrides(cfgPath)

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("SNOWFLAKE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetDefault("schema", "PUBLIC")
	v.SetDefault("debug", false)
	bindEnvKeys(v)

	v.SetConfigFile(cfgPath)
	if _, err := os.Stat(cfgPath); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	// If context provided, drill down to that section while keeping env overrides.
	if contextName == "" {
		contextName = v.GetString("context")
	}
	if contextName != "" {
		sub := v.Sub(fmt.Sprintf("contexts.%s", contextName))
		if sub == nil {
			return Config{}, fmt.Errorf("context %q not found in config", contextName)
		}
		sub.SetEnvPrefix("SNOWFLAKE")
		sub.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		sub.AutomaticEnv()
		bindEnvKeys(sub)
		return decodeConfig(sub)
	}

	return decodeConfig(v)
}

// MergeOverrides applies non-empty values from overrides to the base config.
func MergeOverrides(base, overrides Config) Config {
	result := base
	if overrides.Account != "" {
		result.Account = overrides.Account
	}
	if overrides.User != "" {
		result.User = overrides.User
	}
	if overrides.Password != "" {
		result.Password = overrides.Password
	}
	if overrides.PrivateKeyPath != "" {
		result.PrivateKeyPath = overrides.PrivateKeyPath
	}
	if overrides.Database != "" {
		result.Database = overrides.Database
	}
	if overrides.Schema != "" {
		result.Schema = overrides.Schema
	}
	if overrides.Warehouse != "" {
		result.Warehouse = overrides.Warehouse
	}
	if overrides.Context != "" {
		result.Context = overrides.Context
	}
	if overrides.Debug {
		result.Debug = true
	}
	return result
}

// Validate ensures mandatory fields are present.
func (c Config) Validate() error {
	if c.Account == "" {
		return errors.New("account is required")
	}
	if c.User == "" {
		return errors.New("user is required")
	}
	if c.Password == "" && c.PrivateKeyPath == "" {
		return errors.New("password or private key path is required")
	}
	if c.PrivateKeyPath != "" {
		if _, err := os.Stat(c.PrivateKeyPath); err != nil {
			return fmt.Errorf("private key path: %w", err)
		}
	}
	return nil
}

func decodeConfig(v *viper.Viper) (Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	return cfg, nil
}

func configFilePath() string {
	if custom := os.Getenv("SNOW9S_CONFIG"); custom != "" {
		return custom
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".snow9s", "config.yaml")
}

func bindEnvKeys(v *viper.Viper) {
	for _, key := range []string{"account", "user", "password", "private_key_path", "database", "schema", "warehouse", "context", "debug"} {
		_ = v.BindEnv(key)
	}
}

func ensureConfigDir(cfgPath string) error {
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	createEnvTemplate(dir)
	return nil
}

// loadEnvOverrides reads optional env file (~/.snow9s/env) and sets process env vars
// if they are not already set. This allows users to colocate SNOWFLAKE_* secrets
// alongside the config file without exporting them globally.
func loadEnvOverrides(cfgPath string) {
	dir := filepath.Dir(cfgPath)
	envPath := filepath.Join(dir, "env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" || val == "" {
			continue
		}
		if existing, exists := os.LookupEnv(key); exists && existing != "" {
			continue
		}
		_ = os.Setenv(key, val)
	}
}

func createEnvTemplate(dir string) {
	envPath := filepath.Join(dir, "env")
	if _, err := os.Stat(envPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		return
	}
	template := `# snow9s Snowflake credentials (leave unset to fall back to config.yaml/flags)
# SNOWFLAKE_ACCOUNT=abc123
# SNOWFLAKE_USER=myuser
# SNOWFLAKE_PASSWORD=mypassword
# SNOWFLAKE_PRIVATE_KEY_PATH=/path/to/p8
# SNOWFLAKE_DATABASE=MYDB
# SNOWFLAKE_SCHEMA=PUBLIC
# SNOWFLAKE_WAREHOUSE=COMPUTE_WH
`
	_ = os.WriteFile(envPath, []byte(template), 0o600)
}
