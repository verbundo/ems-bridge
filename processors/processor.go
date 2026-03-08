package processors

import (
	"fmt"

	"ems-bridge/messages"
)

// ProcessorConfig holds the common attributes shared by all processor types.
type ProcessorConfig struct {
	ID         string            `yaml:"id"`
	Type       string            `yaml:"type"`
	Properties map[string]string `yaml:"properties"`
}

// Runner is the interface every concrete processor must implement.
type Runner interface {
	Process(msg *messages.Message) error
}

// New creates a concrete Runner from cfg, dispatching on cfg.Type.
func New(cfg ProcessorConfig) (Runner, error) {
	switch cfg.Type {
	case "jms_send":
		return newJmsSendProcessor(cfg)
	case "transform":
		return newTransformerProcessor(cfg)
	default:
		return nil, fmt.Errorf("unknown processor type: %q", cfg.Type)
	}
}
