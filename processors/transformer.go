package processors

import (
	"fmt"

	exprlib "github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"ems-bridge/messages"
)

// TransformerType enumerates the supported transformation modes.
type TransformerType string

const (
	TransformerTypeCustom   TransformerType = "custom"
	TransformerTypeTemplate TransformerType = "template"
	TransformerTypeXSLT     TransformerType = "xslt"
)

// scriptEnv is the expression environment exposed to custom scripts.
// Fields map directly to what the script can reference by name.
type scriptEnv struct {
	Payload    any
	Headers    map[string]string
	Properties map[string]any
}

// TransformerProcessor transforms a message according to its configured type.
type TransformerProcessor struct {
	ProcessorConfig
	transformerType TransformerType
	program         *vm.Program // non-nil only for TransformerTypeCustom
}

func newTransformerProcessor(cfg ProcessorConfig) (*TransformerProcessor, error) {
	p := cfg.Properties

	rawType, ok := p["type"]
	if !ok {
		return nil, fmt.Errorf("processor %q: missing required property \"type\"", cfg.ID)
	}

	var ttype TransformerType
	switch TransformerType(rawType) {
	case TransformerTypeCustom, TransformerTypeTemplate, TransformerTypeXSLT:
		ttype = TransformerType(rawType)
	default:
		return nil, fmt.Errorf("processor %q: unknown transformer type %q: must be custom, template or xslt", cfg.ID, rawType)
	}

	tp := &TransformerProcessor{
		ProcessorConfig: cfg,
		transformerType: ttype,
	}

	if ttype == TransformerTypeCustom {
		script, ok := p["script"]
		if !ok || script == "" {
			return nil, fmt.Errorf("processor %q: transformer type \"custom\" requires property \"script\"", cfg.ID)
		}
		program, err := exprlib.Compile(script, exprlib.Env(scriptEnv{}))
		if err != nil {
			return nil, fmt.Errorf("processor %q: compiling script: %w", cfg.ID, err)
		}
		tp.program = program
	}

	return tp, nil
}

func (t *TransformerProcessor) Process(msg *messages.Message) error {
	switch t.transformerType {
	case TransformerTypeCustom:
		return t.processCustom(msg)
	default:
		return fmt.Errorf("TransformerProcessor %q: type %q not yet implemented", t.ID, t.transformerType)
	}
}

func (t *TransformerProcessor) processCustom(msg *messages.Message) error {
	env := scriptEnv{
		Payload:    msg.Payload,
		Headers:    msg.Headers(),
		Properties: msg.Properties,
	}
	result, err := exprlib.Run(t.program, env)
	if err != nil {
		return fmt.Errorf("TransformerProcessor %q: evaluating script: %w", t.ID, err)
	}
	msg.Payload = result
	return nil
}
