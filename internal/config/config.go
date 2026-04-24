package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server  ServerConfig  `toml:"server" json:"server"`
	Files   FilesConfig   `toml:"files" json:"files"`
	Logging LoggingConfig `toml:"logging" json:"logging"`
	Refresh RefreshConfig `toml:"refresh" json:"refresh"`
	Kiro    KiroConfig    `toml:"kiro" json:"kiro"`
}

type ServerConfig struct {
	Host        string `toml:"host" json:"host"`
	Port        int    `toml:"port" json:"port"`
	AdminAPIKey string `toml:"admin_api_key" json:"-"`
	PublicURL   string `toml:"public_url" json:"public_url"`
}

type FilesConfig struct {
	ConfigDir      string `toml:"config_dir" json:"config_dir"`
	PoolsPath      string `toml:"pools_path" json:"pools_path"`
	PromptsPath    string `toml:"prompts_path" json:"prompts_path"`
	CredentialsDir string `toml:"credentials_dir" json:"credentials_dir"`
}

type LoggingConfig struct {
	Dir   string `toml:"dir" json:"dir"`
	Level string `toml:"level" json:"level"`
}

type RefreshConfig struct {
	NearMinutes int `toml:"near_minutes" json:"near_minutes"`
	MaxRetries  int `toml:"max_retries" json:"max_retries"`
	BaseDelayMS int `toml:"base_delay_ms" json:"base_delay_ms"`
}

type KiroConfig struct {
	DefaultRegion      string `toml:"default_region" json:"default_region"`
	DefaultIDCRegion   string `toml:"default_idc_region" json:"default_idc_region"`
	DefaultModel       string `toml:"default_model" json:"default_model"`
	AssistantIdentity  string `toml:"assistant_identity" json:"assistant_identity"`
	AuthServiceURL     string `toml:"auth_service_url" json:"auth_service_url"`
	OidcURLTemplate    string `toml:"oidc_url_template" json:"oidc_url_template"`
	RefreshURLTemplate string `toml:"refresh_url_template" json:"refresh_url_template"`
	BaseURLTemplate    string `toml:"base_url_template" json:"base_url_template"`
	KiroVersion        string `toml:"kiro_version" json:"kiro_version"`
	CallbackPath       string `toml:"callback_path" json:"callback_path"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Host:        "0.0.0.0",
			Port:        13120,
			AdminAPIKey: "change-me",
			PublicURL:   "http://127.0.0.1:13120",
		},
		Files: FilesConfig{
			ConfigDir:      "config",
			PoolsPath:      "config/pools.toml",
			PromptsPath:    "config/prompts.toml",
			CredentialsDir: "config/credentials/kiro",
		},
		Logging: LoggingConfig{
			Dir:   "config/logs",
			Level: "info",
		},
		Refresh: RefreshConfig{
			NearMinutes: 30,
			MaxRetries:  3,
			BaseDelayMS: 1000,
		},
		Kiro: KiroConfig{
			DefaultRegion:      "us-east-1",
			DefaultIDCRegion:   "us-east-1",
			DefaultModel:       "claude-sonnet-4-5",
			AssistantIdentity:  "AI 编程助手",
			AuthServiceURL:     "https://prod.us-east-1.auth.desktop.kiro.dev",
			OidcURLTemplate:    "https://oidc.{{region}}.amazonaws.com",
			RefreshURLTemplate: "https://prod.{{region}}.auth.desktop.kiro.dev/refreshToken",
			BaseURLTemplate:    "https://q.{{region}}.amazonaws.com/generateAssistantResponse",
			KiroVersion:        "0.11.63",
			CallbackPath:       "/oauth/kiro/callback",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = filepath.Join(cfg.Files.ConfigDir, "config.toml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := cfg.Ensure(path); err != nil {
				return cfg, err
			}
			return cfg, nil
		}
		return cfg, err
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.normalize()
	if err := cfg.Ensure(path); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) normalize() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 13120
	}
	if c.Files.ConfigDir == "" {
		c.Files.ConfigDir = "config"
	}
	if c.Files.PoolsPath == "" {
		c.Files.PoolsPath = filepath.Join(c.Files.ConfigDir, "pools.toml")
	}
	if c.Files.PromptsPath == "" {
		c.Files.PromptsPath = filepath.Join(c.Files.ConfigDir, "prompts.toml")
	}
	if c.Files.CredentialsDir == "" {
		c.Files.CredentialsDir = filepath.Join(c.Files.ConfigDir, "credentials", "kiro")
	}
	if c.Logging.Dir == "" {
		c.Logging.Dir = filepath.Join(c.Files.ConfigDir, "logs")
	}
	if c.Kiro.DefaultRegion == "" {
		c.Kiro.DefaultRegion = "us-east-1"
	}
	if c.Kiro.DefaultIDCRegion == "" {
		c.Kiro.DefaultIDCRegion = c.Kiro.DefaultRegion
	}
	if c.Kiro.DefaultModel == "" {
		c.Kiro.DefaultModel = "claude-sonnet-4-5"
	}
	if c.Kiro.AssistantIdentity == "" {
		c.Kiro.AssistantIdentity = "AI 编程助手"
	}
	if c.Kiro.AuthServiceURL == "" {
		c.Kiro.AuthServiceURL = "https://prod.us-east-1.auth.desktop.kiro.dev"
	}
	if c.Kiro.OidcURLTemplate == "" {
		c.Kiro.OidcURLTemplate = "https://oidc.{{region}}.amazonaws.com"
	}
	if c.Kiro.RefreshURLTemplate == "" {
		c.Kiro.RefreshURLTemplate = "https://prod.{{region}}.auth.desktop.kiro.dev/refreshToken"
	}
	if c.Kiro.BaseURLTemplate == "" {
		c.Kiro.BaseURLTemplate = "https://q.{{region}}.amazonaws.com/generateAssistantResponse"
	}
	if c.Kiro.KiroVersion == "" {
		c.Kiro.KiroVersion = "0.11.63"
	}
	if c.Kiro.CallbackPath == "" {
		c.Kiro.CallbackPath = "/oauth/kiro/callback"
	}
	if c.Refresh.NearMinutes == 0 {
		c.Refresh.NearMinutes = 30
	}
	if c.Refresh.MaxRetries == 0 {
		c.Refresh.MaxRetries = 3
	}
	if c.Refresh.BaseDelayMS == 0 {
		c.Refresh.BaseDelayMS = 1000
	}
}

func (c Config) Ensure(configPath string) error {
	if configPath == "" {
		configPath = filepath.Join(c.Files.ConfigDir, "config.toml")
	}
	for _, dir := range []string{
		filepath.Dir(configPath),
		c.Files.ConfigDir,
		filepath.Dir(c.Files.PoolsPath),
		filepath.Dir(c.Files.PromptsPath),
		c.Files.CredentialsDir,
		c.Logging.Dir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		return AtomicWriteTOML(configPath, c)
	}
	return nil
}

func AtomicWriteTOML(path string, value any) error {
	data, err := toml.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp-%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if _, err := toml.Marshal(value); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
