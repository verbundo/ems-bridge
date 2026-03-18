package processors

import (
	"fmt"
	"strconv"

	exprlib "github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"ems-bridge/components"
	"ems-bridge/components/jms/tibco"
	"ems-bridge/messages"
)

// jmsPropDef holds pre-compiled expression programs for a single JMS property.
type jmsPropDef struct {
	name      string
	value     *vm.Program
	condition *vm.Program // nil when no condition was specified
}

// JmsSendProcessor sends messages to a JMS destination.
type JmsSendProcessor struct {
	ProcessorConfig
	component       *components.JmsComponent
	destinationType string
	destination     string
	jmsProps        []jmsPropDef
	// QoS
	deliveryMode tibco.DeliveryMode
	priority     int
	timeToLive   int64 // ms; 0 = no expiration
	// Request-reply
	expectReply     bool
	useTmpReplyDest bool
	replyDestProg   *vm.Program // compiled expression → string; nil if not set
	replyTimeout    int64       // ms; 0 = wait forever
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

	env := exprlib.Env(scriptEnv{})

	// --- jmsProperties ---
	props := make([]jmsPropDef, 0, len(cfg.JmsProperties))
	for _, pc := range cfg.JmsProperties {
		if pc.Name == "" {
			return nil, fmt.Errorf("processor %q: jmsProperties entry missing required field \"name\"", cfg.ID)
		}
		if pc.Value == "" {
			return nil, fmt.Errorf("processor %q: jmsProperties entry %q missing required field \"value\"", cfg.ID, pc.Name)
		}
		valProg, err := exprlib.Compile(pc.Value, env)
		if err != nil {
			return nil, fmt.Errorf("processor %q: jmsProperties %q value: %w", cfg.ID, pc.Name, err)
		}
		def := jmsPropDef{name: pc.Name, value: valProg}
		if pc.Condition != "" {
			condProg, err := exprlib.Compile(pc.Condition, env, exprlib.AsBool())
			if err != nil {
				return nil, fmt.Errorf("processor %q: jmsProperties %q condition: %w", cfg.ID, pc.Name, err)
			}
			def.condition = condProg
		}
		props = append(props, def)
	}

	// --- deliveryMode ---
	deliveryMode := tibco.DeliveryPersistent
	if dm := p["deliveryMode"]; dm == "NON_PERSISTENT" {
		deliveryMode = tibco.DeliveryNonPersistent
	} else if dm != "" && dm != "PERSISTENT" {
		return nil, fmt.Errorf("processor %q: deliveryMode must be PERSISTENT or NON_PERSISTENT, got %q", cfg.ID, dm)
	}

	// --- priority ---
	priority := 4
	if ps := p["priority"]; ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil || n < 0 || n > 9 {
			return nil, fmt.Errorf("processor %q: priority must be an integer 0-9", cfg.ID)
		}
		priority = n
	}

	// --- expiration (TTL in ms) ---
	var timeToLive int64
	if es := p["expiration"]; es != "" {
		n, err := strconv.ParseInt(es, 10, 64)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("processor %q: expiration must be a non-negative integer (ms)", cfg.ID)
		}
		timeToLive = n
	}

	// --- expectReply + reply-* properties ---
	expectReply := p["expectReply"] == "true"

	var (
		useTmpReplyDest bool
		replyDestProg   *vm.Program
		replyTimeout    int64
	)
	if expectReply {
		useTmpReplyDest = p["useTmpReplyDestination"] == "true"

		if rd := p["replyDestination"]; rd != "" {
			prog, err := exprlib.Compile(rd, env)
			if err != nil {
				return nil, fmt.Errorf("processor %q: replyDestination: %w", cfg.ID, err)
			}
			replyDestProg = prog
		}

		if rt := p["replyTimeout"]; rt != "" {
			n, err := strconv.ParseInt(rt, 10, 64)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("processor %q: replyTimeout must be a non-negative integer (ms)", cfg.ID)
			}
			replyTimeout = n
		}
	}

	return &JmsSendProcessor{
		ProcessorConfig: cfg,
		component:       jmsComp,
		destinationType: p["destination-type"],
		destination:     p["destination"],
		jmsProps:        props,
		deliveryMode:    deliveryMode,
		priority:        priority,
		timeToLive:      timeToLive,
		expectReply:     expectReply,
		useTmpReplyDest: useTmpReplyDest,
		replyDestProg:   replyDestProg,
		replyTimeout:    replyTimeout,
	}, nil
}

func (p *JmsSendProcessor) Process(msg *messages.Message) error {
	conn := p.component.Conn()
	if conn == nil {
		return fmt.Errorf("JmsSendProcessor %q: component %q has no active tibems connection", p.ID, p.component.Name)
	}

	var destType tibco.DestinationType
	switch p.destinationType {
	case "queue":
		destType = tibco.Queue
	case "topic":
		destType = tibco.Topic
	default:
		return fmt.Errorf("JmsSendProcessor %q: unknown destination-type %q", p.ID, p.destinationType)
	}

	env := scriptEnv{
		Payload:    msg.Payload,
		Headers:    msg.Headers(),
		Properties: msg.Properties,
	}

	// Evaluate jmsProperties.
	var jmsProps []tibco.JmsProp
	for _, def := range p.jmsProps {
		if def.condition != nil {
			cond, err := exprlib.Run(def.condition, env)
			if err != nil {
				return fmt.Errorf("JmsSendProcessor %q: evaluating condition for jmsProperty %q: %w", p.ID, def.name, err)
			}
			if !cond.(bool) {
				continue
			}
		}
		val, err := exprlib.Run(def.value, env)
		if err != nil {
			return fmt.Errorf("JmsSendProcessor %q: evaluating value for jmsProperty %q: %w", p.ID, def.name, err)
		}
		jmsProps = append(jmsProps, tibco.JmsProp{
			Name:  def.name,
			Value: fmt.Sprintf("%v", val),
		})
	}

	// Build send config.
	sendCfg := tibco.SendConfig{
		DeliveryMode:    p.deliveryMode,
		Priority:        p.priority,
		TimeToLive:      p.timeToLive,
		ExpectReply:     p.expectReply,
		UseTmpReplyDest: p.useTmpReplyDest,
		ReplyTimeout:    p.replyTimeout,
	}
	if p.replyDestProg != nil {
		val, err := exprlib.Run(p.replyDestProg, env)
		if err != nil {
			return fmt.Errorf("JmsSendProcessor %q: evaluating replyDestination: %w", p.ID, err)
		}
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("JmsSendProcessor %q: replyDestination must evaluate to a string, got %T", p.ID, val)
		}
		sendCfg.ReplyDestName = s
	}

	body := fmt.Sprintf("%v", msg.Payload)
	result, err := conn.PublishTextMessage(p.destination, destType, body, jmsProps, sendCfg)
	if err != nil {
		return err
	}

	msg.Properties["jms.message.id"] = result.MessageID
	fmt.Printf("JmsSendProcessor %q: sent to %s %q, jms.message.id=%s\n",
		p.ID, p.destinationType, p.destination, result.MessageID)

	if result.ReplyPayload != "" {
		msg.Payload = result.ReplyPayload
		fmt.Printf("JmsSendProcessor %q: received reply: %s\n", p.ID, result.ReplyPayload)
	}

	return nil
}
