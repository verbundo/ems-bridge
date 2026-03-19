//go:build linux && tibco
// Linux implementation — uses CGo against the TIBCO EMS shared library.
// Requires the TIBCO EMS C client SDK (headers + libtibemsssl64.so / libtibems64.so).
// Set CGO_CFLAGS / CGO_LDFLAGS if the SDK is not on the default search paths, e.g.:
//   CGO_CFLAGS="-I/opt/tibco/ems/current/include"
//   CGO_LDFLAGS="-L/opt/tibco/ems/current/lib -ltibems64"

package tibco

/*
#cgo linux LDFLAGS: -ltibems64
#include <tibems/tibems.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"log/slog"
	"strconv"
	"unsafe"
)

// ---- exported types (mirror tibco.go on Windows) ----------------------------

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

// Connection wraps a TIBCO EMS ConnectionFactory and Connection handle.
type Connection struct {
	factory C.tibemsConnectionFactory
	handle  C.tibemsConnection
}

// ---- public API -------------------------------------------------------------

func EMS_NewConnection(serverURL, username, password string) (*Connection, error) {
	slog.Info("creating EMS connection")

	if s := C.tibems_Open(nil); s != C.TIBEMS_OK {
		return nil, cError("tibems_Open", s)
	}

	factory := C.tibemsConnectionFactory_Create()
	if factory == nil {
		return nil, fmt.Errorf("tibemsConnectionFactory_Create returned null")
	}

	cURL := C.CString(serverURL)
	defer C.free(unsafe.Pointer(cURL))
	if s := C.tibemsConnectionFactory_SetServerURL(factory, cURL); s != C.TIBEMS_OK {
		return nil, cError("tibemsConnectionFactory_SetServerURL", s)
	}

	cUser := C.CString(username)
	defer C.free(unsafe.Pointer(cUser))
	cPass := C.CString(password)
	defer C.free(unsafe.Pointer(cPass))

	var conn C.tibemsConnection
	if s := C.tibemsConnectionFactory_CreateConnection(factory, &conn, cUser, cPass); s != C.TIBEMS_OK {
		return nil, cError("tibemsConnectionFactory_CreateConnection", s)
	}

	if s := C.tibemsConnection_Start(conn); s != C.TIBEMS_OK {
		return nil, cError("tibemsConnection_Start", s)
	}

	return &Connection{factory: factory, handle: conn}, nil
}

func (c *Connection) Handle() uintptr { return uintptr(unsafe.Pointer(c.handle)) }

func (c *Connection) Start() error {
	if s := C.tibemsConnection_Start(c.handle); s != C.TIBEMS_OK {
		return cError("tibemsConnection_Start", s)
	}
	return nil
}

func (c *Connection) Stop() error {
	if s := C.tibemsConnection_Stop(c.handle); s != C.TIBEMS_OK {
		return cError("tibemsConnection_Stop", s)
	}
	return nil
}

func (c *Connection) Close() error {
	if s := C.tibemsConnection_Close(c.handle); s != C.TIBEMS_OK {
		return cError("tibemsConnection_Close", s)
	}
	return nil
}

func (c *Connection) ConsumeQueue(
	queueName, messageSelector string,
	acknowledgeMode AcknowledgeMode,
	handler func(payload, replyTo string, props map[string]string) error,
	done <-chan struct{},
) error {
	var session C.tibemsSession
	if s := C.tibemsConnection_CreateSession(
		c.handle, &session,
		C.tibems_bool(C.TIBEMS_FALSE),
		C.tibemsAcknowledgeMode(acknowledgeMode),
	); s != C.TIBEMS_OK {
		return cError("tibemsConnection_CreateSession", s)
	}
	defer C.tibemsSession_Close(session)

	cName := C.CString(queueName)
	defer C.free(unsafe.Pointer(cName))
	var dest C.tibemsQueue
	if s := C.tibemsQueue_Create(&dest, cName); s != C.TIBEMS_OK {
		return cError("tibemsQueue_Create", s)
	}
	defer C.tibemsDestination_Destroy(C.tibemsDestination(dest))

	var selPtr *C.char
	if messageSelector != "" {
		selPtr = C.CString(messageSelector)
		defer C.free(unsafe.Pointer(selPtr))
	}

	var consumer C.tibemsMsgConsumer
	if s := C.tibemsSession_CreateConsumer(
		session, &consumer,
		C.tibemsDestination(dest), selPtr,
		C.tibems_bool(C.TIBEMS_FALSE),
	); s != C.TIBEMS_OK {
		return cError("tibemsSession_CreateConsumer", s)
	}
	defer C.tibemsMsgConsumer_Close(consumer)

	for {
		select {
		case <-done:
			return nil
		default:
		}

		var msg C.tibemsMsg
		C.tibemsMsgConsumer_ReceiveTimeout(consumer, &msg, 250)
		if msg == nil {
			continue
		}

		var textPtr *C.char
		C.tibemsTextMsg_GetText(msg, &textPtr)
		text := C.GoString(textPtr)

		props := cMsgProperties(msg)
		std := cStandardHeaders(msg)
		for k, v := range std {
			props[k] = v
		}

		if err := handler(text, std["JMSReplyTo"], props); err != nil {
			C.tibemsMsg_Destroy(msg)
			return err
		}
		if acknowledgeMode == AcknowledgeClient {
			C.tibemsMsg_Acknowledge(msg)
		}
		C.tibemsMsg_Destroy(msg)
	}
}

func (c *Connection) PublishTextMessage(
	destinationName string,
	destType DestinationType,
	message string,
	props []JmsProp,
	cfg SendConfig,
) (SendResult, error) {
	var session C.tibemsSession
	if s := C.tibemsConnection_CreateSession(
		c.handle, &session,
		C.tibems_bool(C.TIBEMS_FALSE),
		C.tibemsAcknowledgeMode(AcknowledgeAuto),
	); s != C.TIBEMS_OK {
		return SendResult{}, cError("tibemsConnection_CreateSession", s)
	}
	defer C.tibemsSession_Close(session)

	cDestName := C.CString(destinationName)
	defer C.free(unsafe.Pointer(cDestName))

	var dest C.tibemsDestination
	if destType == Queue {
		var q C.tibemsQueue
		if s := C.tibemsQueue_Create(&q, cDestName); s != C.TIBEMS_OK {
			return SendResult{}, cError("tibemsQueue_Create", s)
		}
		dest = C.tibemsDestination(q)
	} else {
		var t C.tibemsTopic
		if s := C.tibemsTopic_Create(&t, cDestName); s != C.TIBEMS_OK {
			return SendResult{}, cError("tibemsTopic_Create", s)
		}
		dest = C.tibemsDestination(t)
	}
	defer C.tibemsDestination_Destroy(dest)

	var producer C.tibemsMsgProducer
	if s := C.tibemsSession_CreateProducer(session, &producer, dest); s != C.TIBEMS_OK {
		return SendResult{}, cError("tibemsSession_CreateProducer", s)
	}

	var msg C.tibemsMsg
	if s := C.tibemsTextMsg_Create(&msg); s != C.TIBEMS_OK {
		return SendResult{}, cError("tibemsTextMsg_Create", s)
	}
	defer C.tibemsMsg_Destroy(msg)

	cMsg := C.CString(message)
	defer C.free(unsafe.Pointer(cMsg))
	if s := C.tibemsTextMsg_SetText(msg, cMsg); s != C.TIBEMS_OK {
		return SendResult{}, cError("tibemsTextMsg_SetText", s)
	}

	for _, p := range props {
		cName := C.CString(p.Name)
		cVal := C.CString(p.Value)
		s := C.tibemsMsg_SetStringProperty(msg, cName, cVal)
		C.free(unsafe.Pointer(cName))
		C.free(unsafe.Pointer(cVal))
		if s != C.TIBEMS_OK {
			return SendResult{}, cError("tibemsMsg_SetStringProperty", s)
		}
	}

	var replyDest C.tibemsDestination
	if cfg.ExpectReply {
		if cfg.UseTmpReplyDest {
			var tmpQ C.tibemsTemporaryQueue
			if s := C.tibemsSession_CreateTemporaryQueue(session, &tmpQ); s != C.TIBEMS_OK {
				return SendResult{}, cError("tibemsSession_CreateTemporaryQueue", s)
			}
			replyDest = C.tibemsDestination(tmpQ)
		} else if cfg.ReplyDestName != "" {
			cReplyName := C.CString(cfg.ReplyDestName)
			defer C.free(unsafe.Pointer(cReplyName))
			var replyQ C.tibemsQueue
			if s := C.tibemsQueue_Create(&replyQ, cReplyName); s != C.TIBEMS_OK {
				return SendResult{}, cError("tibemsQueue_Create (reply)", s)
			}
			replyDest = C.tibemsDestination(replyQ)
		}
		if replyDest != nil {
			if s := C.tibemsMsg_SetReplyTo(msg, replyDest); s != C.TIBEMS_OK {
				return SendResult{}, cError("tibemsMsg_SetReplyTo", s)
			}
		}
	}

	if s := C.tibemsMsgProducer_SendEx(
		producer, msg,
		C.tibemsDeliveryMode(cfg.DeliveryMode),
		C.int(cfg.Priority),
		C.tibemsLong(cfg.TimeToLive),
	); s != C.TIBEMS_OK {
		return SendResult{}, cError("tibemsMsgProducer_SendEx", s)
	}

	var msgIDPtr *C.char
	if s := C.tibemsMsg_GetMessageID(msg, &msgIDPtr); s != C.TIBEMS_OK {
		return SendResult{}, cError("tibemsMsg_GetMessageID", s)
	}
	result := SendResult{MessageID: C.GoString(msgIDPtr)}

	if cfg.ExpectReply && replyDest != nil {
		var consumer C.tibemsMsgConsumer
		if s := C.tibemsSession_CreateConsumer(
			session, &consumer, replyDest, nil,
			C.tibems_bool(C.TIBEMS_FALSE),
		); s != C.TIBEMS_OK {
			return result, cError("tibemsSession_CreateConsumer", s)
		}
		defer C.tibemsMsgConsumer_Close(consumer)
		if cfg.UseTmpReplyDest {
			defer C.tibemsTemporaryQueue_Delete(C.tibemsTemporaryQueue(replyDest))
		}

		var replyMsg C.tibemsMsg
		if s := C.tibemsMsgConsumer_ReceiveTimeout(
			consumer, &replyMsg, C.tibemsLong(cfg.ReplyTimeout),
		); s != C.TIBEMS_OK {
			return result, cError("tibemsMsgConsumer_ReceiveTimeout", s)
		}
		if replyMsg != nil {
			var textPtr *C.char
			C.tibemsTextMsg_GetText(replyMsg, &textPtr)
			result.ReplyPayload = C.GoString(textPtr)
			C.tibemsMsg_Destroy(replyMsg)
		}
	}

	return result, nil
}

// ---- helpers ----------------------------------------------------------------

func cMsgProperties(msg C.tibemsMsg) map[string]string {
	props := make(map[string]string)
	var enum C.tibemsMsgEnum
	if s := C.tibemsMsg_GetPropertyNames(msg, &enum); s != C.TIBEMS_OK || enum == nil {
		return props
	}
	defer C.tibemsMsgEnum_Destroy(enum)

	for {
		var namePtr *C.char
		if s := C.tibemsMsgEnum_GetNextName(enum, &namePtr); s == C.TIBEMS_END_OF_DATA || namePtr == nil {
			break
		} else if s != C.TIBEMS_OK {
			break
		}
		var valPtr *C.char
		if sv := C.tibemsMsg_GetStringProperty(msg, namePtr, &valPtr); sv == C.TIBEMS_OK && valPtr != nil {
			props[C.GoString(namePtr)] = C.GoString(valPtr)
		}
	}
	return props
}

func cStandardHeaders(msg C.tibemsMsg) map[string]string {
	h := make(map[string]string)

	var msgIDPtr *C.char
	if s := C.tibemsMsg_GetMessageID(msg, &msgIDPtr); s == C.TIBEMS_OK && msgIDPtr != nil {
		h["JMSMessageID"] = C.GoString(msgIDPtr)
	}

	var ts C.tibemsLong
	if s := C.tibemsMsg_GetTimestamp(msg, &ts); s == C.TIBEMS_OK {
		h["JMSTimestamp"] = strconv.FormatInt(int64(ts), 10)
	}

	var dm C.tibemsDeliveryMode
	if s := C.tibemsMsg_GetDeliveryMode(msg, &dm); s == C.TIBEMS_OK {
		switch DeliveryMode(dm) {
		case DeliveryPersistent:
			h["JMSDeliveryMode"] = "PERSISTENT"
		case DeliveryNonPersistent:
			h["JMSDeliveryMode"] = "NON_PERSISTENT"
		default:
			h["JMSDeliveryMode"] = strconv.Itoa(int(dm))
		}
	}

	var exp C.tibemsLong
	if s := C.tibemsMsg_GetExpiration(msg, &exp); s == C.TIBEMS_OK {
		h["JMSExpiration"] = strconv.FormatInt(int64(exp), 10)
	}

	var pri C.int
	if s := C.tibemsMsg_GetPriority(msg, &pri); s == C.TIBEMS_OK {
		h["JMSPriority"] = strconv.Itoa(int(pri))
	}

	var redel C.tibems_bool
	if s := C.tibemsMsg_GetRedelivered(msg, &redel); s == C.TIBEMS_OK {
		if redel != C.TIBEMS_FALSE {
			h["JMSRedelivered"] = "true"
		} else {
			h["JMSRedelivered"] = "false"
		}
	}

	var corrPtr *C.char
	if s := C.tibemsMsg_GetCorrelationID(msg, &corrPtr); s == C.TIBEMS_OK && corrPtr != nil {
		h["JMSCorrelationID"] = C.GoString(corrPtr)
	}

	var typePtr *C.char
	if s := C.tibemsMsg_GetType(msg, &typePtr); s == C.TIBEMS_OK && typePtr != nil {
		h["JMSType"] = C.GoString(typePtr)
	}

	var msgDest C.tibemsDestination
	if s := C.tibemsMsg_GetDestination(msg, &msgDest); s == C.TIBEMS_OK && msgDest != nil {
		h["JMSDestination"] = cDestName(msgDest)
	}

	if rt := cReplyToName(msg); rt != "" {
		h["JMSReplyTo"] = rt
	}

	return h
}

func cDestName(dest C.tibemsDestination) string {
	var buf [1024]C.char
	C.tibemsDestination_GetName(dest, &buf[0], 1024)
	return C.GoString(&buf[0])
}

func cReplyToName(msg C.tibemsMsg) string {
	var dest C.tibemsDestination
	if s := C.tibemsMsg_GetReplyTo(msg, &dest); s != C.TIBEMS_OK || dest == nil {
		return ""
	}
	return cDestName(dest)
}

func cError(funcName string, s C.tibemsStatus) error {
	if desc := C.tibems_Status_GetText(s); desc != nil {
		return fmt.Errorf("%s failed (status %d): %s", funcName, int(s), C.GoString(desc))
	}
	return fmt.Errorf("%s failed (status %d)", funcName, int(s))
}
