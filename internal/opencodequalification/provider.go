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

type provider struct {
	mu                          sync.Mutex
	step                        int
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
	step := p.step
	p.step++
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
	if p.smoke {
		step = 99
	}
	selected := "missing-supplied-handle"
	if v := suppliedHandle.FindString(string(data)); v != "" {
		selected = strings.ReplaceAll(v, ":all:", ":1-3:")
	}
	literal := func(v any) string { b, _ := jsonBytes(v); return string(b) }
	calls := []struct {
		name string
		args Object
	}{
		{"execute", Object{"code": "return await tools.corvint_context(" + literal(Object{"task": "Identify the active work queue and required workflow gates"}) + ")"}},
		{"execute", Object{"code": "return await tools.corvint_expand(" + literal(Object{"handle": selected}) + ")"}},
		{"edit", Object{"path": "add.go", "oldString": "// before", "newString": "// after"}},
		{"execute", Object{"code": "return await tools.qualification_verify({})"}},
		{"execute", Object{"code": "return await tools.corvint_record_outcome(" + literal(Object{"task": "Locate Add in add.go.", "changedPaths": []string{"add.go"}, "verification": []Object{{"commandSha256": hash([]byte("git diff --check")), "status": "passed"}}, "outcome": "passed"}) + ")"}},
		{"execute", Object{"code": "await tools.qualification_prompts({}); for(let i=0;i<21;i++)await tools.qualification_noop({}); return \"native samples complete\""}},
	}
	if p.benchmark {
		calls = calls[len(calls)-1:]
	} else {
		calls = calls[:len(calls)-1]
	}
	delta := Object{"role": "assistant", "content": "QUALIFICATION_DONE"}
	reason := "stop"
	if step < len(calls) {
		call := calls[step]
		delta = Object{"role": "assistant", "tool_calls": []Object{{"index": 0, "id": fmt.Sprintf("call_%d", step), "type": "function", "function": Object{"name": call.name, "arguments": literal(call.args)}}}}
		reason = "tool_calls"
	}
	w.Header().Set("Content-Type", "text/event-stream")
	for i := 0; i < 2; i++ {
		var finish any
		if i == 1 {
			delta = Object{}
			finish = reason
		}
		chunk := Object{"id": fmt.Sprintf("local-%d", step), "object": "chat.completion.chunk", "created": 1, "model": "probe", "choices": []Object{{"index": 0, "delta": delta, "finish_reason": finish}}}
		if step == 4 && i == 1 {
			chunk["usage"] = Object{"prompt_tokens": 130000, "completion_tokens": 5000, "total_tokens": 135000}
		}
		fmt.Fprintf(w, "data: %s\n\n", literal(chunk))
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}
