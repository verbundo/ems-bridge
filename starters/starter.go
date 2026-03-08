package starters

import (
	"fmt"
	"net/http"

	"ems-bridge/messages"
)

// StarterConfig holds the common attributes shared by all starter types.
// Mux is not parsed from YAML; it is injected by the Application before
// routes are constructed so that HTTP-based starters can register handlers.
type StarterConfig struct {
	ID         string            `yaml:"id"`
	Type       string            `yaml:"type"`
	Properties map[string]string `yaml:"properties"`
	Mux        *http.ServeMux    `yaml:"-"`
}

// Handler is a function called by a starter for each message it produces.
type Handler func(*messages.Message) error

// Runner is the interface every concrete starter must implement.
type Runner interface {
	Start() error
	Stop() error
}

// New creates a concrete Runner from cfg, dispatching on cfg.Type.
// handler is called for every message the starter produces.
func New(cfg StarterConfig, handler Handler) (Runner, error) {
	switch cfg.Type {
	case "file_event":
		return newFileEventStarter(cfg, handler)
	case "rest":
		return newRestStarter(cfg, handler)
	default:
		return nil, fmt.Errorf("unknown starter type: %q", cfg.Type)
	}
}
