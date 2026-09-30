// SPDX-License-Identifier: AGPL-3.0-or-later
// Package lspstdio implements the capability-free experimental editor transport.
package lspstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxBody = 1 << 20
const maxHeader = 8 << 10
const maxFrames = 100000

var ErrExit = errors.New("exit before shutdown")
var ErrFraming = errors.New("invalid or truncated LSP frame")
var ErrLimit = errors.New("session frame limit reached")

type negotiation struct {
	roots    []string
	encoding string
}

// Serve owns in and out and closes both before returning. Close must unblock any
// pending Read/Write; this is true of the command's owned standard streams.
// There are no workers, providers, filesystem accesses, or outstanding requests.
func Serve(ctx context.Context, in io.ReadCloser, out io.WriteCloser) error {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-ctx.Done():
			in.Close()
			out.Close()
		case <-done:
		}
	}()
	defer func() { close(done); in.Close(); out.Close(); <-stopped }()
	reader := bufio.NewReaderSize(in, maxHeader+1)
	state := 0 // new, awaiting initialized, ready, shutdown
	var session negotiation
	for n := 0; n < maxFrames; n++ {
		body, err := readFrame(reader)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var msg map[string]json.RawMessage
		if !utf8.Valid(body) || !json.Valid(body) {
			if err = reply(out, nil, nil, -32700, "Parse error"); err != nil {
				return err
			}
			continue
		}
		if err = json.Unmarshal(body, &msg); err != nil || msg == nil {
			if err = reply(out, nil, nil, -32600, "Invalid Request"); err != nil {
				return err
			}
			continue
		}
		id, hasID := msg["id"]
		var method, version string
		json.Unmarshal(msg["method"], &method)
		json.Unmarshal(msg["jsonrpc"], &version)
		if version != "2.0" || method == "" || (hasID && !validID(id)) {
			// Client responses are irrelevant: the server never sends requests.
			if version == "2.0" && msg["method"] == nil && (msg["result"] != nil || msg["error"] != nil) {
				continue
			}
			if err = reply(out, nil, nil, -32600, "Invalid Request"); err != nil {
				return err
			}
			continue
		}
		if method == "exit" && !hasID {
			if state == 3 {
				return nil
			}
			return ErrExit
		}
		if !hasID {
			if method == "initialized" && state == 1 {
				state = 2
			}
			// Unknown/completed $/cancelRequest IDs have no pending work to stop.
			continue
		}
		code, message := 0, ""
		var result any
		switch {
		case state == 3:
			code, message = -32600, "Invalid Request"
		case method == "initialize" && state == 0:
			config, e := initialize(msg["params"])
			if e != nil {
				code, message = -32602, "Invalid params"
			} else {
				state = 1
				session = config
				result = map[string]any{"capabilities": map[string]any{"positionEncoding": session.encoding}, "serverInfo": map[string]any{"name": "corvint-lsp-experimental", "version": "0"}}
			}
		case state == 0:
			code, message = -32002, "Server not initialized"
		case method == "initialize":
			code, message = -32600, "Invalid Request"
		case method == "shutdown":
			state = 3
		case state == 1:
			code, message = -32002, "Server not initialized"
		default:
			code, message = -32601, "Method not found"
		}
		if err = reply(out, id, result, code, message); err != nil {
			return err
		}
	}
	return ErrLimit
}

func validID(id json.RawMessage) bool {
	var s string
	if json.Unmarshal(id, &s) == nil && !bytes.Equal(id, []byte("null")) {
		return true
	}
	var n int32
	return json.Unmarshal(id, &n) == nil && !bytes.Equal(id, []byte("null"))
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	total, length := 0, -1
	for {
		line, err := r.ReadSlice('\n')
		if err == io.EOF && total == 0 && len(line) == 0 {
			return nil, io.EOF
		}
		total += len(line)
		if err != nil || total > maxHeader || !bytes.HasSuffix(line, []byte("\r\n")) {
			return nil, ErrFraming
		}
		line = line[:len(line)-2]
		if len(line) == 0 {
			break
		}
		k, v, ok := strings.Cut(string(line), ":")
		if !ok {
			return nil, ErrFraming
		}
		if strings.EqualFold(k, "Content-Length") {
			v = strings.TrimSpace(v)
			if length >= 0 || v == "" || strings.IndexFunc(v, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
				return nil, ErrFraming
			}
			length, err = strconv.Atoi(v)
			if err != nil || length > MaxBody {
				return nil, ErrFraming
			}
		}
		for _, ch := range line {
			if ch > 127 {
				return nil, ErrFraming
			}
		}
		if strings.EqualFold(k, "Content-Type") {
			media, params, e := mime.ParseMediaType(strings.TrimSpace(v))
			encoding := strings.ToLower(params["charset"])
			if e != nil || media != "application/vscode-jsonrpc" || (encoding != "" && encoding != "utf-8" && encoding != "utf8") {
				return nil, ErrFraming
			}
		}
	}
	if length < 0 {
		return nil, ErrFraming
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, ErrFraming
	}
	return body, nil
}

func reply(w io.Writer, id json.RawMessage, result any, code int, message string) error {
	if id == nil {
		id = json.RawMessage("null")
	}
	envelope := map[string]any{"jsonrpc": "2.0", "id": id}
	if code != 0 {
		envelope["error"] = map[string]any{"code": code, "message": message}
	} else {
		envelope["result"] = result
	}
	b, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(b), b)
	return err
}

func initialize(raw json.RawMessage) (negotiation, error) {
	config := negotiation{encoding: "utf-16"}
	if len(raw) == 0 || raw[0] != '{' {
		return config, errors.New("params")
	}
	var p struct {
		RootURI          *string `json:"rootUri"`
		WorkspaceFolders []struct {
			URI string `json:"uri"`
		} `json:"workspaceFolders"`
		Capabilities struct {
			General struct {
				PositionEncodings []string `json:"positionEncodings"`
			} `json:"general"`
		} `json:"capabilities"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return config, errors.New("params")
	}
	for _, encoding := range p.Capabilities.General.PositionEncodings {
		if encoding == "utf-8" || encoding == "utf-16" || encoding == "utf-32" {
			config.encoding = encoding
			break
		}
	}
	roots := []string{}
	if p.WorkspaceFolders != nil {
		for _, f := range p.WorkspaceFolders {
			roots = append(roots, f.URI)
		}
	} else if p.RootURI != nil {
		roots = append(roots, *p.RootURI)
	}
	if len(roots) > 64 {
		return config, errors.New("roots")
	}
	seen := map[string]bool{}
	for _, root := range roots {
		u, e := url.Parse(root)
		if e != nil || u.Scheme != "file" || u.Host != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/") || strings.ContainsRune(u.Path, 0) {
			return config, errors.New("root")
		}
		if seen[root] {
			return config, errors.New("duplicate root")
		}
		seen[root] = true
		config.roots = append(config.roots, root)
	}
	return config, nil
}
