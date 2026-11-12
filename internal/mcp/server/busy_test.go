package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

// DMF-V1-010: a request that arrives at the concurrency limit is never -32603: a tools/call gets
// the server-busy tool error and any other request the server-busy JSON-RPC error, each with a
// message naming the limit, retryable true and a next step.
func TestDMFV1010ServerBusy(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	server, err := New(Config{
		Name: "corvint", Version: "test", MaxConcurrent: 1, DiscoveryTTL: time.Minute,
		Capabilities: map[string]any{"tools": map[string]any{}},
		Handler: HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
			close(started)
			<-release
			return map[string]any{"tools": []any{}}, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), serverInput, serverOutput) }()
	_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","id":"hold","method":"tools/list","params":{`+requestMeta+`}}`+"\n")
	<-started
	go func() {
		_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","id":"call","method":"tools/call","params":{`+requestMeta+`,"name":"corvint.status","arguments":{}}}`+"\n")
		_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","id":"list","method":"tools/list","params":{`+requestMeta+`}}`+"\n")
	}()
	reader := bufio.NewReader(clientOutput)
	responses := map[string]map[string]any{}
	for len(responses) < 2 {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatal(err)
		}
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatal(err)
		}
		id, _ := response["id"].(string)
		responses[id] = response
	}
	close(release)
	_ = clientInput.Close()
	go func() { _, _ = io.Copy(io.Discard, clientOutput) }()
	<-done
	check := func(where string, detail map[string]any) {
		t.Helper()
		message, _ := detail["message"].(string)
		if detail["code"] != protocol.ServerBusyCode || detail["retryable"] != true || !strings.Contains(message, "concurrency limit") || !strings.Contains(message, "1") || detail["next"] == "" || detail["next"] == nil {
			t.Fatalf("%s busy detail %#v", where, detail)
		}
		// The busy object is outside the tool-group profiles: exactly these members, no profile
		// or tool.
		members := slices.Sorted(maps.Keys(detail))
		if !slices.Equal(members, []string{"code", "message", "next", "retryable"}) {
			t.Fatalf("%s busy members %v", where, members)
		}
	}
	call := responses["call"]
	result, _ := call["result"].(map[string]any)
	if call["error"] != nil || result["isError"] != true {
		t.Fatalf("busy tools/call %#v", call)
	}
	check("tools/call", result["structuredContent"].(map[string]any))
	list := responses["list"]
	failure, _ := list["error"].(map[string]any)
	if failure["code"] != float64(protocol.CodeServerBusy) || failure["code"] == float64(protocol.CodeInternalError) {
		t.Fatalf("busy tools/list %#v", list)
	}
	check("tools/list", failure["data"].(map[string]any))
}
