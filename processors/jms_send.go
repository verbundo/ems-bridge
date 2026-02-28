package processors

import "fmt"

// JmsSendProcessor sends messages to a JMS destination.
type JmsSendProcessor struct {
	ProcessorConfig
	componentRef    string
	destinationType string
	destination     string
}

func newJmsSendProcessor(cfg ProcessorConfig) (*JmsSendProcessor, error) {
	p := cfg.Properties
	return &JmsSendProcessor{
		ProcessorConfig: cfg,
		componentRef:    p["component-ref"],
		destinationType: p["destination-type"],
		destination:     p["destination"],
	}, nil
}

func (p *JmsSendProcessor) Process() error {
	fmt.Printf("JmsSendProcessor %q: sending to %s %q via component %q\n",
		p.ID, p.destinationType, p.destination, p.componentRef)
	return nil
}
