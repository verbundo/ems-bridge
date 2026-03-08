package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"ems-bridge/components"
	"ems-bridge/encr"
	"ems-bridge/routes"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration loaded from the main config file.
type Config struct {
	IP       string       `yaml:"ip"`
	Port     int          `yaml:"port"`
	LogLevel string       `yaml:"logLevel"`
	TLSCert  string       `yaml:"tlsCert"`
	TLSKey   string       `yaml:"tlsKey"`
	Apps     []*AppConfig `yaml:"apps"`
}

// AppConfig holds the app file reference and the loaded routes and components.
type AppConfig struct {
	Name       string                 `yaml:"name"`
	File       string                 `yaml:"file"`
	Routes     []routes.RouteConfig
	Components []components.Component
}

// appFile is the structure of each app's yaml definition file.
type appFile struct {
	Routes     []routes.RouteConfig   `yaml:"routes"`
	Components []components.Component `yaml:"components"`
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

	baseDir := filepath.Dir(path)
	for _, app := range cfg.Apps {
		slog.Info("loading app", "name", app.Name, "file", app.File)
		if err := loadApp(app, baseDir, db); err != nil {
			return nil, fmt.Errorf("app %q: %w", app.Name, err)
		}
		slog.Info("app loaded", "name", app.Name, "components", len(app.Components), "routes", len(app.Routes))
	}

	return &cfg, nil
}

func loadApp(app *AppConfig, baseDir string, db *sql.DB) error {
	filePath := app.File
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(baseDir, filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading app file %q: %w", filePath, err)
	}

	var af appFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return fmt.Errorf("parsing app file %q: %w", filePath, err)
	}

	app.Routes = af.Routes
	app.Components = af.Components

	return decryptApp(app, db)
}

func decryptApp(app *AppConfig, db *sql.DB) error {
	for i, component := range app.Components {
		for k, v := range component.Properties {
			dec, err := maybeDecrypt(db, v)
			if err != nil {
				return fmt.Errorf("component %q property %q: %w", component.Name, k, err)
			}
			component.Properties[k] = dec
		}
		app.Components[i] = component
	}
	return nil
}

func maybeDecrypt(db *sql.DB, s string) (string, error) {
	if !strings.HasPrefix(s, "encr:") {
		return s, nil
	}
	return encr.Decrypt(db, s)
}
