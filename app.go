package main

import (
	"fmt"
	"log/slog"

	"ems-bridge/components"
	"ems-bridge/routes"
)

// Application holds all instantiated components and routes.
type Application struct {
	Components []components.Runner
	Routes     []*routes.Route
}

// NewApplication creates an Application from cfg, instantiating all
// components and routes.
func NewApplication(cfg *Config) (*Application, error) {
	app := &Application{}

	for _, cc := range cfg.Components {
		slog.Info("creating component", "name", cc.Name, "type", cc.Type)
		c, err := components.New(cc)
		if err != nil {
			return nil, fmt.Errorf("creating component %q: %w", cc.Name, err)
		}
		app.Components = append(app.Components, c)
	}

	for _, rc := range cfg.Routes {
		slog.Info("creating route", "name", rc.Name)
		r, err := routes.New(rc)
		if err != nil {
			return nil, fmt.Errorf("creating route %q: %w", rc.Name, err)
		}
		app.Routes = append(app.Routes, r)
	}

	return app, nil
}

// Start starts all components in the application.
func (app *Application) Start() error {
	slog.Info("starting components")
	for _, c := range app.Components {
		if err := c.Start(); err != nil {
			return fmt.Errorf("starting component: %w", err)
		}
	}
	return nil
}

func start(cfg *Config) error {
	app, err := NewApplication(cfg)
	if err != nil {
		return err
	}
	return app.Start()
}
