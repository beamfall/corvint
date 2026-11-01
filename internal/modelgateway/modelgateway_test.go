package modelgateway

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func bashRequest(t *testing.T, output string, isError bool, tool string) []byte {
	t.Helper()
	request := map[string]any{
		"model":    "claude-opus-5-5",
		"stream":   true,
		"metadata": map[string]any{"user_id": "user_abc_session_123"},
		"messages": []any{
			map[string]any{"role": "user", "content": "run the tests <please>"},
			map[string]any{"role": "assistant", "content": []any{
				map[string]any{"type": "tool_use", "id": "toolu_1", "name": tool, "input": map[string]any{"command": "go test ./..."}},
			}},
			map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": output, "is_error": isError,
					"cache_control": map[string]any{"type": "ephemeral"}},
			}},
		},
		"max_tokens": 32000,
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func goTestOutput(passing int, tail string) string {
	var b strings.Builder
	for i := 0; i < passing; i++ {
		fmt.Fprintf(&b, "=== RUN   TestThing%d\n--- PASS: TestThing%d (0.00s)\n", i, i)
	}
	for i := 0; i < passing/4; i++ {
		fmt.Fprintf(&b, "ok  \tgithub.com/Beamfall/corvint/internal/pkg%d\t0.%03ds\n", i, i)
	}
	b.WriteString(tail)
	return b.String()
}

func toolResultText(t *testing.T, body []byte) string {
	t.Helper()
	var request struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	var blocks []map[string]any
	if err := json.Unmarshal(request.Messages[2].Content, &blocks); err != nil {
		t.Fatal(err)
	}
	return blocks[0]["content"].(string)
}

func TestLMGV0005FoldIsDeterministic(t *testing.T) {
	folder := NewFolder(FoldOptions{Archive: t.TempDir()})
	body := bashRequest(t, goTestOutput(400, "FAIL: nothing\n"), false, "Bash")
	first, n := folder.FoldRequest(body)
	if n != 1 {
		t.Fatalf("folds %d, want 1", n)
	}
	second, _ := folder.FoldRequest(body)
	if !bytes.Equal(first, second) {
		t.Fatal("folding the same request twice produced different bytes")
	}
	if !bytes.Contains(first, []byte(`"run the tests \u003cplease\u003e"`)) || !bytes.Contains(first, []byte(`"cache_control"`)) {
		t.Fatalf("unfolded fields changed: %s", first)
	}
	var request map[string]any
	if err := json.Unmarshal(first, &request); err != nil || request["max_tokens"] != float64(32000) {
		t.Fatalf("re-encoded body %v: %v", request["max_tokens"], err)
	}
}

func TestLMGV0005FoldSplicesOnlyFoldedText(t *testing.T) {
	folder := NewFolder(FoldOptions{Archive: t.TempDir()})
	big := strings.Repeat("output <line>\n", 1000)
	prefix := `{"model":"m","max_tokens":1.50,"messages":[{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"z":1,"a":"\u003c"},"id":"t1"}]},` +
		`{"role":"user","content":[{"content":`
	suffix := `,"tool_use_id":"t1","type":"tool_result"}]}] , "stream" : true}`
	body := []byte(prefix + jsonString(big) + suffix)
	out, n := folder.FoldRequest(body)
	if n != 1 || !bytes.HasPrefix(out, []byte(prefix)) || !bytes.HasSuffix(out, []byte(suffix)) || len(out) >= len(body) {
		t.Fatalf("folds %d; bytes outside the folded string changed:\n%.300s", n, out)
	}
}

func TestLMGV0006OnlyBashResultsFold(t *testing.T) {
	big := strings.Repeat("line of ordinary output\n", 1000)
	folder := NewFolder(FoldOptions{Archive: t.TempDir()})
	for _, tc := range []struct {
		name   string
		body   []byte
		folded bool
	}{
		{"bash", bashRequest(t, big, false, "Bash"), true},
		{"read", bashRequest(t, big, false, "Read"), false},
		{"error", bashRequest(t, big, true, "Bash"), false},
		{"small", bashRequest(t, "short\n", false, "Bash"), false},
	} {
		out, n := folder.FoldRequest(tc.body)
		if (n == 1) != tc.folded {
			t.Fatalf("%s: folds %d", tc.name, n)
		}
		if !tc.folded && !bytes.Equal(out, tc.body) {
			t.Fatalf("%s: unfolded body changed", tc.name)
		}
	}
	blocks := bashRequest(t, "", false, "Bash")
	blocks = bytes.Replace(blocks, []byte(`"content":""`), []byte(`"content":[{"type":"text","text":`+jsonString(big)+`},{"type":"image","source":{}}]`), 1)
	out, n := folder.FoldRequest(blocks)
	if n != 1 || !bytes.Contains(out, []byte(`"type":"image"`)) {
		t.Fatalf("text-block fold: %d", n)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestLMGV0007GoTestNoiseAndWindows(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "archive")
	folder := NewFolder(FoldOptions{Archive: archive})
	output := goTestOutput(300, "--- FAIL: TestBroken (0.01s)\n    x_test.go:9: want 1, got 2\nFAIL\n")
	folded, ok := folder.Fold(output)
	if !ok {
		t.Fatal("did not fold")
	}
	if strings.Contains(folded, "--- PASS") || !strings.Contains(folded, "--- FAIL: TestBroken") || !strings.Contains(folded, "want 1, got 2") {
		t.Fatalf("noise or failure handling wrong:\n%s", folded)
	}
	if !strings.HasPrefix(folded, "[corvint-gateway: folded ") || !strings.Contains(folded, "dropped 675 passing-test lines") {
		t.Fatalf("marker: %s", strings.SplitN(folded, "\n", 2)[0])
	}
	plain := strings.Repeat("0123456789 abcdefghij\n", 2000)
	folded, ok = folder.Fold(plain)
	if !ok || len(folded) > 4096+2048+600 || !strings.Contains(folded, "lines omitted") {
		t.Fatalf("windowed fold %d bytes", len(folded))
	}
	for _, line := range strings.Split(strings.TrimSuffix(folded, "\n"), "\n")[1:] {
		if line != "0123456789 abcdefghij" && !strings.HasPrefix(line, "[corvint-gateway:") {
			t.Fatalf("window cut mid-line: %q", line)
		}
	}
	path := strings.TrimSuffix(strings.SplitN(folded, ": ", 3)[2], "]\n"+strings.SplitN(folded, "]\n", 2)[1])
	path = path[strings.LastIndex(path, " ")+1:]
	saved, err := os.ReadFile(path)
	if err != nil || string(saved) != plain {
		t.Fatalf("archive %s: %v", path, err)
	}
	info, _ := os.Stat(path)
	dir, _ := os.Stat(archive)
	if info.Mode().Perm() != 0o600 || dir.Mode().Perm() != 0o700 || filepath.Dir(path) != archive {
		t.Fatalf("archive modes %v %v", info.Mode(), dir.Mode())
	}
}

func TestLMGV0007LongLineCutOnRuneBoundary(t *testing.T) {
	folder := NewFolder(FoldOptions{Archive: t.TempDir()})
	folded, ok := folder.Fold("x" + strings.Repeat("é", 10000))
	if !ok || !utf8.ValidString(folded) {
		t.Fatalf("fold %v produced invalid UTF-8", ok)
	}
}

func TestLMGV0008ArchiveReplacesStaleEntry(t *testing.T) {
	archive := t.TempDir()
	folder := NewFolder(FoldOptions{Archive: archive})
	text := strings.Repeat("archive me\n", 2000)
	sum := sha256.Sum256([]byte(text))
	path := filepath.Join(archive, hex.EncodeToString(sum[:])+".txt")
	if err := os.WriteFile(path, []byte("truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := folder.Fold(text); !ok {
		t.Fatal("did not fold")
	}
	saved, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(saved) != text || info.Mode().Perm() != 0o600 {
		t.Fatalf("stale archive entry kept: %d bytes, mode %v", len(saved), info.Mode())
	}
	other := strings.Repeat("blocked\n", 2000)
	sum = sha256.Sum256([]byte(other))
	if err := os.MkdirAll(filepath.Join(archive, hex.EncodeToString(sum[:])+".txt", "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, ok := folder.Fold(other); ok {
		t.Fatal("folded although the archive path is a non-empty directory")
	}
}

func TestLMGV0008ArchiveFailureForwardsUnfolded(t *testing.T) {
	folder := NewFolder(FoldOptions{Archive: t.TempDir()})
	folder.write = func(string, []byte) error { return errors.New("disk full") }
	body := bashRequest(t, strings.Repeat("x\n", 10000), false, "Bash")
	out, n := folder.FoldRequest(body)
	if n != 0 || !bytes.Equal(out, body) {
		t.Fatalf("folds %d", n)
	}
}

func TestLMGV0009MalformedBodyPassesThrough(t *testing.T) {
	folder := NewFolder(FoldOptions{Archive: t.TempDir()})
	for _, body := range [][]byte{[]byte("not json"), []byte(`{"messages":"odd"}`), []byte(`{"a":1}{"b":2}`), []byte(`{"a":1}]`), []byte(`{"a":1}}`)} {
		if out, n := folder.FoldRequest(body); n != 0 || !bytes.Equal(out, body) {
			t.Fatalf("%s changed", body)
		}
	}
}

const sseResponse = "event: message_start\n" +
	`data: {"type":"message_start","message":{"model":"claude-opus-5-5","usage":{"input_tokens":12,"cache_read_input_tokens":3400,"cache_creation_input_tokens":560,"output_tokens":1}}}` + "\n\n" +
	"event: content_block_delta\n" +
	`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}` + "\n\n" +
	"event: message_delta\n" +
	`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":77}}` + "\n\n" +
	"event: message_stop\n" + `data: {"type":"message_stop"}` + "\n\n"

type upstreamSeen struct {
	body   []byte
	header http.Header
	path   string
}

func newGateway(t *testing.T, folder *Folder, handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *bytes.Buffer, *upstreamSeen) {
	t.Helper()
	seen := &upstreamSeen{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.body, _ = io.ReadAll(r.Body)
		seen.header, seen.path = r.Header.Clone(), r.URL.RequestURI()
		handler(w, r)
	}))
	t.Cleanup(upstream.Close)
	target, _ := url.Parse(upstream.URL)
	var ledger bytes.Buffer
	proxy := NewProxy(target, folder, &ledger)
	clock := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	proxy.Now = func() time.Time { clock = clock.Add(250 * time.Millisecond); return clock }
	gateway := httptest.NewServer(proxy)
	t.Cleanup(gateway.Close)
	return gateway, &ledger, seen
}

// ledgerLines closes the gateway first: Close waits for every handler, so each
// ledger line is written before it is read.
func ledgerLines(t *testing.T, gateway *httptest.Server, ledger *bytes.Buffer) []Usage {
	t.Helper()
	gateway.Close()
	var out []Usage
	for _, line := range strings.Split(strings.TrimSpace(ledger.String()), "\n") {
		if line == "" {
			continue
		}
		var u Usage
		if err := json.Unmarshal([]byte(line), &u); err != nil {
			t.Fatal(err)
		}
		out = append(out, u)
	}
	return out
}

func TestLMGV0003PassthroughByteForByte(t *testing.T) {
	gateway, _, seen := newGateway(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Request-Id", "req_1")
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"model":"m","usage":{"input_tokens":5,"output_tokens":6}}`)
	})
	body := bashRequest(t, strings.Repeat("y\n", 20000), false, "Bash")
	request, _ := http.NewRequest(http.MethodPost, gateway.URL+"/v1/messages?beta=true", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer secret-token")
	request.Header.Set("Anthropic-Version", "2023-06-01")
	request.Header.Set("Connection", "keep-alive, X-Hop")
	request.Header.Add("Connection", "X-Hop-Two")
	request.Header.Set("X-Hop", "drop me")
	request.Header.Set("X-Hop-Two", "drop me too")
	request.Header.Set("Accept-Encoding", "br")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	reply, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !bytes.Equal(seen.body, body) || seen.path != "/v1/messages?beta=true" {
		t.Fatalf("upstream saw a different request: path %s, %d vs %d bytes", seen.path, len(seen.body), len(body))
	}
	if seen.header.Get("Authorization") != "Bearer secret-token" || seen.header.Get("Anthropic-Version") != "2023-06-01" || seen.header.Get("X-Hop") != "" || seen.header.Get("X-Hop-Two") != "" || seen.header.Get("Accept-Encoding") != "br" {
		t.Fatalf("headers %v", seen.header)
	}
	if response.StatusCode != 201 || response.Header.Get("Request-Id") != "req_1" || !strings.Contains(string(reply), `"input_tokens":5`) {
		t.Fatalf("response %d %v %s", response.StatusCode, response.Header, reply)
	}
	escaped, err := http.Get(gateway.URL + "/v1/files/a%2Fb")
	if err != nil {
		t.Fatal(err)
	}
	escaped.Body.Close()
	if seen.path != "/v1/files/a%2Fb" {
		t.Fatalf("escaped path forwarded as %s", seen.path)
	}
}

func TestLMGV0003StreamsAndNoRedirect(t *testing.T) {
	release := make(chan struct{})
	gateway, _, _ := newGateway(t, nil, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/redirect" {
			http.Redirect(w, r, "https://example.invalid/", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: ping\ndata: {\"type\":\"ping\"}\n\n")
		w.(http.Flusher).Flush()
		<-release
	})
	response, err := http.Post(gateway.URL+"/v1/messages", "application/json", strings.NewReader(`{"stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	first := make([]byte, 64)
	n, err := response.Body.Read(first)
	close(release)
	response.Body.Close()
	if err != nil || !strings.Contains(string(first[:n]), "ping") {
		t.Fatalf("first chunk was not flushed before the stream ended: %q %v", first[:n], err)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	redirect, err := client.Get(gateway.URL + "/v1/redirect")
	if err != nil {
		t.Fatal(err)
	}
	redirect.Body.Close()
	if redirect.StatusCode != http.StatusFound {
		t.Fatalf("gateway followed the redirect: %d", redirect.StatusCode)
	}
}

func TestLMGV0004LedgerFromStream(t *testing.T) {
	folder := NewFolder(FoldOptions{Archive: t.TempDir()})
	gateway, ledger, seen := newGateway(t, folder, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, part := range []string{sseResponse[:40], sseResponse[40:200], sseResponse[200:]} {
			_, _ = io.WriteString(w, part)
			w.(http.Flusher).Flush()
		}
	})
	body := bashRequest(t, goTestOutput(500, "done\n"), false, "Bash")
	request, _ := http.NewRequest(http.MethodPost, gateway.URL+"/v1/messages", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer secret-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	reply, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(reply) != sseResponse {
		t.Fatal("stream bytes changed")
	}
	lines := ledgerLines(t, gateway, ledger)
	if len(lines) != 1 {
		t.Fatalf("ledger lines %d", len(lines))
	}
	u := lines[0]
	if u.Model != "claude-opus-5-5" || !u.Stream || *u.InputTokens != 12 || *u.CacheReadTokens != 3400 || *u.CacheCreationTokens != 560 || *u.OutputTokens != 77 {
		t.Fatalf("usage %+v", u)
	}
	if u.Folds != 1 || u.RequestBytes != len(body) || u.ForwardedBytes != len(seen.body) || u.ForwardedBytes >= u.RequestBytes || u.DurationMs != 250 || len(u.SessionKey) != 16 {
		t.Fatalf("request facts %+v", u)
	}
	raw := ledger.String()
	for _, leak := range []string{"secret-token", "user_abc", "go test", "TestThing"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("ledger leaked %q: %s", leak, raw)
		}
	}
}

func TestLMGV0004LedgerFromJSON(t *testing.T) {
	gateway, ledger, _ := newGateway(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"claude-haiku-5-5","usage":{"input_tokens":9,"output_tokens":4}}`)
	})
	for _, path := range []string{"/v1/messages", "/v1/messages/count_tokens", "/v1/models"} {
		response, err := http.Post(gateway.URL+path, "application/json", strings.NewReader(`{"model":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
	}
	lines := ledgerLines(t, gateway, ledger)
	if len(lines) != 1 || lines[0].Model != "claude-haiku-5-5" || *lines[0].InputTokens != 9 || lines[0].CacheReadTokens != nil {
		t.Fatalf("ledger %+v", lines)
	}
}

func TestLMGV0009OversizeRefused(t *testing.T) {
	called := false
	gateway, _, _ := newGateway(t, nil, func(w http.ResponseWriter, r *http.Request) { called = true })
	target, _ := url.Parse(gateway.URL)
	var refused bytes.Buffer
	proxy := NewProxy(target, nil, &refused)
	proxy.MaxBody = 10
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(strings.Repeat("z", 11))))
	if recorder.Code != http.StatusRequestEntityTooLarge || called || !strings.Contains(refused.String(), `"status":413`) {
		t.Fatalf("code %d, upstream called %v", recorder.Code, called)
	}

	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	down, _ := url.Parse(closed.URL)
	var buf bytes.Buffer
	broken := NewProxy(down, nil, &buf)
	var log bytes.Buffer
	broken.Log = &log
	recorder = httptest.NewRecorder()
	broken.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`)))
	if recorder.Code != http.StatusBadGateway || !strings.Contains(buf.String(), `"status":502`) || !strings.Contains(log.String(), "upstream request failed") {
		t.Fatalf("upstream failure: %d %s", recorder.Code, buf.String())
	}
}

func TestLMGV0002RefusesNonLoopback(t *testing.T) {
	for addr, ok := range map[string]bool{"127.0.0.1:8787": true, "[::1]:0": true, "localhost:9": false, "0.0.0.0:8787": false, "192.168.4.3:80": false, ":8787": false} {
		if err := CheckLoopback(addr); (err == nil) != ok {
			t.Fatalf("%s: %v", addr, err)
		}
	}
}

func TestLMGV0002UpstreamScheme(t *testing.T) {
	for raw, ok := range map[string]bool{"https://api.anthropic.com": true, "http://127.0.0.1:9000": true, "http://localhost:9000": false, "http://api.anthropic.com": false, "ftp://x": false, "https://": false} {
		if _, err := ParseUpstream(raw); (err == nil) != ok {
			t.Fatalf("%s: %v", raw, err)
		}
	}
}

func TestLMGV0010Report(t *testing.T) {
	ledger := `{"profile":"corvint-gateway-usage/0","status":200,"inputTokens":10,"cacheReadInputTokens":90,"outputTokens":5,"requestBytes":1000,"forwardedBytes":600,"folds":2}
{"profile":"corvint-gateway-usage/0","status":502,"error":"upstream request failed","requestBytes":10,"forwardedBytes":10}
garbage
`
	totals, err := Summarise(strings.NewReader(ledger))
	if err != nil {
		t.Fatal(err)
	}
	want := Totals{Requests: 2, Errors: 2, Input: 10, CacheRead: 90, Output: 5, RequestBytes: 1010, ForwardedBytes: 610, Folds: 2}
	if totals != want {
		t.Fatalf("totals %+v, want %+v", totals, want)
	}
}

func TestLMGV0004MultiLineEventsAndGzip(t *testing.T) {
	split := "event: message_start\n" +
		`data: {"type":"message_start",` + "\n" +
		`data: "message":{"model":"claude-opus-5-5","usage":{"input_tokens":3,"output_tokens":1}}}` + "\n\n" +
		`data: {"type":"message_delta","usage":{"output_tokens":8}}` + "\n\n"
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	_, _ = io.WriteString(zw, `{"model":"claude-haiku-5-5","usage":{"input_tokens":21,"output_tokens":2}}`)
	_ = zw.Close()
	gateway, ledger, _ := newGateway(t, nil, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") == "gzip" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Encoding", "gzip")
			_, _ = w.Write(zipped.Bytes())
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, split)
	})
	plain, _ := http.NewRequest(http.MethodPost, gateway.URL+"/v1/messages", strings.NewReader(`{}`))
	plain.Header.Set("Accept-Encoding", "identity")
	response, err := http.DefaultClient.Do(plain)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	request, _ := http.NewRequest(http.MethodPost, gateway.URL+"/v1/messages", strings.NewReader(`{}`))
	request.Header.Set("Accept-Encoding", "gzip")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	reply, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !bytes.Equal(reply, zipped.Bytes()) {
		t.Fatal("gzip body was not relayed unchanged")
	}
	lines := ledgerLines(t, gateway, ledger)
	if len(lines) != 2 || lines[0].InputTokens == nil || *lines[0].InputTokens != 3 || *lines[0].OutputTokens != 8 ||
		lines[1].InputTokens == nil || *lines[1].InputTokens != 21 || lines[1].Model != "claude-haiku-5-5" {
		t.Fatalf("ledger %s", ledger)
	}
}

func TestLMGV0009TruncatedStreamAborts(t *testing.T) {
	gateway, ledger, _ := newGateway(t, nil, func(w http.ResponseWriter, r *http.Request) {
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n")
		_ = buf.Flush()
		conn.Close()
	})
	response, err := http.Post(gateway.URL+"/v1/messages", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(bufio.NewReader(response.Body))
	response.Body.Close()
	if err == nil {
		t.Fatal("a truncated upstream stream ended cleanly for the client")
	}
	gateway.Close() // waits for the handler, which records before aborting
	if !strings.Contains(ledger.String(), `"error":"upstream read failed"`) {
		t.Fatalf("ledger %s", ledger)
	}
}

func TestLMGV0011LedgerPrefixKeys(t *testing.T) {
	gateway, ledger, _ := newGateway(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"m","usage":{"input_tokens":1}}`)
	})
	system, tools := `[{"type":"text","text":"SECRET-SYSTEM"}]`, `[{"name":"Bash","input_schema":{}}]`
	for _, body := range []string{
		`{"model":"m","system":` + system + `,"tools":` + tools + `,"metadata":{"user_id":"u"}}`,
		`{"model":"m","system":` + system + `,"tools":` + tools + `}`,
		`{"model":"m"}`,
	} {
		response, err := http.Post(gateway.URL+"/v1/messages", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
	}
	lines := ledgerLines(t, gateway, ledger)
	raw := ledger.String()
	if strings.Contains(raw, "SECRET-SYSTEM") || strings.Contains(raw, "input_schema") {
		t.Fatalf("ledger stored prefix text: %s", raw)
	}
	first, second, bare := lines[0], lines[1], lines[2]
	if first.SystemBytes != len(system) || first.ToolsBytes != len(tools) || len(first.SystemKey) != 16 || len(first.ToolsKey) != 16 ||
		first.SystemKey == first.ToolsKey || first.SessionKey == "" {
		t.Fatalf("first %+v", first)
	}
	if second.SystemKey != first.SystemKey || second.ToolsKey != first.ToolsKey || second.SessionKey != "" {
		t.Fatalf("second %+v", second)
	}
	if bare.SystemBytes != 0 || bare.SystemKey != "" || bare.ToolsBytes != 0 || bare.ToolsKey != "" {
		t.Fatalf("bare %+v", bare)
	}
	totals, err := Summarise(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if totals.ToolsBytes != int64(2*len(tools)) || totals.ToolsRequestBytes != int64(first.RequestBytes+second.RequestBytes) {
		t.Fatalf("totals %+v", totals)
	}
}

func TestLMGV0012CacheBreaks(t *testing.T) {
	// line is one request that ended at 12:mm:ss and took ms milliseconds.
	line := func(at string, ms int, session, system, tools string, read, created int, extra string) string {
		return fmt.Sprintf(`{"profile":"corvint-gateway-usage/0","at":"2026-11-01T12:%sZ","durationMs":%d,"status":200,"model":"m","sessionKey":%q,"systemKey":%q,"toolsKey":%q,"cacheReadInputTokens":%d,"cacheCreationInputTokens":%d%s}`,
			at, ms, session, system, tools, read, created, extra)
	}
	ledger := strings.Join([]string{
		line("00:00", 1000, "s", "a", "t", 0, 50000, ""),                                // warm-up
		line("00:05", 1000, "s", "", "", 0, 0, ""),                                      // side request, skipped
		line("00:10", 1000, "s", "a", "t", 50000, 500, ""),                              // stable
		line("00:20", 1000, "s", "a", "t", 50300, 200, ""),                              // 200 short: noise
		line("00:30", 1000, "s", "a", "t", 0, 60000, `,"error":"upstream read failed"`), // failed stream, skipped
		line("01:00", 1000, "s", "a", "t", 4000, 47000, ""),                             // 1: edited prefix
		line("01:30", 1000, "s", "a", "t", 3000, 9000, ""),                              // 2: compaction
		line("02:00", 1000, "s", "a", "t", 12000, 600, ""),                              // stable after compaction
		line("02:30", 1000, "s", "b", "u", 12600, 2000, ""),                             // new key, full hit: no break
		line("03:00", 1000, "s", "c", "v", 0, 9000, ""),                                 // 3: subagent or tool change
		line("03:30", 1000, "s", "a", "t", 12600, 100, ""),                              // main resumes, stable
		line("10:00", 300000, "s", "a", "t", 12700, 300, ""),                            // 5-minute stream, stable
		line("10:30", 1000, "s", "a", "t", 0, 13500, ""),                                // 4: idle since the stream started
		line("11:00", 20000, "p", "a", "t", 0, 40000, ""),                               // two parallel cold
		line("11:05", 25000, "p", "a", "t", 0, 40000, ""),                               // requests: no break
		line("11:30", 1000, "p", "a", "t", 40000, 100, ""),                              // stable
		`{"profile":"corvint-gateway-usage/0","status":529,"sessionKey":"s","cacheReadInputTokens":0,"cacheCreationInputTokens":0}`,
		"garbage",
	}, "\n")
	breaks, err := CacheBreaks(strings.NewReader(ledger))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		cause                   string
		expected, read, created int64
		gap                     time.Duration
	}{
		{CausePrefixChanged, 50500, 4000, 47000, 40 * time.Second},
		{CausePrefixShrank, 51000, 3000, 9000, 30 * time.Second},
		{CausePrefixSwitch, 14600, 0, 9000, 30 * time.Second},
		{CauseIdle, 13000, 0, 13500, 5*time.Minute + 29*time.Second},
	}
	if len(breaks) != len(want) {
		t.Fatalf("breaks %+v", breaks)
	}
	for i, w := range want {
		b := breaks[i]
		if b.Cause != w.cause || b.Expected != w.expected || b.Read != w.read || b.Created != w.created || b.Gap != w.gap {
			t.Fatalf("break %d %+v, want %+v", i, b, w)
		}
	}
}

// nativeSessionKey is cmd/corvint's claudeSessionHash("native-session-é"), the
// key TestClaudeNativeDogfoodLifecycle enrols under.
const nativeSessionKey = "81c2630f9c914634e81e95e307785d68ed4a291236f537bd4ef9574bc99ff531"

func TestLMGV0013ClaudeSessionKey(t *testing.T) {
	gateway, ledger, _ := newGateway(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"m","usage":{"input_tokens":1}}`)
	})
	userID := `{"device_id":"DEVICE-SECRET","account_uuid":"ACCOUNT-SECRET","session_id":"native-session-é"}`
	encoded, _ := json.Marshal(userID)
	for _, metadata := range []string{
		`{"user_id":` + string(encoded) + `}`,
		`{"user_id":"user_abc_account_def_session_native-session-é"}`,
		`{"user_id":"{\"session_id\":\"\"}"}`,
	} {
		response, err := http.Post(gateway.URL+"/v1/messages", "application/json", strings.NewReader(`{"model":"m","metadata":`+metadata+`}`))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
	}
	lines := ledgerLines(t, gateway, ledger)
	if raw := ledger.String(); strings.Contains(raw, "SECRET") || strings.Contains(raw, "native-session") {
		t.Fatalf("ledger stored an identifier: %s", raw)
	}
	if lines[0].ClaudeSessionKey != nativeSessionKey || lines[0].SessionKey == "" {
		t.Fatalf("claude line %+v", lines[0])
	}
	if lines[1].ClaudeSessionKey != "" || lines[2].ClaudeSessionKey != "" {
		t.Fatalf("non-Claude user_id keyed: %+v %+v", lines[1], lines[2])
	}
}

func TestLMGV0014FilterSession(t *testing.T) {
	other := strings.Repeat("0", 64)
	ledger := `{"profile":"corvint-gateway-usage/0","status":200,"claudeSessionKey":"` + nativeSessionKey + `","inputTokens":10,"outputTokens":5}
{"profile":"corvint-gateway-usage/0","status":200,"claudeSessionKey":"` + other + `","inputTokens":99}
garbage
{"profile":"corvint-gateway-usage/0","status":200,"inputTokens":7}
{"profile":"corvint-gateway-usage/0","status":200,"claudeSessionKey":"` + nativeSessionKey + `","cacheReadInputTokens":40}
`
	lines, err := FilterSession(strings.NewReader(ledger), nativeSessionKey)
	if err != nil {
		t.Fatal(err)
	}
	totals, err := Summarise(bytes.NewReader(lines))
	if err != nil {
		t.Fatal(err)
	}
	if want := (Totals{Requests: 2, Input: 10, CacheRead: 40, Output: 5}); totals != want {
		t.Fatalf("totals %+v, want %+v", totals, want)
	}
	if none, err := FilterSession(strings.NewReader(ledger), strings.Repeat("f", 64)); err != nil || len(none) != 0 {
		t.Fatalf("unknown key kept %q, %v", none, err)
	}
}
