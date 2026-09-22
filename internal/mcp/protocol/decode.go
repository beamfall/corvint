package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	integerPattern    = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	jsonNumberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(?:\.([0-9]+))?(?:[eE]([+-]?[0-9]+))?$`)
	metaKeyPattern    = regexp.MustCompile(`^(?:[A-Za-z](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z](?:[A-Za-z0-9-]*[A-Za-z0-9])?)*/)?(?:[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?)?$`)
)

type decodeFailure struct {
	invalidEnvelope bool
}

func (e *decodeFailure) Error() string { return "decode failure" }

// Decode accepts one newline-free stdio message body. Syntax, UTF-8,
// duplicate-key, size, and depth failures intentionally discard any ID.
func Decode(raw []byte) (Inbound, *DecodeError) {
	if len(raw) == 0 || len(raw) > MaxMessageBytes || !utf8.Valid(raw) {
		return Inbound{}, parseError()
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeValue(decoder, 0)
	if err != nil {
		var failure *decodeFailure
		if errors.As(err, &failure) && failure.invalidEnvelope {
			return Inbound{}, invalidRequest(nil)
		}
		return Inbound{}, parseError()
	}
	if _, err := decoder.Token(); err != io.EOF {
		return Inbound{}, parseError()
	}
	object, ok := value.(map[string]any)
	if !ok {
		return Inbound{}, invalidRequest(nil)
	}

	rawID, idPresent := object["id"]
	id, _, idValid := requestID(rawID)
	for key := range object {
		switch key {
		case "jsonrpc", "id", "method", "params":
		default:
			return Inbound{}, invalidRequest(validID(id, idValid))
		}
	}
	if object["jsonrpc"] != JSONRPCVersion {
		return Inbound{}, invalidRequest(validID(id, idValid))
	}
	method, ok := object["method"].(string)
	if !ok || method == "" || len(method) > 256 || strings.IndexFunc(method, isControl) >= 0 {
		return Inbound{}, invalidRequest(validID(id, idValid))
	}
	if _, present := object["result"]; present {
		return Inbound{}, invalidRequest(validID(id, idValid))
	}
	if _, present := object["error"]; present {
		return Inbound{}, invalidRequest(validID(id, idValid))
	}
	if idPresent && !idValid {
		return Inbound{}, invalidRequest(nil)
	}
	params := map[string]any(nil)
	if value, present := object["params"]; present {
		params, ok = value.(map[string]any)
		if !ok {
			return Inbound{}, invalidRequest(validID(id, idValid))
		}
	}
	if idPresent {
		return Inbound{ID: id, Method: method, Params: params}, nil
	}
	return Inbound{Method: method, Params: params, Notification: true}, nil
}

func parseError() *DecodeError {
	return &DecodeError{Code: CodeParseError, Message: "Parse error"}
}

func invalidRequest(id *ID) *DecodeError {
	return &DecodeError{Code: CodeInvalidRequest, Message: "Invalid Request", ID: id}
}

func validID(id *ID, valid bool) *ID {
	if valid {
		return id
	}
	return nil
}

func requestID(value any) (*ID, bool, bool) {
	if value == nil {
		return nil, false, false
	}
	switch typed := value.(type) {
	case string:
		id, err := NewStringID(typed)
		if err != nil {
			return nil, true, false
		}
		return &id, true, true
	case json.Number:
		raw := string(typed)
		if !integerPattern.MatchString(raw) || raw == "-0" {
			return nil, true, false
		}
		if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
			return nil, true, false
		}
		id := ID{raw: raw, key: "n:" + raw}
		return &id, true, true
	default:
		return nil, true, false
	}
}

// ParseIDValue validates a decoded JSON value as a protocol request ID.
func ParseIDValue(value any) (ID, bool) {
	id, present, valid := requestID(value)
	if !present || !valid {
		return ID{}, false
	}
	return *id, true
}

func decodeValue(decoder *json.Decoder, depth int) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, &decodeFailure{}
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return token, nil
	}
	if depth >= MaxJSONDepth {
		return nil, &decodeFailure{}
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, &decodeFailure{}
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, &decodeFailure{}
			}
			if _, exists := object[key]; exists {
				return nil, &decodeFailure{}
			}
			value, err := decodeValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
			return nil, &decodeFailure{}
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := decodeValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
			return nil, &decodeFailure{}
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter")
	}
}

func DecodeRequestMeta(params map[string]any) (RequestMeta, *RPCError) {
	if params == nil {
		return RequestMeta{}, InvalidParams("Missing request metadata")
	}
	rawMeta, ok := params["_meta"].(map[string]any)
	if !ok {
		return RequestMeta{}, InvalidParams("Missing request metadata")
	}
	for key := range rawMeta {
		if !metaKeyPattern.MatchString(key) {
			return RequestMeta{}, InvalidParams("Invalid request metadata")
		}
	}
	version, ok := rawMeta["io.modelcontextprotocol/protocolVersion"].(string)
	if !ok || version == "" || len(version) > MaxProtocolVersionBytes {
		return RequestMeta{}, InvalidParams("Missing protocol version")
	}
	capabilities, ok := rawMeta["io.modelcontextprotocol/clientCapabilities"].(map[string]any)
	if !ok || !clientCapabilitiesValid(capabilities) {
		return RequestMeta{}, InvalidParams("Missing client capabilities")
	}
	meta := RequestMeta{ProtocolVersion: version, ClientCapabilities: capabilities}
	if info, present := rawMeta["io.modelcontextprotocol/clientInfo"]; present {
		object, ok := info.(map[string]any)
		if !ok || !implementationValid(object) {
			return RequestMeta{}, InvalidParams("Invalid client information")
		}
		meta.ClientInfo = object
	}
	if level, present := rawMeta["io.modelcontextprotocol/logLevel"]; present {
		value, ok := level.(string)
		if !ok || !validLogLevel(value) {
			return RequestMeta{}, InvalidParams("Invalid log level")
		}
		meta.LogLevel = value
	}
	if token, present := rawMeta["progressToken"]; present {
		if !validOpaqueID(token) {
			return RequestMeta{}, InvalidParams("Invalid progress token")
		}
		meta.ProgressToken = token
	}
	return meta, nil
}

// DecodeCursor returns the optional cursor on current list requests.
func DecodeCursor(params map[string]any) (string, *RPCError) {
	if params == nil {
		return "", InvalidParams("Missing request parameters")
	}
	value, present := params["cursor"]
	if !present {
		return "", nil
	}
	cursor, ok := value.(string)
	if !ok {
		return "", InvalidParams("Invalid cursor")
	}
	return cursor, nil
}

func implementationValid(value map[string]any) bool {
	if _, ok := value["name"].(string); !ok {
		return false
	}
	if _, ok := value["version"].(string); !ok {
		return false
	}
	for key, field := range value {
		switch key {
		case "name", "version", "title", "description", "websiteUrl":
			if _, ok := field.(string); !ok {
				return false
			}
		case "icons":
			if !iconsValid(field) {
				return false
			}
		}
	}
	return true
}

func clientCapabilitiesValid(capabilities map[string]any) bool {
	for name, value := range capabilities {
		switch name {
		case "roots":
			if _, ok := value.(map[string]any); !ok {
				return false
			}
		case "sampling":
			if !capabilityObjectValid(value, "context", "tools") {
				return false
			}
		case "elicitation":
			if !capabilityObjectValid(value, "form", "url") {
				return false
			}
		case "experimental", "extensions":
			object, ok := value.(map[string]any)
			if !ok {
				return false
			}
			for key, settings := range object {
				if name == "extensions" && (strings.IndexByte(key, '/') < 1 || !metaKeyPattern.MatchString(key)) {
					return false
				}
				if !jsonObjectValid(settings) {
					return false
				}
			}
		}
	}
	return true
}

func capabilityObjectValid(value any, objectFields ...string) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for _, name := range objectFields {
		if field, present := object[name]; present {
			if !jsonObjectValid(field) {
				return false
			}
		}
	}
	return true
}

func jsonObjectValid(value any) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for _, field := range object {
		if !jsonValueValid(field) {
			return false
		}
	}
	return true
}

func jsonValueValid(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		return jsonObjectValid(typed)
	case []any:
		for _, field := range typed {
			if !jsonValueValid(field) {
				return false
			}
		}
		return true
	case string, bool:
		return true
	case json.Number:
		return jsonIntegerValid(typed)
	default:
		return false
	}
}

func jsonIntegerValid(value json.Number) bool {
	parts := jsonNumberPattern.FindStringSubmatch(string(value))
	if parts == nil {
		return false
	}
	digits := parts[1] + parts[2]
	if strings.Trim(digits, "0") == "" {
		return true
	}
	exponent := int64(0)
	if parts[3] != "" {
		parsed, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return !strings.HasPrefix(parts[3], "-")
		}
		exponent = parsed
	}
	fractionDigits := int64(len(parts[2]))
	if exponent >= fractionDigits {
		return true
	}
	if exponent < fractionDigits-int64(len(digits)) {
		return false
	}
	scale := fractionDigits - exponent
	return strings.Trim(digits[len(digits)-int(scale):], "0") == ""
}

func iconsValid(value any) bool {
	icons, ok := value.([]any)
	if !ok {
		return false
	}
	for _, value := range icons {
		icon, ok := value.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := icon["src"].(string); !ok {
			return false
		}
		if mimeType, present := icon["mimeType"]; present {
			if _, ok := mimeType.(string); !ok {
				return false
			}
		}
		if theme, present := icon["theme"]; present {
			value, ok := theme.(string)
			if !ok || value != "dark" && value != "light" {
				return false
			}
		}
		if sizes, present := icon["sizes"]; present {
			values, ok := sizes.([]any)
			if !ok {
				return false
			}
			for _, size := range values {
				if _, ok := size.(string); !ok {
					return false
				}
			}
		}
	}
	return true
}

func validOpaqueID(value any) bool {
	switch typed := value.(type) {
	case string:
		return len(typed) <= MaxMessageBytes
	case json.Number:
		raw := string(typed)
		if !integerPattern.MatchString(raw) || raw == "-0" {
			return false
		}
		_, err := strconv.ParseInt(raw, 10, 64)
		return err == nil
	default:
		return false
	}
}

func validLogLevel(value string) bool {
	switch value {
	case "alert", "critical", "debug", "emergency", "error", "info", "notice", "warning":
		return true
	default:
		return false
	}
}

func isControl(value rune) bool { return value < 0x20 || value == 0x7f }
