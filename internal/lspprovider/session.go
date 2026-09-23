package lspprovider

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// maxMessageBytes bounds one LSP message body the client will read; the
// process output as a whole is bounded by the process group (EEP-V0-024).
const maxMessageBytes = 4 << 20

var errSession = errors.New("language server session failed")

// session is a minimal JSON-RPC 2.0 client over LSP base-protocol framing.
// It answers every server-to-client request so the server never waits on
// the client, and it ignores notifications.
type session struct {
	reader *bufio.Reader
	writer io.Writer
	root   string
	next   int
}

type message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *session) send(value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = s.writer.Write(body)
	return err
}

func (s *session) notify(method string, params any) error {
	return s.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// call sends one request and reads until its response, answering server
// requests on the way. A response error is returned as an error.
func (s *session) call(method string, params any) (json.RawMessage, error) {
	s.next++
	id := strconv.Itoa(s.next)
	if err := s.send(map[string]any{"jsonrpc": "2.0", "id": s.next, "method": method, "params": params}); err != nil {
		return nil, err
	}
	for {
		incoming, err := s.read()
		if err != nil {
			return nil, err
		}
		if incoming.Method != "" && len(incoming.ID) != 0 {
			if err := s.answer(incoming); err != nil {
				return nil, err
			}
			continue
		}
		if incoming.Method != "" || string(incoming.ID) != id {
			continue
		}
		if incoming.Error != nil {
			return nil, fmt.Errorf("%s: server error %d: %s", method, incoming.Error.Code, incoming.Error.Message)
		}
		return incoming.Result, nil
	}
}

// answer replies to a server request: workspace/configuration gets one null
// per item, workspace/workspaceFolders the root folder, anything else null.
func (s *session) answer(request message) error {
	var result any
	switch request.Method {
	case "workspace/configuration":
		var params struct {
			Items []json.RawMessage `json:"items"`
		}
		_ = json.Unmarshal(request.Params, &params)
		result = make([]any, len(params.Items))
	case "workspace/workspaceFolders":
		result = []any{map[string]any{"uri": fileURI(s.root), "name": "root"}}
	}
	return s.send(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
}

func (s *session) read() (message, error) {
	length := -1
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return message{}, errSession
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if found && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return message{}, errSession
			}
		}
	}
	if length < 0 || length > maxMessageBytes {
		return message{}, errSession
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(s.reader, body); err != nil {
		return message{}, errSession
	}
	var decoded message
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&decoded); err != nil {
		return message{}, errSession
	}
	return decoded, nil
}
