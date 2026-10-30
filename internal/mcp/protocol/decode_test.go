package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeStrictRequest(t *testing.T) {
	raw := []byte(` {"jsonrpc":"2.0","id":"q-1","method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}} `)
	message, protocolErr := Decode(raw)
	if protocolErr != nil {
		t.Fatal(protocolErr)
	}
	if message.ID == nil || message.ID.Key() != "s:q-1" || message.Method != "tools/list" || message.Notification {
		t.Fatalf("unexpected request: %+v", message)
	}
	meta, rpcErr := DecodeRequestMeta(message.Params)
	if rpcErr != nil || meta.ProtocolVersion != Version {
		t.Fatalf("meta=%+v err=%+v", meta, rpcErr)
	}
}

func TestDecodeRejectsHostileMessages(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		code int
	}{
		{"empty", nil, CodeParseError},
		{"invalid utf8", []byte{'{', 0xff, '}'}, CodeParseError},
		{"duplicate key", []byte(`{"jsonrpc":"2.0","jsonrpc":"2.0","id":1,"method":"x"}`), CodeParseError},
		{"batch", []byte(`[]`), CodeInvalidRequest},
		{"null id", []byte(`{"jsonrpc":"2.0","id":null,"method":"x"}`), CodeInvalidRequest},
		{"fraction id", []byte(`{"jsonrpc":"2.0","id":1.5,"method":"x"}`), CodeInvalidRequest},
		{"params array", []byte(`{"jsonrpc":"2.0","id":1,"method":"x","params":[]}`), CodeInvalidRequest},
		{"response", []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`), CodeInvalidRequest},
		{"unknown envelope member", []byte(`{"jsonrpc":"2.0","id":1,"method":"x","extra":true}`), CodeInvalidRequest},
		{"trailing", []byte(`{"jsonrpc":"2.0","id":1,"method":"x"}{}`), CodeParseError},
		{"oversize", []byte(strings.Repeat(" ", MaxMessageBytes+1)), CodeParseError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode(test.raw)
			if err == nil || err.Code != test.code {
				t.Fatalf("err=%+v want code %d", err, test.code)
			}
		})
	}
}

func TestDecodeBoundsDepth(t *testing.T) {
	nested := strings.Repeat(`{"a":`, MaxJSONDepth+2) + `0` + strings.Repeat(`}`, MaxJSONDepth+2)
	_, err := Decode([]byte(`{"jsonrpc":"2.0","id":1,"method":"x","params":` + nested + `}`))
	if err == nil || err.Code != CodeParseError {
		t.Fatalf("err=%+v", err)
	}
}

func TestDecodeRequestMeta(t *testing.T) {
	base := map[string]any{"_meta": map[string]any{
		"io.modelcontextprotocol/protocolVersion":    Version,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		"progressToken": json.Number("7"),
	}}
	meta, err := DecodeRequestMeta(base)
	if err != nil || meta.ProgressToken != json.Number("7") {
		t.Fatalf("meta=%+v err=%+v", meta, err)
	}
	base["_meta"].(map[string]any)["bad key"] = true
	if _, err := DecodeRequestMeta(base); err == nil || err.Code != CodeInvalidParams {
		t.Fatalf("err=%+v", err)
	}
}

func TestDecodeRequestMetaValidatesKnownClientSchemas(t *testing.T) {
	valid := []map[string]any{
		{},
		{"roots": map[string]any{}},
		{"sampling": map[string]any{"context": map[string]any{"levels": []any{json.Number("1.0"), true}}, "tools": map[string]any{}}},
		{"elicitation": map[string]any{"form": map[string]any{}, "url": map[string]any{}}},
		{"experimental": map[string]any{"x": map[string]any{"nested": []any{"value", json.Number("10e-1")}}}},
		{"extensions": map[string]any{"com.example/extension": map[string]any{}}},
		{"unknown.example/capability": true},
	}
	for _, capabilities := range valid {
		params := metaParams(capabilities, nil)
		if _, err := DecodeRequestMeta(params); err != nil {
			t.Fatalf("valid capabilities=%+v err=%+v", capabilities, err)
		}
	}
	invalid := []map[string]any{
		{"roots": true},
		{"sampling": []any{}},
		{"sampling": map[string]any{"tools": true}},
		{"elicitation": "form"},
		{"elicitation": map[string]any{"url": false}},
		{"experimental": map[string]any{"x": true}},
		{"experimental": map[string]any{"x": map[string]any{"null": nil}}},
		{"sampling": map[string]any{"context": map[string]any{"fraction": json.Number("1.5")}}},
		{"elicitation": map[string]any{"form": map[string]any{"nested": []any{nil}}}},
		{"extensions": map[string]any{"extension": map[string]any{}}},
		{"extensions": map[string]any{"com..example/extension": map[string]any{}}},
		{"extensions": map[string]any{"com.example/x": "yes"}},
	}
	for _, capabilities := range invalid {
		if _, err := DecodeRequestMeta(metaParams(capabilities, nil)); err == nil || err.Code != CodeInvalidParams {
			t.Fatalf("invalid capabilities accepted: %+v", capabilities)
		}
	}
}

func TestDecodeRequestMetaValidatesClientInfoKnownFields(t *testing.T) {
	valid := map[string]any{
		"name": "client", "version": "1", "title": "Client", "description": "MCP client",
		"websiteUrl": "https://example.invalid", "icons": []any{map[string]any{
			"src": "data:image/png;base64,AA==", "mimeType": "image/png", "sizes": []any{"16x16", "any"}, "theme": "dark",
		}},
	}
	if _, err := DecodeRequestMeta(metaParams(map[string]any{}, valid)); err != nil {
		t.Fatalf("valid client info: %+v", err)
	}
	invalid := []map[string]any{
		{"name": "client", "version": 1},
		{"name": "client", "version": "1", "title": true},
		{"name": "client", "version": "1", "icons": map[string]any{}},
		{"name": "client", "version": "1", "icons": []any{map[string]any{"theme": "dark"}}},
		{"name": "client", "version": "1", "icons": []any{map[string]any{"src": "x", "theme": "auto"}}},
	}
	for _, info := range invalid {
		if _, err := DecodeRequestMeta(metaParams(map[string]any{}, info)); err == nil || err.Code != CodeInvalidParams {
			t.Fatalf("invalid client info accepted: %+v", info)
		}
	}
}

func metaParams(capabilities map[string]any, info map[string]any) map[string]any {
	meta := map[string]any{
		"io.modelcontextprotocol/protocolVersion":    Version,
		"io.modelcontextprotocol/clientCapabilities": capabilities,
	}
	if info != nil {
		meta["io.modelcontextprotocol/clientInfo"] = info
	}
	return map[string]any{"_meta": meta}
}

func FuzzDecodeNeverPanics(f *testing.F) {
	f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}`))
	f.Add([]byte(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		message, _ := Decode(raw)
		if message.Params != nil {
			_, _ = DecodeRequestMeta(message.Params)
			_, _ = DecodeCursor(message.Params)
		}
	})
}
