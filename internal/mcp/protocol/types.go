// Package protocol implements bounded MCP 2026-07-28 wire types.
package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	Version        = "2026-07-28"
	JSONRPCVersion = "2.0"

	MaxMessageBytes = 1 << 20
	MaxJSONDepth    = 64
	// Reflectable request scalars are smaller than a frame so every error
	// response remains independently bounded.
	MaxRequestIDBytes       = 128
	MaxProtocolVersionBytes = 64

	CodeParseError                 = -32700
	CodeInvalidRequest             = -32600
	CodeMethodNotFound             = -32601
	CodeInvalidParams              = -32602
	CodeInternalError              = -32603
	CodeUnsupportedProtocolVersion = -32022
)

// ID preserves the type and exact integer spelling of a JSON-RPC request ID.
type ID struct {
	raw string
	key string
}

func NewStringID(value string) (ID, error) {
	raw, err := json.Marshal(value)
	if err != nil || len(value) > MaxRequestIDBytes {
		return ID{}, fmt.Errorf("invalid request id")
	}
	return ID{raw: string(raw), key: "s:" + value}, nil
}

func NewIntegerID(value int64) ID {
	raw := fmt.Sprintf("%d", value)
	return ID{raw: raw, key: "n:" + raw}
}

func (id ID) Valid() bool { return id.raw != "" }
func (id ID) Key() string { return id.key }

func (id ID) MarshalJSON() ([]byte, error) {
	if !id.Valid() {
		return nil, fmt.Errorf("invalid request id")
	}
	return []byte(id.raw), nil
}

type Inbound struct {
	ID           *ID
	Method       string
	Params       map[string]any
	Notification bool
}

type RequestMeta struct {
	ProtocolVersion    string
	ClientCapabilities map[string]any
	ClientInfo         map[string]any
	LogLevel           string
	ProgressToken      any
}

type Request struct {
	ID     ID
	Method string
	Params map[string]any
	Meta   RequestMeta
}

type RPCError struct {
	Code    int
	Message string
	Data    any
}

func NewError(code int, message string) *RPCError {
	return &RPCError{Code: code, Message: message}
}

func InvalidParams(message string) *RPCError {
	return NewError(CodeInvalidParams, message)
}

func MethodNotFound() *RPCError {
	return NewError(CodeMethodNotFound, "Method not found")
}

type DecodeError struct {
	Code    int
	Message string
	ID      *ID
}

func (e *DecodeError) Error() string { return e.Message }
