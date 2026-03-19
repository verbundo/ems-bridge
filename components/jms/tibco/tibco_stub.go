//go:build !(tibco && (windows || linux))

// Package tibco provides lazy-loaded access to the TIBCO EMS C client DLL on
// Windows, enabling creation and lifecycle management of native EMS connections
// via the tibemsConnectionFactory / tibemsConnection C API.
package tibco

import "errors"

var errWindowsOnly = errors.New("tibco EMS is only supported on Windows")

type AcknowledgeMode uintptr

const (
	AcknowledgeAuto   AcknowledgeMode = 1
	AcknowledgeClient AcknowledgeMode = 2
	AcknowledgeDupsOK AcknowledgeMode = 3
)

type DestinationType int

const (
	Queue DestinationType = iota
	Topic
)

type DeliveryMode int32

const (
	DeliveryPersistent    DeliveryMode = 1
	DeliveryNonPersistent DeliveryMode = 2
)

type JmsProp struct {
	Name  string
	Value string
}

type SendConfig struct {
	DeliveryMode    DeliveryMode
	Priority        int
	TimeToLive      int64
	ExpectReply     bool
	UseTmpReplyDest bool
	ReplyDestName   string
	ReplyTimeout    int64
}

type SendResult struct {
	MessageID    string
	ReplyPayload string
}

type Connection struct{}

func EMS_NewConnection(serverURL, username, password string) (*Connection, error) {
	return nil, errWindowsOnly
}

func (c *Connection) Handle() uintptr { return 0 }

func (c *Connection) Start() error { return errWindowsOnly }
func (c *Connection) Stop() error  { return errWindowsOnly }
func (c *Connection) Close() error { return errWindowsOnly }

func (c *Connection) ConsumeQueue(queueName, messageSelector string, acknowledgeMode AcknowledgeMode, handler func(payload, replyTo string, props map[string]string) error, done <-chan struct{}) error {
	return errWindowsOnly
}

func (c *Connection) PublishTextMessage(destinationName string, destType DestinationType, message string, props []JmsProp, cfg SendConfig) (SendResult, error) {
	return SendResult{}, errWindowsOnly
}
