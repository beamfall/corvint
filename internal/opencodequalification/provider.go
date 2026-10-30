package opencodequalification

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

var suppliedHandle = regexp.MustCompile(`cv1:[0-9a-f]+:[0-9a-f]+:all:add.go`)

// sourceTask is the scripted corvint_context task whose packet ranks the seeded add.go source; its
// tool result is where the exact expansion handle comes from (AHI-213).
const sourceTask = "Locate Add in add.go."

// framePrompt is the campaign prompt (AHI-218). The AHI-215 prompt hook withholds its frame for a
// prompt that names an existing path or whose rows carry only syntax authority, which sourceTask
// does, so the campaign prompts with a task the fixture's AGENTS.md governs: the frame is emitted
// and the receipt-linked delivery checks exercise it.
const framePrompt = "Identify the active work queue and required workflow gates"

// reasonNoExpansionHandle is the typed provider fault when no corvint_context tool result in the
// request carried a cv1 handle for add.go; the scripted provider then skips corvint_expand rather
// than sending a placeholder handle (AHI-213).
const reasonNoExpansionHandle = "context-packet-carried-no-expansion-handle"

type scriptedCall struct {
	name string
	args Object
	// expand marks the corvint_expand call, which needs a handle supplied by an earlier tool result.
	expand bool
	// compact attaches the synthetic over-budget usage that triggers OpenCode compaction.
	compact bool
}

type provider struct {
	mu                          sync.Mutex
	requests, next              int
	root                        string
	benchmark, smoke, interrupt bool
	ctx                         context.Context
	handlers                    sync.WaitGroup
}

func startProvider(ctx context.Context, root string, benchmark, smoke, interrupt bool) (string, func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	p := &provider{root: root, benchmark: benchmark, smoke: smoke, interrupt: interrupt, ctx: ctx}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		cancel()
		return "", nil, e
	}
	s := &http.Server{Handler: p, ReadHeaderTimeout: 5 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	done := make(chan struct{})
	go func() { defer close(done); _ = s.Serve(listener) }()
	return "http://" + listener.Addr().String() + "/v1", func() { cancel(); _ = s.Close(); <-done; p.handlers.Wait() }, nil
}
func (p *provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.handlers.Add(1)
	defer p.handlers.Done()
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	data, e := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if e != nil {
		http.Error(w, "read", 400)
		return
	}
	var request Object
	if e = json.Unmarshal(data, &request); e != nil {
		http.Error(w, "json", 400)
		return
	}
	p.mu.Lock()
	step := p.requests
	p.requests++
	e = appendJSON(p.root+"/requests.jsonl", request)
	p.mu.Unlock()
	if e != nil {
		http.Error(w, "capture", 500)
		return
	}
	if p.interrupt {
		select {
		case <-p.ctx.Done():
		case <-r.Context().Done():
		}
		return
	}
	supplied := suppliedHandle.FindString(string(data))
	calls := scriptedCalls(strings.ReplaceAll(supplied, ":all:", ":1-3:"), p.benchmark)
	p.mu.Lock()
	index := p.next
	if p.smoke {
		index = len(calls)
	}
	if index < len(calls) && calls[index].expand && supplied == "" {
		// AHI-213: no corvint_context result carried a handle; record the typed reason and go on
		// without corvint_expand, so native-exact-expansion fails with that reason.
		e = appendJSON(p.root+"/provider-faults.jsonl", Object{"check": "native-exact-expansion", "reason": reasonNoExpansionHandle, "request": step})
		index++
	}
	p.next = index + 1
	p.mu.Unlock()
	if e != nil {
		http.Error(w, "capture", 500)
		return
	}
	literal := func(v any) string { b, _ := jsonBytes(v); return string(b) }
	delta := Object{"role": "assistant", "content": "QUALIFICATION_DONE"}
	reason := "stop"
	compact := false
	if index < len(calls) {
		call := calls[index]
		delta = Object{"role": "assistant", "tool_calls": []Object{{"index": 0, "id": fmt.Sprintf("call_%d", step), "type": "function", "function": Object{"name": call.name, "arguments": literal(call.args)}}}}
		reason = "tool_calls"
		compact = call.compact
	}
	w.Header().Set("Content-Type", "text/event-stream")
	for i := 0; i < 2; i++ {
		var finish any
		if i == 1 {
			delta = Object{}
			finish = reason
		}
		chunk := Object{"id": fmt.Sprintf("local-%d", step), "object": "chat.completion.chunk", "created": 1, "model": "probe", "choices": []Object{{"index": 0, "delta": delta, "finish_reason": finish}}}
		if compact && i == 1 {
			chunk["usage"] = Object{"prompt_tokens": 130000, "completion_tokens": 5000, "total_tokens": 135000}
		}
		fmt.Fprintf(w, "data: %s\n\n", literal(chunk))
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}

// scriptedCalls is the provider's fixed tool-call script. corvint_context is a direct tool
// (AHI-211); the rest stay in the Code Mode catalog behind execute. The first query exercises
// governance; the second is the source query whose packet supplies the add.go expansion handle
// (AHI-213), which the expand call narrows to lines 1-3. handle is empty when none was supplied.
func scriptedCalls(handle string, benchmark bool) []scriptedCall {
	literal := func(v any) string { b, _ := jsonBytes(v); return string(b) }
	calls := []scriptedCall{
		{name: "corvint_context", args: Object{"task": framePrompt}},
		{name: "corvint_context", args: Object{"task": sourceTask}},
		{name: "execute", args: Object{"code": "return await tools.corvint_expand(" + literal(Object{"handle": handle}) + ")"}, expand: true},
		{name: "edit", args: Object{"path": "add.go", "oldString": "// before", "newString": "// after"}},
		{name: "execute", args: Object{"code": "return await tools.qualification_verify({})"}},
		{name: "execute", args: Object{"code": "return await tools.corvint_record_outcome(" + literal(Object{"task": sourceTask, "changedPaths": []string{"add.go"}, "verification": []Object{{"commandSha256": hash([]byte("git diff --check")), "status": "passed"}}, "outcome": "passed"}) + ")"}, compact: true},
		{name: "execute", args: Object{"code": "await tools.qualification_prompts({}); for(let i=0;i<21;i++)await tools.qualification_noop({}); return \"native samples complete\""}},
	}
	if benchmark {
		return calls[len(calls)-1:]
	}
	return calls[:len(calls)-1]
}
