// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

type output struct{ bytes.Buffer }

func (*output) Close() error   { return nil }
func frame(body string) string { return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body) }
func transcript(t *testing.T, messages ...string) ([]map[string]any, error) {
	t.Helper()
	var input strings.Builder
	for _, m := range messages {
		input.WriteString(frame(m))
	}
	out := &output{}
	err := Serve(context.Background(), io.NopCloser(strings.NewReader(input.String())), out)
	var responses []map[string]any
	r := bufio.NewReader(&out.Buffer)
	for {
		b, e := readFrame(r)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		var m map[string]any
		if e = json.Unmarshal(b, &m); e != nil {
			t.Fatal(e)
		}
		responses = append(responses, m)
	}
	return responses, err
}

const initMessage = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":"file:///tmp/work","capabilities":{}}}`
const initialized = `{"jsonrpc":"2.0","method":"initialized","params":{}}`
const shutdown = `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`
const exit = `{"jsonrpc":"2.0","method":"exit"}`

func errorCode(m map[string]any) float64 { return m["error"].(map[string]any)["code"].(float64) }

func TestLifecycle(t *testing.T) {
	r, err := transcript(t, `{"jsonrpc":"2.0","id":0,"method":"hover"}`, initMessage, initialized, `{"jsonrpc":"2.0","id":"x","method":"textDocument/hover"}`, shutdown, `{"jsonrpc":"2.0","id":3,"method":"shutdown"}`, exit)
	if err != nil || len(r) != 5 {
		t.Fatalf("%v %v", r, err)
	}
	if errorCode(r[0]) != -32002 || errorCode(r[2]) != -32601 || errorCode(r[4]) != -32600 {
		t.Fatal(r)
	}
	caps := r[1]["result"].(map[string]any)["capabilities"].(map[string]any)
	if len(caps) != 1 || caps["positionEncoding"] != "utf-16" {
		t.Fatal(caps)
	}
	if r[3]["result"] != nil {
		t.Fatal(r[3])
	}
	_, err = transcript(t, exit)
	if !errors.Is(err, ErrExit) {
		t.Fatal(err)
	}
	r, err = transcript(t, initMessage, initMessage, initialized, shutdown, exit)
	if err != nil || errorCode(r[1]) != -32600 {
		t.Fatal(r, err)
	}
}

func TestInitializeValidation(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"rootUri":"https://host/x"}`, `{"rootUri":"file://host/x"}`, `{"rootUri":"file:///x%00"}`, `{"workspaceFolders":[{"uri":"file:///a"},{"uri":"file:///a"}]}`, `{"capabilities":{"general":{"positionEncodings":3}}}`} {
		if _, err := initialize(json.RawMessage(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	n, err := initialize(json.RawMessage(`{"rootUri":"file:///ignored","workspaceFolders":[{"uri":"file:///a"},{"uri":"file:///b"}],"capabilities":{"general":{"positionEncodings":["unknown","utf-8","utf-16"]}}}`))
	if err != nil || n.encoding != "utf-8" || strings.Join(n.roots, ",") != "file:///a,file:///b" {
		t.Fatal(n, err)
	}
	r, err := transcript(t, `{"jsonrpc":"2.0","id":7,"method":"initialize","params":null}`, initMessage, initialized, shutdown, exit)
	if err != nil || errorCode(r[0]) != -32602 || r[1]["result"] == nil {
		t.Fatal(r, err)
	}
}

func TestFraming(t *testing.T) {
	for _, s := range []string{"Content-Length: 2\n\n{}", "Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}", "Content-Length: -1\r\n\r\n", "Content-Length: 1048577\r\n\r\n", "Content-Length: 2\r\n\r\n{", "Content-Length: 2\r\n", "X: " + strings.Repeat("a", 8192) + "\r\n\r\n", "Content-Type: text/plain\r\nContent-Length: 2\r\n\r\n{}"} {
		if _, err := readFrame(bufio.NewReader(strings.NewReader(s))); !errors.Is(err, ErrFraming) {
			t.Errorf("bad framing accepted: %v", err)
		}
	}
	b, err := readFrame(bufio.NewReader(strings.NewReader(frame(`{"unicode":"λ😀"}`))))
	if err != nil || string(b) != `{"unicode":"λ😀"}` {
		t.Fatal(string(b), err)
	}
}

func TestInvalidMessages(t *testing.T) {
	r, err := transcript(t, `{`, `[]`, `null`, `{"jsonrpc":"1.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":null,"method":"initialize"}`, initMessage, initialized, `{"jsonrpc":"2.0","method":"unknown"}`, shutdown, exit)
	if err != nil || len(r) != 7 {
		t.Fatal(r, err)
	}
	for i, code := range []float64{-32700, -32600, -32600, -32600, -32600} {
		if errorCode(r[i]) != code {
			t.Fatal(r)
		}
	}
}

func TestCancellationAndEOF(t *testing.T) {
	r, err := transcript(t, initMessage, initialized, `{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":1}}`, `{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":"unknown"}}`, shutdown, exit)
	if err != nil || len(r) != 2 {
		t.Fatal(r, err)
	}
	for _, prefix := range []string{"", "Content-Length: 100\r\n\r\n{"} {
		in, feed := io.Pipe()
		out := &output{}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- Serve(ctx, in, out) }()
		if prefix != "" {
			if _, err := io.WriteString(feed, prefix); err != nil {
				t.Fatal(err)
			}
		}
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("cancellation leaked reader")
		}
		feed.Close()
	}
	if _, err := transcript(t); err != nil {
		t.Fatal(err)
	}
}

func TestCancellationBlockedOutput(t *testing.T) {
	out, writer := io.Pipe()
	defer out.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, io.NopCloser(strings.NewReader(frame(initMessage))), writer) }()
	// Read only one byte so the response writer remains blocked.
	b := make([]byte, 1)
	if _, err := out.Read(b); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation leaked writer")
	}
}
