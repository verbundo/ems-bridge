package starters

import (
	"fmt"
	"net/http"

	"gopkg.in/yaml.v3"

	"ems-bridge/components"
	"ems-bridge/messages"
)

// JmsPropConfig defines a JMS property to evaluate and attach to a received message.
type JmsPropConfig struct {
	Name      string `yaml:"name"`
	Value     string `yaml:"value"`
	Condition string `yaml:"condition"`
}

// StarterConfig holds the common attributes shared by all starter types.
// Mux and Registry are not parsed from YAML; they are injected by the route
// builder before starters are constructed.
// jmsProperties is extracted from inside the properties map, same as ProcessorConfig.
type StarterConfig struct {
	ID            string            `yaml:"id"`
	Type          string            `yaml:"type"`
	Properties    map[string]string // populated by UnmarshalYAML
	JmsProperties []JmsPropConfig   // populated by UnmarshalYAML
	Mux           *http.ServeMux    `yaml:"-"`
	Registry      components.Registry `yaml:"-"`
}

// UnmarshalYAML decodes a StarterConfig, extracting jmsProperties from inside
// the properties mapping before treating the remaining entries as plain strings.
func (c *StarterConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain struct {
		ID         string    `yaml:"id"`
		Type       string    `yaml:"type"`
		Properties yaml.Node `yaml:"properties"`
	}
	var p plain
	if err := value.Decode(&p); err != nil {
		return err
	}
	c.ID = p.ID
	c.Type = p.Type
	c.Properties = make(map[string]string)

	if p.Properties.Kind != yaml.MappingNode {
		return nil
	}
	nodes := p.Properties.Content
	for i := 0; i+1 < len(nodes); i += 2 {
		key := nodes[i].Value
		val := nodes[i+1]
		if key == "jmsProperties" {
			if err := val.Decode(&c.JmsProperties); err != nil {
				return fmt.Errorf("starter %q: decoding jmsProperties: %w", c.ID, err)
			}
		} else {
			c.Properties[key] = val.Value
		}
	}
	return nil
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
	case "jms_queue_consumer":
		return newJmsQueueConsumerStarter(cfg, handler)
	default:
		return nil, fmt.Errorf("unknown starter type: %q", cfg.Type)
	}
}
