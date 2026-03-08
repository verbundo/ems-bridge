package main

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"ems-bridge/components"
	"ems-bridge/routes"
)

// Application holds all instantiated components and routes.
type Application struct {
	Components []components.Runner
	Routes     []*routes.Route
	mux        *http.ServeMux
	addr       string
	tlsCert    string
	tlsKey     string
}

// NewApplication creates an Application from cfg, instantiating all
// components and routes.
func NewApplication(cfg *Config) (*Application, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	app := &Application{
		mux:     mux,
		addr:    fmt.Sprintf("%s:%d", cfg.IP, cfg.Port),
		tlsCert: cfg.TLSCert,
		tlsKey:  cfg.TLSKey,
	}

	for _, appCfg := range cfg.Apps {
		for _, cc := range appCfg.Components {
			slog.Info("creating component", "name", cc.Name, "type", cc.Type, "app", appCfg.Name)
			c, err := components.New(cc)
			if err != nil {
				return nil, fmt.Errorf("app %q: creating component %q: %w", appCfg.Name, cc.Name, err)
			}
			app.Components = append(app.Components, c)
		}

		for _, rc := range appCfg.Routes {
			slog.Info("creating route", "name", rc.Name, "app", appCfg.Name)
			r, err := routes.New(rc, mux)
			if err != nil {
				return nil, fmt.Errorf("app %q: creating route %q: %w", appCfg.Name, rc.Name, err)
			}
			app.Routes = append(app.Routes, r)
		}
	}

	return app, nil
}

// Start starts all components and initializes all routes.
func (app *Application) Start() error {
	slog.Info("starting components")
	for _, c := range app.Components {
		if err := c.Start(); err != nil {
			return fmt.Errorf("starting component: %w", err)
		}
	}

	slog.Info("initializing routes")
	for _, r := range app.Routes {
		if err := r.Init(); err != nil {
			return fmt.Errorf("initializing route %q: %w", r.Name, err)
		}
	}
	return nil
}

// Serve starts the HTTP/2 server and blocks until it stops.
// With TLS: "h2" is placed in NextProtos before the TLS listener is created so
// ALPN negotiates HTTP/2. Without TLS: h2c wraps the mux.
func (app *Application) Serve() error {
	if app.tlsCert != "" && app.tlsKey != "" {
		server := &http.Server{
			Addr:    app.addr,
			Handler: app.mux,
			TLSConfig: &tls.Config{
				NextProtos: []string{"h2", "http/1.1"},
			},
		}
		slog.Info("starting HTTPS/2 server", "addr", app.addr, "cert", app.tlsCert)
		return server.ListenAndServeTLS(app.tlsCert, app.tlsKey)
	}

	server := &http.Server{
		Addr:    app.addr,
		Handler: h2c.NewHandler(app.mux, &http2.Server{}),
	}
	slog.Info("starting HTTP/2 cleartext server", "addr", app.addr)
	return server.ListenAndServe()
}

func start(cfg *Config) error {
	app, err := NewApplication(cfg)
	if err != nil {
		return err
	}
	if err := app.Start(); err != nil {
		return err
	}
	return app.Serve()
}
