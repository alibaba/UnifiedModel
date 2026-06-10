package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	Addr string `yaml:"addr,omitempty"`
}

type Config struct {
	Current      string             `yaml:"current,omitempty"`
	OutputFormat string             `yaml:"output_format,omitempty"`
	Profiles     map[string]Profile `yaml:"profiles,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Current:      "default",
		OutputFormat: "json",
		Profiles: map[string]Profile{
			"default": {Addr: "http://localhost:8080"},
		},
	}
}

func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".umctl")
	}
	return filepath.Join(home, ".umctl")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func LoadConfig() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}
	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	path := ConfigPath()
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (c *Config) CurrentProfile() Profile {
	name := c.Current
	if name == "" {
		name = "default"
	}
	if p, ok := c.Profiles[name]; ok {
		return p
	}
	return Profile{Addr: "http://localhost:8080"}
}

func (c *Config) ResolveAddr(flagAddr, envAddr string) string {
	if flagAddr != "" {
		return flagAddr
	}
	if envAddr != "" {
		return envAddr
	}
	return c.CurrentProfile().Addr
}
