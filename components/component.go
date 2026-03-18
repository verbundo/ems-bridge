package components

import "fmt"

// Component is the base struct holding common attributes shared by all component types.
type Component struct {
	Name       string            `yaml:"name"`
	Type       string            `yaml:"type"`
	Properties map[string]string `yaml:"properties"`
}

// Runner is the interface every concrete component must implement.
type Runner interface {
	Start() error
	Stop() error
}

// Registry maps component names to their live Runner instances.
// It is built once after all components are created and passed to routes so
// processors can resolve their component-ref at construction time.
type Registry map[string]Runner

// New creates a concrete Runner from cfg, dispatching on cfg.Type.
func New(cfg Component) (Runner, error) {
	switch cfg.Type {
	case "jms":
		return newJmsComponent(cfg)
	default:
		return nil, fmt.Errorf("unknown component type: %q", cfg.Type)
	}
}
