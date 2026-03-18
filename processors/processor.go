package processors

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"ems-bridge/components"
	"ems-bridge/messages"
)

// JmsPropConfig defines a single JMS message property to be set at send time.
// Value and Condition are expression strings evaluated against the current message.
type JmsPropConfig struct {
	Name      string `yaml:"name"`
	Value     string `yaml:"value"`
	Condition string `yaml:"condition"`
}

// ProcessorConfig holds the common attributes shared by all processor types.
// jmsProperties is read from inside the properties map in YAML:
//
//	properties:
//	  component-ref: ems-dev
//	  jmsProperties:
//	    - name: Foo
//	      value: "'bar'"
type ProcessorConfig struct {
	ID            string            `yaml:"id"`
	Type          string            `yaml:"type"`
	Properties    map[string]string // populated by UnmarshalYAML
	JmsProperties []JmsPropConfig   // populated by UnmarshalYAML
}

// UnmarshalYAML decodes a ProcessorConfig, extracting jmsProperties from inside
// the properties mapping before treating the remaining entries as plain strings.
func (c *ProcessorConfig) UnmarshalYAML(value *yaml.Node) error {
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
				return fmt.Errorf("processor %q: decoding jmsProperties: %w", c.ID, err)
			}
		} else {
			c.Properties[key] = val.Value
		}
	}
	return nil
}

// Runner is the interface every concrete processor must implement.
type Runner interface {
	Process(msg *messages.Message) error
}

// New creates a concrete Runner from cfg, dispatching on cfg.Type.
// registry is used by processors that reference a named component (e.g. jms_send).
func New(cfg ProcessorConfig, registry components.Registry) (Runner, error) {
	switch cfg.Type {
	case "jms_send":
		return newJmsSendProcessor(cfg, registry)
	case "transform":
		return newTransformerProcessor(cfg)
	default:
		return nil, fmt.Errorf("unknown processor type: %q", cfg.Type)
	}
}
