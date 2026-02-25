package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"ems-bridge/encr"
	"gopkg.in/yaml.v3"
)

type ConnectorConfig struct {
	Type     string `yaml:"type"`
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type RouteConfig struct {
	Name string   `yaml:"name"`
	From string   `yaml:"from"`
	To   []string `yaml:"to"`
}

type Config struct {
	Routes     []RouteConfig              `yaml:"routes"`
	Connectors map[string]ConnectorConfig `yaml:",inline"`
}

func LoadConfig(path string, db *sql.DB) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.decrypt(db); err != nil {
		return nil, fmt.Errorf("decrypting config: %w", err)
	}

	return &cfg, nil
}

func (cfg *Config) decrypt(db *sql.DB) error {
	for name, conn := range cfg.Connectors {
		dec, err := maybeDecrypt(db, conn.Password)
		if err != nil {
			return fmt.Errorf("connector %q password: %w", name, err)
		}
		conn.Password = dec
		cfg.Connectors[name] = conn
	}
	return nil
}

func maybeDecrypt(db *sql.DB, s string) (string, error) {
	if !strings.HasPrefix(s, "encr:") {
		return s, nil
	}
	return encr.Decrypt(db, s)
}
