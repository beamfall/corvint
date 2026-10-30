package protocol

import "strings"

const LegacyVersion = "2025-11-25"

// ExtractVersionArgument removes the optional, closed protocol selector before
// the command validates its own required arguments.
func ExtractVersionArgument(arguments []string) ([]string, string, bool) {
	remaining := make([]string, 0, len(arguments))
	version := Version
	selected := false
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "--protocol-version" {
			remaining = append(remaining, arguments[index])
			continue
		}
		if selected || index+1 == len(arguments) {
			return nil, "", false
		}
		selected = true
		index++
		version = arguments[index]
		if version != Version && version != LegacyVersion {
			return nil, "", false
		}
	}
	for _, argument := range remaining {
		if selected && argument == "--version" {
			return nil, "", false
		}
	}
	return remaining, version, true
}

func DecodeLegacyInitialize(params map[string]any) (RequestMeta, *RPCError) {
	for key := range params {
		switch key {
		case "protocolVersion", "capabilities", "clientInfo", "_meta":
		default:
			return RequestMeta{}, InvalidParams("Invalid params")
		}
	}
	version, ok := params["protocolVersion"].(string)
	if !ok || version == "" || len(version) > MaxProtocolVersionBytes {
		return RequestMeta{}, InvalidParams("Invalid protocol version")
	}
	capabilities, ok := params["capabilities"].(map[string]any)
	if !ok || !clientCapabilitiesValid(capabilities) {
		return RequestMeta{}, InvalidParams("Invalid client capabilities")
	}
	if roots, ok := capabilities["roots"].(map[string]any); ok {
		if changed, present := roots["listChanged"]; present {
			if _, ok := changed.(bool); !ok {
				return RequestMeta{}, InvalidParams("Invalid roots capability")
			}
		}
	}
	info, ok := params["clientInfo"].(map[string]any)
	if !ok || !implementationValid(info) {
		return RequestMeta{}, InvalidParams("Invalid client information")
	}
	meta, rpcErr := DecodeLegacyRequestMeta(params)
	meta.ClientCapabilities = capabilities
	meta.ClientInfo = info
	return meta, rpcErr
}

// Legacy requests carry optional progress metadata; modern transport metadata
// must never change the connection's explicitly selected protocol.
func DecodeLegacyRequestMeta(params map[string]any) (RequestMeta, *RPCError) {
	meta := RequestMeta{ProtocolVersion: LegacyVersion}
	value, present := params["_meta"]
	if !present {
		return meta, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return RequestMeta{}, InvalidParams("Invalid request metadata")
	}
	for key := range object {
		if !metaKeyPattern.MatchString(key) || strings.HasPrefix(key, "io.modelcontextprotocol/") {
			return RequestMeta{}, InvalidParams("Invalid request metadata")
		}
	}
	if token, present := object["progressToken"]; present {
		if !validOpaqueID(token) {
			return RequestMeta{}, InvalidParams("Invalid progress token")
		}
		meta.ProgressToken = token
	}
	return meta, nil
}
