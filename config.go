package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"ems-bridge/components"
	"ems-bridge/encr"
	"ems-bridge/routes"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Routes     []routes.RouteConfig    `yaml:"routes"`
	Components []components.Component  `yaml:"components"`
}

func LoadConfig(path string, db *sql.DB) (*Config, error) {
	slog.Info("reading config", "path", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	slog.Info("config loaded", "components", len(cfg.Components), "routes", len(cfg.Routes))

	if err := cfg.decrypt(db); err != nil {
		return nil, fmt.Errorf("decrypting config: %w", err)
	}

	return &cfg, nil
}

func (cfg *Config) decrypt(db *sql.DB) error {
	for name, component := range cfg.Components {
		for k, v := range component.Properties {
			dec, err := maybeDecrypt(db, v)
			if err != nil {
				return fmt.Errorf("component %q property %q: %w", name, k, err)
			}
			component.Properties[k] = dec
		}
		cfg.Components[name] = component
	}
	return nil
}

func maybeDecrypt(db *sql.DB, s string) (string, error) {
	if !strings.HasPrefix(s, "encr:") {
		return s, nil
	}
	return encr.Decrypt(db, s)
}
