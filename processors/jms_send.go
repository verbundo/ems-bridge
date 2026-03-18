package processors

import (
	"fmt"

	"ems-bridge/components"
	"ems-bridge/messages"
)

// JmsSendProcessor sends messages to a JMS destination.
type JmsSendProcessor struct {
	ProcessorConfig
	component       *components.JmsComponent
	destinationType string
	destination     string
}

func newJmsSendProcessor(cfg ProcessorConfig, registry components.Registry) (*JmsSendProcessor, error) {
	p := cfg.Properties
	ref := p["component-ref"]

	runner, ok := registry[ref]
	if !ok {
		return nil, fmt.Errorf("processor %q: component %q not found in registry", cfg.ID, ref)
	}
	jmsComp, ok := runner.(*components.JmsComponent)
	if !ok {
		return nil, fmt.Errorf("processor %q: component %q is not a JmsComponent", cfg.ID, ref)
	}

	return &JmsSendProcessor{
		ProcessorConfig: cfg,
		component:       jmsComp,
		destinationType: p["destination-type"],
		destination:     p["destination"],
	}, nil
}

func (p *JmsSendProcessor) Process(msg *messages.Message) error {
	fmt.Printf("JmsSendProcessor %q: sending to %s %q via component %q\n",
		p.ID, p.destinationType, p.destination, p.component.Name)
	return nil
}
