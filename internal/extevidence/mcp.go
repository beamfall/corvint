package extevidence

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const mcpSourcePrefix = "\x00mcp\x00"
const maxMCPBytes = 8 << 20

var errMCP = errors.New("MCP session did not satisfy the bounded evidence profile")

// ParseMCP uses precisely the command argv admission; file paths cannot select it.
func ParseMCP(value string) (string, error) {
	source, err := ParseCommand(value)
	return strings.Replace(source, commandSourcePrefix, mcpSourcePrefix, 1), err
}

func loadMCP(ctx context.Context, root rootRepository, encoded string) provider {
	entry := provider{source: "mcp:" + encoded, state: StateUnavailable}
	var argv []string
	if json.Unmarshal([]byte(encoded), &argv) != nil {
		entry.reason = errMCP.Error()
		return entry
	}
	var data []byte
	dir, err := filepath.Abs(root.dir)
	if err != nil {
		entry.reason = errMCP.Error()
		return entry
	}
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv: argv, Dir: filepath.Clean(dir), Env: commandEnvironment(),
		Timeout: commandTimeout, OutputLimit: maxMCPBytes, StderrLimit: maxCommandStderrBytes,
		Dialogue: func(reader io.Reader, writer io.WriteCloser) error {
			var err error
			data, err = mcpExchange(reader, writer)
			return err
		},
	})
	if _, reason := commandFailure(observation); reason != "" {
		entry.reason = errMCP.Error()
		return entry
	}
	return decodeRecord(ctx, root, entry, data)
}

func mcpExchange(reader io.Reader, writer io.WriteCloser) ([]byte, error) {
	scanner := bufio.NewScanner(io.LimitReader(reader, maxMCPBytes+1))
	scanner.Buffer(make([]byte, 4096), maxMCPBytes)
	if _, err := io.WriteString(writer, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"corvint-evidence","version":"1"}}}`+"\n"); err != nil {
		return nil, errMCP
	}
	initial, err := mcpResponse(scanner, "1")
	if err != nil {
		return nil, err
	}
	var info struct {
		ProtocolVersion string                     `json:"protocolVersion"`
		Capabilities    map[string]json.RawMessage `json:"capabilities"`
		ServerInfo      json.RawMessage            `json:"serverInfo"`
		Instructions    string                     `json:"instructions,omitempty"`
	}
	if strictMCP(initial, &info) != nil {
		return nil, errMCP
	}
	if info.ProtocolVersion != "2025-11-25" {
		return nil, errMCP
	}
	if !bytes.HasPrefix(bytes.TrimSpace(info.Capabilities["tools"]), []byte("{")) {
		return nil, errMCP
	}
	if !bytes.HasPrefix(bytes.TrimSpace(info.ServerInfo), []byte("{")) {
		return nil, errMCP
	}
	server, err := mcpObject(info.ServerInfo, "name", "version", "title")
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"name", "version"} {
		var value string
		if json.Unmarshal(server[key], &value) != nil || value == "" {
			return nil, errMCP
		}
	}
	if _, err := io.WriteString(writer, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"+`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"corvint_evidence","arguments":{}}}`+"\n"); err != nil {
		return nil, errMCP
	}
	result, err := mcpResponse(scanner, "2")
	if err != nil {
		return nil, err
	}
	fields, err := mcpObject(result, "content", "isError")
	if err != nil {
		return nil, err
	}
	if value, present := fields["isError"]; present && string(value) != "false" {
		return nil, errMCP
	}
	var blocks []json.RawMessage
	if json.Unmarshal(fields["content"], &blocks) != nil || len(blocks) != 1 {
		return nil, errMCP
	}
	if _, err := mcpObject(blocks[0], "type", "text"); err != nil {
		return nil, err
	}
	var call struct {
		Content []struct {
			Type string  `json:"type"`
			Text *string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError,omitempty"`
	}
	if strictMCP(result, &call) != nil {
		return nil, errMCP
	}
	if call.IsError || len(call.Content) != 1 {
		return nil, errMCP
	}
	block := call.Content[0]
	if block.Type != "text" || block.Text == nil {
		return nil, errMCP
	}
	data := []byte(*block.Text)
	if len(data) > MaxRecordBytes {
		return nil, errMCP
	}
	if err := writer.Close(); err != nil {
		return nil, errMCP
	}
	if scanner.Scan() || scanner.Err() != nil {
		return nil, errMCP
	}
	return data, nil
}

func mcpResponse(scanner *bufio.Scanner, id string) (json.RawMessage, error) {
	if !scanner.Scan() {
		return nil, errMCP
	}
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	if strictMCP(scanner.Bytes(), &response) != nil {
		return nil, errMCP
	}
	if response.JSONRPC != "2.0" || string(response.ID) != id {
		return nil, errMCP
	}
	if !bytes.HasPrefix(bytes.TrimSpace(response.Result), []byte("{")) {
		return nil, errMCP
	}
	return response.Result, nil
}

func mcpObject(data []byte, allowed ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if strictMCP(data, &fields) != nil || fields == nil {
		return nil, errMCP
	}
	for key := range fields {
		found := false
		for _, name := range allowed {
			found = found || name == key
		}
		if !found {
			return nil, errMCP
		}
	}
	return fields, nil
}

func strictMCP(data []byte, target any) error {
	if !utf8.Valid(data) {
		return errMCP
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if uniqueMCPValue(decoder, 0) != nil {
		return errMCP
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errMCP
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return errMCP
	}
	return nil
}

func uniqueMCPValue(decoder *json.Decoder, depth int) error {
	if depth > 32 {
		return errMCP
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delim == '{' {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errMCP
			}
			seen[name] = true
		}
		if uniqueMCPValue(decoder, depth+1) != nil {
			return errMCP
		}
	}
	_, err = decoder.Token()
	return err
}
