package starters

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	exprlib "github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"ems-bridge/components"
	"ems-bridge/components/jms/tibco"
	"ems-bridge/messages"

)

// msgEnv is the expression environment available to jmsProperties expressions.
type msgEnv struct {
	Payload    any
	Headers    map[string]string
	Properties map[string]any
}

// jmsPropDef holds pre-compiled expression programs for a single JMS property.
type jmsPropDef struct {
	name      string
	value     *vm.Program
	condition *vm.Program // nil when no condition was specified
}

// JmsQueueConsumerStarter polls a JMS queue and forwards each message downstream.
type JmsQueueConsumerStarter struct {
	StarterConfig
	component       *components.JmsComponent
	queueName       string
	acknowledgeMode tibco.AcknowledgeMode
	messageSelector string
	consumerCount   int
	jmsProps        []jmsPropDef
	done            chan struct{}
	stopOnce        sync.Once
	handler         Handler
}

func newJmsQueueConsumerStarter(cfg StarterConfig, handler Handler) (*JmsQueueConsumerStarter, error) {
	if cfg.Registry == nil {
		return nil, fmt.Errorf("starter %q: no component registry available", cfg.ID)
	}

	ref := cfg.Properties["component-ref"]
	if ref == "" {
		return nil, fmt.Errorf("starter %q: missing required property \"component-ref\"", cfg.ID)
	}
	runner, ok := cfg.Registry[ref]
	if !ok {
		return nil, fmt.Errorf("starter %q: component %q not found in registry", cfg.ID, ref)
	}
	jmsComp, ok := runner.(*components.JmsComponent)
	if !ok {
		return nil, fmt.Errorf("starter %q: component %q is not a JmsComponent", cfg.ID, ref)
	}

	queueName := cfg.Properties["queueName"]
	if queueName == "" {
		return nil, fmt.Errorf("starter %q: missing required property \"queueName\"", cfg.ID)
	}

	// acknowledgementMode
	acknowledgeMode := tibco.AcknowledgeAuto
	switch cfg.Properties["acknowledgementMode"] {
	case "", "AUTO":
		acknowledgeMode = tibco.AcknowledgeAuto
	case "CLIENT":
		acknowledgeMode = tibco.AcknowledgeClient
	case "DUPS_OK":
		acknowledgeMode = tibco.AcknowledgeDupsOK
	default:
		return nil, fmt.Errorf("starter %q: acknowledgementMode must be AUTO, CLIENT or DUPS_OK", cfg.ID)
	}

	// messageSelector
	messageSelector := cfg.Properties["messageSelector"]

	// consumerCount
	consumerCount := 1
	if c := cfg.Properties["consumerCount"]; c != "" {
		n, err := strconv.Atoi(c)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("starter %q: consumerCount must be a positive integer", cfg.ID)
		}
		consumerCount = n
	}

	// jmsProperties
	env := exprlib.Env(msgEnv{})
	props := make([]jmsPropDef, 0, len(cfg.JmsProperties))
	for _, pc := range cfg.JmsProperties {
		if pc.Name == "" {
			return nil, fmt.Errorf("starter %q: jmsProperties entry missing required field \"name\"", cfg.ID)
		}
		if pc.Value == "" {
			return nil, fmt.Errorf("starter %q: jmsProperties entry %q missing required field \"value\"", cfg.ID, pc.Name)
		}
		valProg, err := exprlib.Compile(pc.Value, env)
		if err != nil {
			return nil, fmt.Errorf("starter %q: jmsProperties %q value: %w", cfg.ID, pc.Name, err)
		}
		def := jmsPropDef{name: pc.Name, value: valProg}
		if pc.Condition != "" {
			condProg, err := exprlib.Compile(pc.Condition, env, exprlib.AsBool())
			if err != nil {
				return nil, fmt.Errorf("starter %q: jmsProperties %q condition: %w", cfg.ID, pc.Name, err)
			}
			def.condition = condProg
		}
		props = append(props, def)
	}

	return &JmsQueueConsumerStarter{
		StarterConfig:   cfg,
		component:       jmsComp,
		queueName:       queueName,
		acknowledgeMode: acknowledgeMode,
		messageSelector: messageSelector,
		consumerCount:   consumerCount,
		jmsProps:        props,
		done:            make(chan struct{}),
		handler:         handler,
	}, nil
}

func (s *JmsQueueConsumerStarter) Start() error {
	conn := s.component.Conn()
	if conn == nil {
		return fmt.Errorf("JmsQueueConsumerStarter %q: component %q has no active tibems connection", s.ID, s.component.Name)
	}

	slog.Info("JmsQueueConsumerStarter starting",
		"id", s.ID, "queue", s.queueName,
		"consumerCount", s.consumerCount, "acknowledgeMode", s.acknowledgeMode)

	for i := 0; i < s.consumerCount; i++ {
		go func() {
			err := conn.ConsumeQueue(s.queueName, s.messageSelector, s.acknowledgeMode, func(payload, replyTo string, jmsProps map[string]string) error {
				jmsProps["starter.id"] = s.ID
				msg := messages.NewMessage(payload, jmsProps, map[string]any{})

				env := msgEnv{
					Payload:    msg.Payload,
					Headers:    msg.Headers(),
					Properties: msg.Properties,
				}
				for _, def := range s.jmsProps {
					if def.condition != nil {
						cond, err := exprlib.Run(def.condition, env)
						if err != nil {
							return fmt.Errorf("evaluating condition for jmsProperty %q: %w", def.name, err)
						}
						if !cond.(bool) {
							continue
						}
					}
					val, err := exprlib.Run(def.value, env)
					if err != nil {
						return fmt.Errorf("evaluating value for jmsProperty %q: %w", def.name, err)
					}
					msg.Properties[def.name] = fmt.Sprintf("%v", val)
				}

				slog.Info("JmsQueueConsumerStarter: message received", "id", s.ID, "queue", s.queueName)
				msg.Print()

				if err := s.handler(msg); err != nil {
					return err
				}

				if replyTo != "" {
					var replyBody string
					switch v := msg.Payload.(type) {
					case string:
						replyBody = v
					case []byte:
						replyBody = string(v)
					default:
						replyBody = fmt.Sprintf("%v", v)
					}
					slog.Info("JmsQueueConsumerStarter: sending reply", "id", s.ID, "replyTo", replyTo)
					_, err := conn.PublishTextMessage(replyTo, tibco.Queue, replyBody, nil, tibco.SendConfig{
						DeliveryMode: tibco.DeliveryPersistent,
						Priority:     4,
					})
					return err
				}

				return nil
			}, s.done)
			if err != nil {
				slog.Error("JmsQueueConsumerStarter: consumer stopped with error",
					"id", s.ID, "queue", s.queueName, "err", err)
			}
		}()
	}

	return nil
}

func (s *JmsQueueConsumerStarter) Stop() error {
	s.stopOnce.Do(func() { close(s.done) })
	slog.Info("JmsQueueConsumerStarter stopped", "id", s.ID)
	return nil
}
