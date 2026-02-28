package messages

import (
	"encoding/json"
	"fmt"
)

// Header is a read-only string attribute set by a route starter at message
// creation time.
type Header struct {
	name  string
	value string
}

// NewHeader creates a Header with the given name and value.
func NewHeader(name, value string) Header {
	return Header{name: name, value: value}
}

func (h Header) Name() string  { return h.name }
func (h Header) Value() string { return h.value }

// Property is a named attribute attached to a Message.
type Property struct {
	Name  string
	Value any
}

// Message represents a unit of data received and processed by a route.
type Message struct {
	Payload    any
	Properties map[string]any
	headers    map[string]string
}

// NewMessage creates a Message with the given payload, properties and headers.
// Headers are immutable after creation.
func NewMessage(payload any, headers map[string]string, properties map[string]any) *Message {
	return &Message{
		Payload:    payload,
		headers:    headers,
		Properties: properties,
	}
}

// Header returns the header with the given name and whether it was found.
// Processors may read header values but cannot add, delete, or replace headers.
func (m *Message) Header(name string) (string, bool) {
	h, ok := m.headers[name]
	return h, ok
}

// Print outputs the message headers, properties and payload as indented JSON.
func (m *Message) Print() {
	type messageJSON struct {
		Headers    map[string]string `json:"headers"`
		Properties map[string]any    `json:"properties"`
		Payload    any               `json:"payload"`
	}

	out := messageJSON{
		Headers:    m.headers,
		Properties: m.Properties,
		Payload:    m.Payload,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Printf("error marshalling message: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
