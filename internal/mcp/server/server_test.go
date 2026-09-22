package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

const requestMeta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`

func newTestServer(t *testing.T, handler Handler) *Server {
	t.Helper()
	server, err := New(Config{
		Name: "corvint", Version: "test", Handler: handler,
		Capabilities: map[string]any{"tools": map[string]any{}},
		DiscoveryTTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func exchange(t *testing.T, server *Server, request string) map[string]any {
	t.Helper()
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), serverInput, serverOutput) }()
	if _, err := io.WriteString(clientInput, request+"\n"); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(clientOutput).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	_ = clientInput.Close()
	_ = serverOutput.Close()
	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "closed pipe") {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
	var response map[string]any
	if err := json.Unmarshal(line, &response); err != nil {
		t.Fatalf("decode %s: %v", line, err)
	}
	return response
}

func TestDiscoverIsStatelessAndExact(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		t.Fatal("discover reached handler")
		return nil, nil
	}))
	response := exchange(t, server, `{"jsonrpc":"2.0","id":"d","method":"server/discover","params":{`+requestMeta+`}}`)
	result := response["result"].(map[string]any)
	if result["resultType"] != "complete" || result["cacheScope"] != "public" || result["ttlMs"] != float64(60_000) {
		t.Fatalf("result=%+v", result)
	}
	versions := result["supportedVersions"].([]any)
	if len(versions) != 1 || versions[0] != protocol.Version {
		t.Fatalf("versions=%+v", versions)
	}
	capabilities := result["capabilities"].(map[string]any)
	if len(capabilities) != 1 || capabilities["tools"] == nil {
		t.Fatalf("capabilities=%+v", capabilities)
	}
}

func TestDiscoverRejectsEveryExtraParameter(t *testing.T) {
	called := false
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		called = true
		return nil, nil
	}))
	for index, extra := range []string{
		`"cursor":""`,
		`"arguments":{}`,
		`"clientRoot":"file:///untrusted"`,
	} {
		request := `{"jsonrpc":"2.0","id":` + string(rune('1'+index)) + `,"method":"server/discover","params":{` + requestMeta + `,` + extra + `}}`
		response := exchange(t, server, request)
		assertErrorCode(t, response, protocol.CodeInvalidParams)
		if response["id"] != float64(index+1) {
			t.Fatalf("id=%v want %d", response["id"], index+1)
		}
	}
	if called {
		t.Fatal("invalid discovery request reached handler")
	}
}

func TestPerRequestVersionAndMetadata(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		return map[string]any{"tools": []any{}, "cacheScope": "public", "ttlMs": 0}, nil
	}))
	missing := exchange(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	assertErrorCode(t, missing, protocol.CodeInvalidParams)
	unsupported := exchange(t, server, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"1900-01-01","io.modelcontextprotocol/clientCapabilities":{}}}}`)
	assertErrorCode(t, unsupported, protocol.CodeUnsupportedProtocolVersion)
	data := unsupported["error"].(map[string]any)["data"].(map[string]any)
	if data["requested"] != "1900-01-01" {
		t.Fatalf("data=%+v", data)
	}
}

// A legacy client sends initialize or ping without 2026-07-28 metadata, so the
// method-not-found answer MCPV0-006 promises must not depend on that metadata.
func TestLegacyLifecycleMethodsWithoutMetadataAreNotFound(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		t.Error("legacy request reached handler")
		return nil, protocol.MethodNotFound()
	}))
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"legacy","version":"0"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
	}
	for _, request := range requests {
		assertErrorCode(t, exchange(t, server, request), protocol.CodeMethodNotFound)
	}
}

func TestResultDecorationAndSanitizedErrors(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(_ context.Context, request protocol.Request, _ Notifier) (map[string]any, *protocol.RPCError) {
		if request.Method == "tools/list" {
			return map[string]any{"tools": []any{}, "cacheScope": "public", "ttlMs": 0}, nil
		}
		return nil, &protocol.RPCError{Code: protocol.CodeInvalidParams, Message: "secret argument body"}
	}))
	response := exchange(t, server, `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{`+requestMeta+`}}`)
	result := response["result"].(map[string]any)
	if result["resultType"] != "complete" || result["_meta"] == nil {
		t.Fatalf("result=%+v", result)
	}
	failed := exchange(t, server, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{`+requestMeta+`}}`)
	if message := failed["error"].(map[string]any)["message"]; message != "Invalid params" {
		t.Fatalf("message=%v", message)
	}
}

func TestCancellationReachesInFlightHandler(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server := newTestServer(t, HandlerFunc(func(ctx context.Context, _ protocol.Request, notifier Notifier) (map[string]any, *protocol.RPCError) {
		close(started)
		<-ctx.Done()
		_ = notifier.Progress(1, nil, "must not emit")
		close(cancelled)
		return nil, &protocol.RPCError{Code: protocol.CodeInvalidParams, Message: "must not emit"}
	}))
	serverInput, clientInput := io.Pipe()
	var output bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), serverInput, &output) }()
	_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","id":"work","method":"tools/call","params":{`+requestMeta+`}}`+"\n")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"work","reason":"unused"}}`+"\n")
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("handler was not cancelled")
	}
	_ = clientInput.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
	if output.Len() != 0 {
		t.Fatalf("cancelled request emitted %s", output.Bytes())
	}
}

func TestProgressAndLoggingAreOptIn(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(_ context.Context, _ protocol.Request, notifier Notifier) (map[string]any, *protocol.RPCError) {
		_ = notifier.Progress(1, nil, "done")
		_ = notifier.Log("error", "corvint", "hidden")
		return map[string]any{"content": []any{}}, nil
	}))
	request := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{},"progressToken":"p","io.modelcontextprotocol/logLevel":"debug"}}}`
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), serverInput, serverOutput) }()
	_, _ = io.WriteString(clientInput, request+"\n")
	reader := bufio.NewReader(clientOutput)
	first, _ := reader.ReadBytes('\n')
	second, _ := reader.ReadBytes('\n')
	_ = clientInput.Close()
	_ = serverOutput.Close()
	<-done
	combined := string(first) + string(second)
	if !strings.Contains(combined, `"method":"notifications/progress"`) || !strings.Contains(combined, `"result"`) {
		t.Fatalf("output=%s", combined)
	}
	if strings.Contains(combined, "hidden") {
		t.Fatalf("unadvertised logging emitted: %s", combined)
	}
}

func TestOversizedFrameDrainsAndContinues(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		return nil, protocol.MethodNotFound()
	}))
	input := strings.Repeat("x", protocol.MaxMessageBytes+1) + "\n" +
		`{"jsonrpc":"2.0","id":9,"method":"server/discover","params":{` + requestMeta + `}}` + "\n"
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), serverInput, serverOutput) }()
	go func() { _, _ = io.WriteString(clientInput, input) }()
	reader := bufio.NewReader(clientOutput)
	first, _ := reader.ReadBytes('\n')
	second, _ := reader.ReadBytes('\n')
	_ = clientInput.Close()
	_ = serverOutput.Close()
	<-done
	if !strings.Contains(string(first), `"code":-32700`) || !strings.Contains(string(second), `"result"`) {
		t.Fatalf("first=%s second=%s", first, second)
	}
}

func TestFrameLimitCountsTrailingCarriageReturn(t *testing.T) {
	exact := strings.NewReader(strings.Repeat("x", protocol.MaxMessageBytes-1) + "\r\n")
	frame, state, err := readFrame(bufio.NewReaderSize(exact, 64<<10))
	if err != nil || state != frameComplete || len(frame) != protocol.MaxMessageBytes || frame[len(frame)-1] != '\r' {
		t.Fatalf("exact state=%v bytes=%d err=%v", state, len(frame), err)
	}
	over := strings.NewReader(strings.Repeat("x", protocol.MaxMessageBytes) + "\r\n")
	frame, state, err = readFrame(bufio.NewReaderSize(over, 64<<10))
	if err != nil || state != frameOversized || frame != nil {
		t.Fatalf("over state=%v bytes=%d err=%v", state, len(frame), err)
	}
}

func TestCleanEOFFlushesAdmittedResponse(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		close(started)
		<-release
		return map[string]any{"tools": []any{}, "cacheScope": "public", "ttlMs": 0}, nil
	}))
	input := `{"jsonrpc":"2.0","id":11,"method":"tools/list","params":{` + requestMeta + `}}` + "\n"
	var output bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), strings.NewReader(input), &output) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	if output.Len() != 0 {
		t.Fatalf("response escaped before handler release: %s", output.Bytes())
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("clean EOF did not flush admitted response")
	}
	if !strings.Contains(output.String(), `"id":11`) || !strings.Contains(output.String(), `"result"`) {
		t.Fatalf("output=%s", output.Bytes())
	}
}

func TestContextCancellationInterruptsOpenStdin(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		t.Fatal("blocked stdin reached handler")
		return nil, nil
	}))
	serverInput, clientInput := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, serverInput, io.Discard) }()
	// Let the subprocess-equivalent OS file read enter the poller before
	// cancellation; cancelling before Read starts does not cover SIGINT.
	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		_ = clientInput.Close()
		<-done
		t.Fatal("context cancellation did not interrupt open stdin")
	}
	_ = clientInput.Close()
}

func TestContextCancellationInterruptsOpenOSPipeStdin(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		t.Fatal("blocked stdin reached handler")
		return nil, nil
	}))
	serverInput, clientInput, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverInput.Close() })
	t.Cleanup(func() { _ = clientInput.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, serverInput, io.Discard) }()
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		_ = clientInput.Close()
		<-done
		t.Fatal("context cancellation did not interrupt OS pipe stdin")
	}
}

func TestContextCancellationInterruptsBlockedOutput(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		return nil, protocol.MethodNotFound()
	}))
	clientOutput, serverOutput := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	input := `{"jsonrpc":"2.0","id":21,"method":"server/discover","params":{` + requestMeta + `}}` + "\n"
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, strings.NewReader(input), serverOutput) }()
	time.Sleep(25 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		_ = clientOutput.Close()
		<-done
		t.Fatal("context cancellation did not interrupt blocked output")
	}
	_ = clientOutput.Close()
}

// A response larger than the pipe buffer meets EAGAIN on the nonblocking
// descriptor; the retry must resume at the first unaccepted byte.
func TestBackpressuredOSPipeWriteDeliversExactBytes(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	payload := bytes.Repeat([]byte("0123456789abcdef"), 1<<16)
	received := make(chan []byte, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		data, _ := io.ReadAll(reader)
		received <- data
	}()
	written, writeErr := interruptibleWrite(context.Background(), writer, payload)
	_ = writer.Close()
	data := <-received
	if writeErr != nil || written != len(payload) || !bytes.Equal(data, payload) {
		t.Fatalf("written=%d err=%v received=%d bytes, exact=%v", written, writeErr, len(data), bytes.Equal(data, payload))
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestAsyncWriteFailureInterruptsOpenStdin(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		return nil, protocol.MethodNotFound()
	}))
	serverInput, clientInput := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), serverInput, failingWriter{}) }()
	_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","id":22,"method":"server/discover","params":{`+requestMeta+`}}`+"\n")
	select {
	case err := <-done:
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(250 * time.Millisecond):
		_ = clientInput.Close()
		<-done
		t.Fatal("write failure did not interrupt open stdin")
	}
	_ = clientInput.Close()
}

type blockedWriteCloser struct {
	started chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func newBlockedWriteCloser() *blockedWriteCloser {
	return &blockedWriteCloser{started: make(chan struct{}), closed: make(chan struct{})}
}

func (writer *blockedWriteCloser) Write([]byte) (int, error) {
	writer.once.Do(func() { close(writer.started) })
	<-writer.closed
	return 0, io.ErrClosedPipe
}

func (writer *blockedWriteCloser) Close() error {
	select {
	case <-writer.closed:
	default:
		close(writer.closed)
	}
	return nil
}

func TestRequestCancellationNeverWaitsBehindBlockedOutput(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		return nil, protocol.MethodNotFound()
	}))
	serverInput, clientInput := io.Pipe()
	output := newBlockedWriteCloser()
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), serverInput, output) }()
	_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","id":"blocked","method":"server/discover","params":{`+requestMeta+`}}`+"\n")
	select {
	case <-output.started:
	case <-time.After(time.Second):
		t.Fatal("response write did not start")
	}
	_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"blocked"}}`+"\n")
	readNext := make(chan error, 1)
	go func() {
		_, err := io.WriteString(clientInput, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"unknown"}}`+"\n")
		readNext <- err
	}()
	select {
	case err := <-readNext:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		_ = output.Close()
		_ = clientInput.Close()
		<-done
		t.Fatal("request cancellation waited behind blocked output")
	}
	// Transport cancellation still owns termination of a response that won
	// arbitration but cannot complete because the client does not read.
	_ = output.Close()
	_ = clientInput.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server did not stop after blocked transport closed")
	}
}

// A request cancelled while its progress frame is under backpressure must not
// abandon the frame midway: the next output would be appended to a torn line.
func TestCancelledProgressNeverTearsFrame(t *testing.T) {
	started := make(chan struct{})
	server := newTestServer(t, HandlerFunc(func(ctx context.Context, request protocol.Request, notifier Notifier) (map[string]any, *protocol.RPCError) {
		if request.Method == "server/discover" {
			return nil, nil
		}
		close(started)
		_ = notifier.Progress(1, nil, strings.Repeat("p", 512<<10))
		<-ctx.Done()
		return nil, nil
	}))
	serverInput, clientInput := io.Pipe()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(context.Background(), serverInput, writer)
		_ = writer.Close()
	}()
	progressMeta := strings.Replace(requestMeta, `clientCapabilities":{}`, `clientCapabilities":{},"progressToken":"work"`, 1)
	_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","id":"work","method":"tools/call","params":{`+progressMeta+`}}`+"\n")
	<-started
	time.Sleep(50 * time.Millisecond)
	_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"work"}}`+"\n")
	time.Sleep(50 * time.Millisecond)
	_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","id":2,"method":"server/discover","params":{`+requestMeta+`}}`+"\n")
	_ = clientInput.Close()
	data, _ := io.ReadAll(reader)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	for index, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("output line %d of %d is not one JSON value (%d bytes)", index, len(lines), len(line))
		}
	}
}

func TestReflectableRequestScalarsAreBounded(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		t.Fatal("hostile scalar reached handler")
		return nil, nil
	}))
	tests := []struct {
		name     string
		request  string
		code     int
		response map[string]any
	}{
		{
			name: "huge request id",
			request: `{"jsonrpc":"2.0","id":"` + strings.Repeat("i", protocol.MaxRequestIDBytes+1) +
				`","method":"server/discover","params":{` + requestMeta + `}}` + "\n",
			code: protocol.CodeInvalidRequest,
		},
		{
			name: "huge unsupported version",
			request: `{"jsonrpc":"2.0","id":23,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"` +
				strings.Repeat("v", protocol.MaxProtocolVersionBytes+1) +
				`","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n",
			code: protocol.CodeInvalidParams,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := server.Serve(context.Background(), strings.NewReader(test.request), &output); err != nil {
				t.Fatal(err)
			}
			if output.Len()-1 > protocol.MaxMessageBytes {
				t.Fatalf("response bytes=%d", output.Len()-1)
			}
			var response map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &response); err != nil {
				t.Fatalf("output=%q err=%v", output.Bytes(), err)
			}
			assertErrorCode(t, response, test.code)
			if test.code == protocol.CodeInvalidRequest && !reflect.DeepEqual(response["id"], nil) {
				t.Fatalf("hostile id reflected: %T", response["id"])
			}
		})
	}
}

func assertErrorCode(t *testing.T, response map[string]any, code int) {
	t.Helper()
	errorBody := response["error"].(map[string]any)
	if errorBody["code"] != float64(code) {
		t.Fatalf("error=%+v", errorBody)
	}
}

type signalWriter struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	marker string
	seen   chan struct{}
	once   sync.Once
}

func (writer *signalWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if strings.Contains(string(data), writer.marker) {
		writer.once.Do(func() { close(writer.seen) })
	}
	return writer.buffer.Write(data)
}

func (writer *signalWriter) String() string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.buffer.String()
}

// MCPV0-011: EOF after an unterminated tail reports one parse error and still drains admitted work.
func TestUnterminatedEOFFlushesAdmittedResponse(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := newTestServer(t, HandlerFunc(func(ctx context.Context, _ protocol.Request, _ Notifier) (map[string]any, *protocol.RPCError) {
		close(started)
		select {
		case <-release:
			return map[string]any{"tools": []any{}}, nil
		case <-ctx.Done():
			return nil, protocol.MethodNotFound()
		}
	}))
	input := `{"jsonrpc":"2.0","id":11,"method":"tools/list","params":{` + requestMeta + `}}` + "\n" + `{"jsonrpc":"2.0"`
	output := &signalWriter{marker: `"code":-32700`, seen: make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), strings.NewReader(input), output) }()
	for _, wait := range []chan struct{}{started, output.seen} {
		select {
		case <-wait:
		case <-time.After(time.Second):
			t.Fatalf("handler or parse error missing: %s", output.String())
		}
	}
	select {
	case <-done:
		t.Fatalf("unterminated EOF cancelled admitted request: %s", output.String())
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
	if !strings.Contains(output.String(), `"id":11`) || strings.Count(output.String(), "-32700") != 1 {
		t.Fatalf("output=%s", output.String())
	}
}

// MCPV0-004: a duplicate in-flight ID is rejected without an ID a client could correlate with the live request.
func TestDuplicateInFlightIDOmitsID(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		close(started)
		<-release
		return map[string]any{"tools": []any{}}, nil
	}))
	request := `{"jsonrpc":"2.0","id":"work","method":"tools/list","params":{` + requestMeta + `}}` + "\n"
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), serverInput, serverOutput) }()
	_, _ = io.WriteString(clientInput, request)
	<-started
	go func() { _, _ = io.WriteString(clientInput, request) }()
	reader := bufio.NewReader(clientOutput)
	var rejected, live map[string]any
	for _, target := range []*map[string]any{&rejected, &live} {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(line, target); err != nil {
			t.Fatal(err)
		}
		if target == &rejected {
			close(release)
		}
	}
	_ = clientInput.Close()
	go func() { _, _ = io.Copy(io.Discard, clientOutput) }()
	<-done
	assertErrorCode(t, rejected, protocol.CodeInvalidRequest)
	if _, present := rejected["id"]; present {
		t.Fatalf("duplicate rejection carries live id: %+v", rejected)
	}
	if live["id"] != "work" || live["result"] == nil {
		t.Fatalf("live=%+v", live)
	}
}

// MCPV0-004: a request leaves flight once its response wins the output race, so a
// client that reuses the ID after reading that response is never refused as a duplicate.
func TestIDReusableOnceResponseIsRead(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		return map[string]any{"tools": []any{}}, nil
	}))
	request := `{"jsonrpc":"2.0","id":"again","method":"tools/list","params":{` + requestMeta + `}}` + "\n"
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), serverInput, serverOutput) }()
	reader := bufio.NewReader(clientOutput)
	for round := range 200 {
		_, _ = io.WriteString(clientInput, request)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(line, []byte(`"error"`)) {
			t.Fatalf("round %d: reused ID after its response was refused: %s", round, line)
		}
	}
	_ = clientInput.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// MCPV0-004: an unencodable result is one sanitized -32603 for that request, not a session failure.
func TestUnencodableResultIsRequestError(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		return map[string]any{"bad": math.NaN()}, nil
	}))
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{` + requestMeta + `}}` + "\n"
	var output bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &response); err != nil {
		t.Fatalf("output=%q err=%v", output.Bytes(), err)
	}
	assertErrorCode(t, response, protocol.CodeInternalError)
	if response["id"] != float64(1) {
		t.Fatalf("response=%+v", response)
	}
}
