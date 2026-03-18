// Package tibco provides lazy-loaded access to the TIBCO EMS C client DLL on
// Windows, enabling creation and lifecycle management of native EMS connections
// via the tibemsConnectionFactory / tibemsConnection C API.
package tibco

import (
	"fmt"
	"log/slog"
	"syscall"
	"unsafe"
)

// status mirrors tibemsStatus, the integer return code used throughout the
// TIBCO EMS C API. Zero means success.
type status int32

const statusOK status = 0

// Connection wraps a pair of opaque TIBCO EMS handles — a ConnectionFactory
// and the Connection itself — obtained by calling into the native C DLL.
type Connection struct {
	factory   uintptr
	handle    uintptr
	procStart *syscall.LazyProc
	procStop  *syscall.LazyProc
	procClose *syscall.LazyProc
}

var (
	dllName = "tibems.dll"
	dll = syscall.NewLazyDLL(dllName)

	procFactory_Create            = dll.NewProc("tibemsConnectionFactory_Create")
	procFactory_SetServerURL      = dll.NewProc("tibemsConnectionFactory_SetServerURL")
	procFactory_CreateConnection  = dll.NewProc("tibemsConnectionFactory_CreateConnection")
	procFactory_Destroy           = dll.NewProc("tibemsConnectionFactory_Destroy")

	procConnection_Start         = dll.NewProc("tibemsConnection_Start")
	procConnection_CreateSession = dll.NewProc("tibemsConnection_CreateSession")
	procConnection_Close         = dll.NewProc("tibemsConnection_Close")

	procQueue_Create          = dll.NewProc("tibemsQueue_Create")
	procTopic_Create          = dll.NewProc("tibemsTopic_Create")
	procSession_CreateProducer = dll.NewProc("tibemsSession_CreateProducer")
	procSession_Close          = dll.NewProc("tibemsSession_Close")

	procDestination_Destroy = dll.NewProc("tibemsDestination_Destroy")

	procMsgProducer_Send   = dll.NewProc("tibemsMsgProducer_Send")
	procMsgProducer_SendEx = dll.NewProc("tibemsMsgProducer_SendEx")
	procMsgProducer_Close  = dll.NewProc("tibemsMsgProducer_Close")

	procTextMsg_Create  = dll.NewProc("tibemsTextMsg_Create")
	procTextMsg_SetText  = dll.NewProc("tibemsTextMsg_SetText")
	procTextMsg_GetText  = dll.NewProc("tibemsTextMsg_GetText")
	procMsg_Destroy             = dll.NewProc("tibemsMsg_Destroy")
	procMsg_SetReplyTo          = dll.NewProc("tibemsMsg_SetReplyTo")
	procMsg_GetMessageID        = dll.NewProc("tibemsMsg_GetMessageID")
	procMsg_SetStringProperty   = dll.NewProc("tibemsMsg_SetStringProperty")

	procSession_CreateTemporaryQueue = dll.NewProc("tibemsSession_CreateTemporaryQueue")
	procSession_CreateConsumer       = dll.NewProc("tibemsSession_CreateConsumer")
	procTemporaryQueue_Delete        = dll.NewProc("tibemsTemporaryQueue_Delete")

	procMsgConsumer_ReceiveTimeout = dll.NewProc("tibemsMsgConsumer_ReceiveTimeout")
	procMsgConsumer_Close          = dll.NewProc("tibemsMsgConsumer_Close")

	procStatus_GetText = dll.NewProc("tibems_Status_GetText")

	procOpen  = dll.NewProc("tibems_Open")
	procClose = dll.NewProc("tibems_Close")
)

const (
	tibemsOK              = uintptr(0)
	tibemsFALSE           = uintptr(0)
	tibemsAutoAcknowledge = uintptr(1) // TIBEMS_AUTO_ACKNOWLEDGE
	tibemsWaitForever     = uintptr(0) // TIBEMS_WAIT_FOREVER for ReceiveTimeout
)

type DestinationType int

const (
	Queue DestinationType = iota
	Topic
)

// DeliveryMode mirrors tibemsDeliveryMode.
type DeliveryMode int32

const (
	DeliveryPersistent    DeliveryMode = 1
	DeliveryNonPersistent DeliveryMode = 2
)

// JmsProp is a name/value pair set as a JMS string property on the outbound message.
type JmsProp struct {
	Name  string
	Value string
}

// SendConfig carries per-send QoS settings and optional request-reply parameters.
type SendConfig struct {
	DeliveryMode    DeliveryMode
	Priority        int
	TimeToLive      int64  // ms; 0 = no expiration
	ExpectReply     bool
	UseTmpReplyDest bool
	ReplyDestName   string // named reply-to queue (used when !UseTmpReplyDest)
	ReplyTimeout    int64  // ms; 0 = wait forever
}

// SendResult is returned by PublishTextMessage.
type SendResult struct {
	MessageID    string
	ReplyPayload string // non-empty only when ExpectReply=true and a reply was received
}

// Handle returns the raw tibemsConnection handle. Processors that need to
// create Sessions or MessageProducers can use this value in further DLL calls.
func (c *Connection) Handle() uintptr { return c.handle }

func (c *Connection) PublishTextMessage(destinationName string, destType DestinationType, message string, props []JmsProp, cfg SendConfig) (SendResult, error) {
	var session uintptr
	var dest uintptr
	var producer uintptr
	var msg uintptr

	if s, _, _ := procConnection_CreateSession.Call(
		c.handle,
		uintptr(unsafe.Pointer(&session)),
		tibemsFALSE,           // not transacted
		tibemsAutoAcknowledge, // auto acknowledge
	); s != tibemsOK {
		return SendResult{}, tibemsError("tibemsConnection_CreateSession", s)
	}

	destNameCStr, _ := syscall.BytePtrFromString(destinationName)

	if destType == Queue {
		if s, _, _ := procQueue_Create.Call(
			uintptr(unsafe.Pointer(&dest)),
			uintptr(unsafe.Pointer(destNameCStr)),
		); s != tibemsOK {
			return SendResult{}, tibemsError("tibemsQueue_Create", s)
		}
	} else {
		if s, _, _ := procTopic_Create.Call(
			uintptr(unsafe.Pointer(&dest)),
			uintptr(unsafe.Pointer(destNameCStr)),
		); s != tibemsOK {
			return SendResult{}, tibemsError("tibemsTopic_Create", s)
		}
	}

	if s, _, _ := procSession_CreateProducer.Call(
		session, uintptr(unsafe.Pointer(&producer)), dest,
	); s != tibemsOK {
		return SendResult{}, tibemsError("tibemsSession_CreateProducer", s)
	}

	if s, _, _ := procTextMsg_Create.Call(uintptr(unsafe.Pointer(&msg))); s != tibemsOK {
		return SendResult{}, tibemsError("tibemsTextMsg_Create", s)
	}

	messageCStr, _ := syscall.BytePtrFromString(message)
	if s, _, _ := procTextMsg_SetText.Call(msg, uintptr(unsafe.Pointer(messageCStr))); s != tibemsOK {
		return SendResult{}, tibemsError("tibemsTextMsg_SetText", s)
	}

	for _, p := range props {
		nameCStr, _ := syscall.BytePtrFromString(p.Name)
		valCStr, _ := syscall.BytePtrFromString(p.Value)
		if s, _, _ := procMsg_SetStringProperty.Call(
			msg,
			uintptr(unsafe.Pointer(nameCStr)),
			uintptr(unsafe.Pointer(valCStr)),
		); s != tibemsOK {
			return SendResult{}, tibemsError("tibemsMsg_SetStringProperty", s)
		}
	}

	// Set reply-to destination when request-reply is expected.
	var replyDest uintptr
	if cfg.ExpectReply {
		if cfg.UseTmpReplyDest {
			if s, _, _ := procSession_CreateTemporaryQueue.Call(
				session, uintptr(unsafe.Pointer(&replyDest)),
			); s != tibemsOK {
				return SendResult{}, tibemsError("tibemsSession_CreateTemporaryQueue", s)
			}
		} else if cfg.ReplyDestName != "" {
			replyNameCStr, _ := syscall.BytePtrFromString(cfg.ReplyDestName)
			if s, _, _ := procQueue_Create.Call(
				uintptr(unsafe.Pointer(&replyDest)),
				uintptr(unsafe.Pointer(replyNameCStr)),
			); s != tibemsOK {
				return SendResult{}, tibemsError("tibemsQueue_Create (reply)", s)
			}
		}
		if replyDest != 0 {
			if s, _, _ := procMsg_SetReplyTo.Call(msg, replyDest); s != tibemsOK {
				return SendResult{}, tibemsError("tibemsMsg_SetReplyTo", s)
			}
		}
	}

	if s, _, _ := procMsgProducer_SendEx.Call(
		producer, msg,
		uintptr(cfg.DeliveryMode),
		uintptr(cfg.Priority),
		uintptr(cfg.TimeToLive),
	); s != tibemsOK {
		return SendResult{}, tibemsError("tibemsMsgProducer_SendEx", s)
	}

	var msgIDPtr uintptr
	if s, _, _ := procMsg_GetMessageID.Call(msg, uintptr(unsafe.Pointer(&msgIDPtr))); s != tibemsOK {
		return SendResult{}, tibemsError("tibemsMsg_GetMessageID", s)
	}

	result := SendResult{MessageID: cString(msgIDPtr)}

	// Wait for reply when requested.
	if cfg.ExpectReply && replyDest != 0 {
		var consumer uintptr
		if s, _, _ := procSession_CreateConsumer.Call(
			session,
			uintptr(unsafe.Pointer(&consumer)),
			replyDest,
			0,           // no message selector
			tibemsFALSE, // noLocal = false
		); s != tibemsOK {
			return result, tibemsError("tibemsSession_CreateConsumer", s)
		}
		defer procMsgConsumer_Close.Call(consumer)
		if cfg.UseTmpReplyDest {
			defer procTemporaryQueue_Delete.Call(replyDest)
		}

		var replyMsg uintptr
		if s, _, _ := procMsgConsumer_ReceiveTimeout.Call(
			consumer,
			uintptr(unsafe.Pointer(&replyMsg)),
			uintptr(cfg.ReplyTimeout), // 0 = TIBEMS_WAIT_FOREVER
		); s != tibemsOK {
			return result, tibemsError("tibemsMsgConsumer_ReceiveTimeout", s)
		}
		if replyMsg != 0 {
			var textPtr uintptr
			procTextMsg_GetText.Call(replyMsg, uintptr(unsafe.Pointer(&textPtr)))
			result.ReplyPayload = cString(textPtr)
			procMsg_Destroy.Call(replyMsg)
		}
	}

	return result, nil
}

// NewConnection lazy-loads the TIBCO EMS C client DLL identified by dllName
// (e.g. "tibjms.dll"), creates a ConnectionFactory, sets serverURL on it, and
// calls CreateConnection with the supplied credentials.
//
// The returned Connection is not yet active; call Start to begin message
// delivery.
func EMS_NewConnection(serverURL, username, password string) (*Connection, error) {
	slog.Info("creating EMS connection")

	if err := dll.Load(); err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", dllName, err)
	}

	serverURLPtr, _ := syscall.BytePtrFromString(serverURL)
	usernamePtr, _ := syscall.BytePtrFromString(username)
	passwordPtr, _ := syscall.BytePtrFromString(password)

	var (
		factory     uintptr
		conn  uintptr
	)

	if s, _, _ := procOpen.Call(0); s != tibemsOK {
		return nil, tibemsError("tibems_Open", s)
	}

	factory, _, _ = procFactory_Create.Call()
	if factory == 0 {
		return nil, fmt.Errorf("tibemsConnectionFactory_Create failed: returned null")
	}

	if s, _, _ := procFactory_SetServerURL.Call(
		factory, uintptr(unsafe.Pointer(serverURLPtr)),
	); s != tibemsOK {
		return nil, tibemsError("tibemsConnectionFactory_SetServerURL", s)
	}

	if s, _, _ := procFactory_CreateConnection.Call(
		factory,
		uintptr(unsafe.Pointer(&conn)),
		uintptr(unsafe.Pointer(usernamePtr)),
		uintptr(unsafe.Pointer(passwordPtr)),
	); s != tibemsOK {
		return nil, tibemsError("tibemsConnectionFactory_CreateConnection", s)
	}

	if s, _, _ := procConnection_Start.Call(conn); s != tibemsOK {
		return nil, tibemsError("tibemsConnection_Start", s)
	}

	return &Connection{
		factory:   factory,
		handle:    conn,
		procStart: dll.NewProc("tibemsConnection_Start"),
		procStop:  dll.NewProc("tibemsConnection_Stop"),
		procClose: dll.NewProc("tibemsConnection_Close"),
	}, nil
}

// Start activates the connection and enables message delivery.
func (c *Connection) Start() error {
	r, _, _ := c.procStart.Call(c.handle)
	if status(r) != statusOK {
		return fmt.Errorf("tibemsConnection_Start: status %d", r)
	}
	return nil
}

// Stop suspends message delivery without closing the underlying connection.
func (c *Connection) Stop() error {
	r, _, _ := c.procStop.Call(c.handle)
	if status(r) != statusOK {
		return fmt.Errorf("tibemsConnection_Stop: status %d", r)
	}
	return nil
}

// Close releases the connection and all resources associated with it.
func (c *Connection) Close() error {
	r, _, _ := c.procClose.Call(c.handle)
	if status(r) != statusOK {
		return fmt.Errorf("tibemsConnection_Close: status %d", r)
	}
	return nil
}

// tibemsError formats a TIBCO EMS error with the status description from the DLL.
// Falls back to a numeric status if tibems_Status_GetText is not exported by this DLL version.
func tibemsError(funcName string, status uintptr) error {
	if procStatus_GetText.Find() == nil {
		var descPtr uintptr
		procStatus_GetText.Call(status, uintptr(unsafe.Pointer(&descPtr)))
		if descPtr != 0 {
			return fmt.Errorf("%s failed (status %d): %s", funcName, status, cString(descPtr))
		}
	}
	return fmt.Errorf("%s failed (status %d)", funcName, status)
}

// cString converts a null-terminated C string pointer to a Go string
func cString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var buf []byte
	for i := uintptr(0); ; i++ {
		b := *(*byte)(unsafe.Pointer(ptr + i))
		if b == 0 {
			break
		}
		buf = append(buf, b)
	}
	return string(buf)
}
