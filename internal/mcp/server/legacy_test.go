package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

const legacyInitialize = `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{"roots":{}},"clientInfo":{"name":"opencode","version":"1.18.31"}}}`
const legacyInitializedNotification = `{"jsonrpc":"2.0","method":"notifications/initialized"}`

func legacyServer(t *testing.T, handler Handler) *Server {
	t.Helper()
	instance, err := New(Config{Name: "corvint", Version: "test", ProtocolVersion: protocol.LegacyVersion, Handler: handler,
		Capabilities: map[string]any{"tools": map[string]any{"listChanged": false}, "logging": map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	return instance
}

func legacyExchange(t *testing.T, instance *Server, frames ...string) map[float64]map[string]any {
	t.Helper()
	var output bytes.Buffer
	if err := instance.Serve(context.Background(), strings.NewReader(strings.Join(frames, "\n")+"\n"), &output); err != nil {
		t.Fatal(err)
	}
	results := map[float64]map[string]any{}
	decoder := json.NewDecoder(&output)
	for {
		var response map[string]any
		err := decoder.Decode(&response)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		id, ok := response["id"].(float64)
		if !ok {
			t.Fatalf("unexpected response: %#v", response)
		}
		if results[id] != nil {
			t.Fatalf("duplicate response id: %v", id)
		}
		results[id] = response
	}
	return results
}

func TestMCPV0022LegacyAdmissionAndReceipt(t *testing.T) {
	t.Run("MCPV0-022 ordered initialization and receipt", func(t *testing.T) {
		receipt := map[string]any{"resultType": "application", "ttlMs": 17, "cacheScope": "receipt", "state": "UNKNOWN"}
		instance := legacyServer(t, HandlerFunc(func(_ context.Context, request protocol.Request, _ Notifier) (map[string]any, *protocol.RPCError) {
			if request.Meta.ProtocolVersion != protocol.LegacyVersion || request.Meta.ClientInfo["name"] != "opencode" || request.Params == nil {
				t.Errorf("request=%#v", request)
			}
			return map[string]any{"structuredContent": receipt, "content": []any{}, "cacheScope": "private", "ttlMs": 0, "resultType": "complete"}, nil
		}))
		results := legacyExchange(t, instance,
			legacyInitializedNotification,
			`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
			`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
			legacyInitialize,
			`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
			`{"jsonrpc":"2.0","method":"notifications/initialized","params":{"bad":true}}`,
			`{"jsonrpc":"2.0","id":4,"method":"tools/list"}`,
			legacyInitializedNotification, legacyInitializedNotification,
			`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"receipt","arguments":{}}}`,
			strings.Replace(legacyInitialize, `"id":0`, `"id":6`, 1),
			`{"jsonrpc":"2.0","id":7,"method":"tools/list"}`,
			`{"jsonrpc":"2.0","id":8,"method":"server/discover"}`,
			`{"jsonrpc":"2.0","id":9,"method":"tools/list","params":{`+requestMeta+`}}`,
			`{"jsonrpc":"2.0","id":10,"method":"ping","params":{}}`,
		)
		if len(results) != 11 {
			t.Fatalf("responses=%#v", results)
		}
		for _, id := range []float64{1, 3, 4, 6} {
			assertErrorCode(t, results[id], protocol.CodeInvalidRequest)
		}
		assertErrorCode(t, results[8], protocol.CodeMethodNotFound)
		assertErrorCode(t, results[9], protocol.CodeInvalidParams)
		init := results[0]["result"].(map[string]any)
		if len(init) != 3 || init["protocolVersion"] != protocol.LegacyVersion || len(init["capabilities"].(map[string]any)) != 1 {
			t.Fatalf("initialize=%#v", init)
		}
		for _, id := range []float64{2, 10} {
			if !reflect.DeepEqual(results[id]["result"], map[string]any{}) {
				t.Fatalf("ping=%#v", results[id])
			}
		}
		for _, id := range []float64{5, 7} {
			result := results[id]["result"].(map[string]any)
			if len(result) != 2 || result["structuredContent"].(map[string]any)["resultType"] != "application" || result["structuredContent"].(map[string]any)["state"] != "UNKNOWN" {
				t.Fatalf("receipt=%#v", result)
			}
		}
		// Reusing a Server never reuses a connection's initialization authority.
		assertErrorCode(t, legacyExchange(t, instance, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)[1], protocol.CodeInvalidRequest)
	})
}

func TestMCPV0022LegacyFailedInitializeCannotAdmitTools(t *testing.T) {
	instance := legacyServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		t.Error("unexpected handler")
		return nil, nil
	}))
	for _, invalid := range []string{
		strings.Replace(legacyInitialize, `"protocolVersion":"2025-11-25"`, `"protocolVersion":null`, 1),
		strings.Replace(legacyInitialize, `"roots":{}`, `"roots":{"listChanged":"yes"}`, 1),
		strings.Replace(legacyInitialize, `"clientInfo":{`, `"clientInfo":null,"other":{`, 1),
	} {
		results := legacyExchange(t, instance, invalid, legacyInitializedNotification, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		assertErrorCode(t, results[0], protocol.CodeInvalidParams)
		assertErrorCode(t, results[1], protocol.CodeInvalidRequest)
	}
	// A valid different offer receives the server's sole supported version.
	negotiated := legacyExchange(t, instance, strings.Replace(legacyInitialize, protocol.LegacyVersion, "2024-11-05", 1))[0]
	if negotiated["result"].(map[string]any)["protocolVersion"] != protocol.LegacyVersion {
		t.Fatal(negotiated)
	}
	// Encoding failure emits an error, never a successful lifecycle transition.
	instance.config.Instructions = strings.Repeat("x", protocol.MaxMessageBytes)
	results := legacyExchange(t, instance, legacyInitialize, legacyInitializedNotification, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	assertErrorCode(t, results[0], protocol.CodeInternalError)
	assertErrorCode(t, results[1], protocol.CodeInvalidRequest)
}

func TestMCPV0022LegacyCancellationAndProgress(t *testing.T) {
	input, sender := io.Pipe()
	output, writer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	called, stopped := make(chan struct{}), make(chan struct{})
	instance := legacyServer(t, HandlerFunc(func(ctx context.Context, request protocol.Request, notifier Notifier) (map[string]any, *protocol.RPCError) {
		if err := notifier.Progress(1, nil, "working"); err != nil {
			t.Error(err)
		}
		close(called)
		<-ctx.Done()
		close(stopped)
		return map[string]any{"content": []any{}}, nil
	}))
	done := make(chan error, 1)
	go func() { done <- instance.Serve(ctx, input, writer) }()
	t.Cleanup(func() {
		cancel()
		_ = sender.Close()
		_ = output.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("server survived cancellation")
		}
	})
	decoder := json.NewDecoder(output)
	send := func(frame string) {
		t.Helper()
		if _, err := io.WriteString(sender, frame+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	receive := func() map[string]any {
		t.Helper()
		var value map[string]any
		if err := decoder.Decode(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	send(legacyInitialize)
	receive()
	send(legacyInitializedNotification)
	send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"_meta":{"progressToken":"p"}}}`)
	progress := receive()
	if progress["method"] != "notifications/progress" || progress["params"].(map[string]any)["progressToken"] != "p" {
		t.Fatal(progress)
	}
	<-called
	send(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}`)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("handler survived cancellation")
	}
	send(`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if response := receive(); response["id"] != float64(2) {
		t.Fatalf("cancelled call responded: %#v", response)
	}
}

func TestMCPV0022LegacyBlockedInitializeCancels(t *testing.T) {
	instance := legacyServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		t.Error("unexpected handler")
		return nil, nil
	}))
	output, writer := io.Pipe()
	defer output.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- instance.Serve(ctx, strings.NewReader(legacyInitialize+"\n"+legacyInitializedNotification+"\n"), writer)
	}()
	// Reading one byte proves the initialization write started but cannot finish.
	one := make([]byte, 1)
	if _, err := output.Read(one); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled write succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("initialize write survived cancellation")
	}
}
