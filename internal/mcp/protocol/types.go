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

// InvalidParamsData is the closed data member of a -32602 error (MCPV0-952):
// a reason code, the offending field as a dotted path, and when known the
// values or the rule it must meet.
type InvalidParamsData struct {
	Reason  string `json:"reason"`
	Field   string `json:"field"`
	Allowed []any  `json:"allowed,omitempty"`
	Rule    string `json:"rule,omitempty"`
	// Code, Message, Retryable and Next are the DMF-V1-010 error members every MCP tool error
	// carries; the writer fills them through Described, after the MCPV0-952 members.
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Retryable bool   `json:"retryable"`
	Next      string `json:"next"`
}

// ServerBusyCode is the tool error of a request that arrives at the concurrency limit; it is
// retryable (DMF-V1-010). CodeServerBusy is its JSON-RPC code outside tools/call, in the
// implementation-defined server-error range.
const (
	ServerBusyCode = "server-busy"
	CodeServerBusy = -32000
)

// Described fills the DMF-V1-010 members of an MCPV0-952 data object: the reason as code, a message
// naming the field and what is wrong with it, never retryable, and the argument change that fixes
// it.
func (data InvalidParamsData) Described() InvalidParamsData {
	if data.Code == "" {
		data.Code = data.Reason
	}
	what, next := invalidParamsWording(data)
	if data.Message == "" {
		data.Message = "Invalid params: " + data.Field + " " + what
	}
	if data.Next == "" {
		data.Next = next
	}
	data.Retryable = false
	return data
}

func invalidParamsWording(data InvalidParamsData) (string, string) {
	field := data.Field
	switch data.Reason {
	case "unknown-param", "unknown-argument":
		return "is not accepted here", "remove " + field
	case "missing-argument", "missing-param", "missing-tool-name":
		return "is required and missing", "add " + field
	case "unknown-tool":
		return "names no listed tool", "set " + field + " to a tool tools/list returns"
	case "invalid-type":
		return "must be of type " + data.Rule, "give " + field + " as " + data.Rule
	case "invalid-enum", "invalid-const":
		return "is not one of the allowed values", "set " + field + " to one of the allowed values"
	case "pattern-mismatch":
		return "does not match the required pattern", "change " + field + " to match the rule"
	case "out-of-range":
		return "is out of range (" + data.Rule + ")", "change " + field + " to meet " + data.Rule
	case "duplicate-item":
		return "repeats an earlier item", "remove the duplicate " + field
	case "invalid-cursor":
		return "is not a cursor this server issued", "omit " + field + " or send an empty cursor"
	}
	if data.Rule != "" {
		return "breaks the rule " + data.Rule, "change " + field + " to meet the rule"
	}
	return "is invalid (" + data.Reason + ")", "change " + field
}

// MaxInvalidParamsField bounds the request-supplied field path an
// invalid-params error echoes (MCPV0-952); a longer one drops the data, so an
// oversized key can never grow the error past the output frame.
const MaxInvalidParamsField = 1024

// InvalidParamsAt is InvalidParams with its reason and offending field.
func InvalidParamsAt(message, reason, field string) *RPCError {
	failure := InvalidParams(message)
	if len(field) <= MaxInvalidParamsField {
		failure.Data = InvalidParamsData{Reason: reason, Field: field}
	}
	return failure
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
