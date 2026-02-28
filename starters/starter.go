package starters

import "fmt"

// StarterConfig holds the common attributes shared by all starter types.
type StarterConfig struct {
	ID         string            `yaml:"id"`
	Type       string            `yaml:"type"`
	Properties map[string]string `yaml:"properties"`
}

// Runner is the interface every concrete starter must implement.
type Runner interface {
	Start() error
	Stop() error
}

// New creates a concrete Runner from cfg, dispatching on cfg.Type.
func New(cfg StarterConfig) (Runner, error) {
	switch cfg.Type {
	case "file_event":
		return newFileEventStarter(cfg)
	default:
		return nil, fmt.Errorf("unknown starter type: %q", cfg.Type)
	}
}
