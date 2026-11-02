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
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// UsageProfile names one ledger line (LMG-V0-004).
const UsageProfile = "corvint-gateway-usage/0"

// DefaultMaxBody is the request size refused with 413 (LMG-V0-009).
const DefaultMaxBody = 64 << 20

// Usage is one ledger line. Counts the response did not report stay nil.
type Usage struct {
	Profile             string `json:"profile"`
	At                  string `json:"at"`
	Path                string `json:"path"`
	Status              int    `json:"status"`
	Model               string `json:"model,omitempty"`
	Stream              bool   `json:"stream"`
	InputTokens         *int64 `json:"inputTokens,omitempty"`
	CacheReadTokens     *int64 `json:"cacheReadInputTokens,omitempty"`
	CacheCreationTokens *int64 `json:"cacheCreationInputTokens,omitempty"`
	OutputTokens        *int64 `json:"outputTokens,omitempty"`
	RequestBytes        int    `json:"requestBytes"`
	ForwardedBytes      int    `json:"forwardedBytes"`
	Folds               int    `json:"folds"`
	DurationMs          int64  `json:"durationMs"`
	SessionKey          string `json:"sessionKey,omitempty"`
	ClaudeSessionKey    string `json:"claudeSessionKey,omitempty"`
	SystemBytes         int    `json:"systemBytes,omitempty"`
	SystemKey           string `json:"systemKey,omitempty"`
	ToolsBytes          int    `json:"toolsBytes,omitempty"`
	ToolsKey            string `json:"toolsKey,omitempty"`
	Error               string `json:"error,omitempty"`
}

// Proxy forwards requests to Upstream, folding Messages requests when Folder
// is set and appending one Usage line per Messages request to Ledger.
type Proxy struct {
	Upstream *url.URL
	Client   *http.Client
	Folder   *Folder
	Ledger   io.Writer
	Log      io.Writer // optional; receives one line per upstream or ledger failure
	MaxBody  int64
	Now      func() time.Time

	mu sync.Mutex
}

// NewProxy returns a Proxy whose client never follows redirects (LMG-V0-003).
func NewProxy(upstream *url.URL, folder *Folder, ledger io.Writer) *Proxy {
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Proxy{Upstream: upstream, Client: client, Folder: folder, Ledger: ledger, MaxBody: DefaultMaxBody, Now: time.Now}
}

// CheckLoopback refuses a listen address whose host is not a numeric loopback
// IP (LMG-V0-002). A name such as localhost is refused because its resolution
// is outside the gateway's control.
func CheckLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if !loopback(host) {
		return errors.New("address " + addr + " is not a loopback IP such as 127.0.0.1 or [::1]: the gateway has no authentication and must not be reachable from another host")
	}
	return nil
}

// ParseUpstream accepts an https URL, or http only for a numeric loopback IP (LMG-V0-002).
func ParseUpstream(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && loopback(u.Hostname()))) {
		return nil, errors.New("upstream " + raw + " must be an https URL, or http on a loopback IP")
	}
	return u, nil
}

func loopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// hopHeaders are dropped in both directions (RFC 9110 §7.6.1).
var hopHeaders = []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
	for _, value := range src.Values("Connection") {
		for _, key := range strings.Split(value, ",") {
			if key = strings.TrimSpace(key); key != "" {
				dst.Del(key)
			}
		}
	}
	for _, key := range hopHeaders {
		dst.Del(key)
	}
}

// ServeHTTP forwards one request (LMG-V0-003) and records it (LMG-V0-004).
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := p.Now()
	messages := r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/v1/messages")
	body, err := io.ReadAll(io.LimitReader(r.Body, p.MaxBody+1))
	if err != nil {
		http.Error(w, "corvint-gateway: reading request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if int64(len(body)) > p.MaxBody {
		p.record(messages, &Usage{Profile: UsageProfile, Path: r.URL.Path, Status: http.StatusRequestEntityTooLarge,
			RequestBytes: len(body), Error: "request body exceeds the size limit"}, start)
		http.Error(w, "corvint-gateway: request body exceeds the size limit", http.StatusRequestEntityTooLarge)
		return
	}
	usage := Usage{Profile: UsageProfile, Path: r.URL.Path, RequestBytes: len(body)}
	forward := body
	if messages {
		if p.Folder != nil {
			forward, usage.Folds = p.Folder.FoldRequest(body)
		}
		requestFacts(body, &usage)
	}
	usage.ForwardedBytes = len(forward)

	target := *p.Upstream
	target.Path = strings.TrimSuffix(p.Upstream.Path, "/") + r.URL.Path
	target.RawPath = strings.TrimSuffix(p.Upstream.EscapedPath(), "/") + r.URL.EscapedPath()
	target.RawQuery = r.URL.RawQuery
	out, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), bytes.NewReader(forward))
	if err != nil {
		http.Error(w, "corvint-gateway: "+err.Error(), http.StatusBadGateway)
		return
	}
	copyHeaders(out.Header, r.Header)
	out.Header.Del("Content-Length")
	out.ContentLength = int64(len(forward))

	response, err := p.Client.Do(out)
	if err != nil {
		usage.Status, usage.Error = http.StatusBadGateway, "upstream request failed"
		p.record(messages, &usage, start)
		if p.Log != nil {
			fmt.Fprintf(p.Log, "corvint-gateway: upstream request failed: %v\n", err)
		}
		http.Error(w, "corvint-gateway: upstream request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	copyHeaders(w.Header(), response.Header)
	if response.Uncompressed {
		w.Header().Del("Content-Length")
		w.Header().Del("Content-Encoding")
	}
	w.WriteHeader(response.StatusCode)
	usage.Status = response.StatusCode

	var meter usageMeter
	sse := strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream")
	feed := func(chunk []byte) { meter.feed(chunk, sse) }
	// The client's encoding is relayed untouched; the meter reads a decoded
	// copy of gzip, and an encoding it cannot decode leaves the counts absent.
	var gunzip *gunzipFeed
	switch encoding := response.Header.Get("Content-Encoding"); {
	case !messages || response.Uncompressed || encoding == "" || encoding == "identity":
	case encoding == "gzip":
		gunzip = newGunzipFeed(feed)
		feed = gunzip.write
	default:
		feed = func([]byte) {}
	}
	flusher, _ := w.(http.Flusher)
	buffer := make([]byte, 32<<10)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if messages {
				feed(buffer[:n])
			}
			if _, err := w.Write(buffer[:n]); err != nil {
				usage.Error = "client write failed"
				break
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				usage.Error = "upstream read failed"
			}
			break
		}
	}
	if gunzip != nil {
		gunzip.close()
	}
	if messages {
		meter.finish(sse)
		meter.apply(&usage)
	}
	p.record(messages, &usage, start)
	if usage.Error == "upstream read failed" {
		// The status is already sent; aborting tells the client the body is
		// truncated instead of letting it end as if complete.
		panic(http.ErrAbortHandler)
	}
}

// gunzipFeed decodes a gzip body for the meter on its own goroutine.
type gunzipFeed struct {
	pipe *io.PipeWriter
	done chan struct{}
}

func newGunzipFeed(sink func([]byte)) *gunzipFeed {
	reader, writer := io.Pipe()
	g := &gunzipFeed{pipe: writer, done: make(chan struct{})}
	go func() {
		defer close(g.done)
		// Closing the reader makes later writes fail at once rather than block.
		defer reader.Close()
		decoder, err := gzip.NewReader(reader)
		if err != nil {
			return
		}
		buffer := make([]byte, 32<<10)
		for {
			n, err := decoder.Read(buffer)
			if n > 0 {
				sink(buffer[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	return g
}

func (g *gunzipFeed) write(chunk []byte) { _, _ = g.pipe.Write(chunk) }

func (g *gunzipFeed) close() {
	_ = g.pipe.Close()
	<-g.done
}

func (p *Proxy) record(messages bool, usage *Usage, start time.Time) {
	if !messages || p.Ledger == nil {
		return
	}
	now := p.Now()
	usage.At = now.UTC().Format(time.RFC3339Nano)
	usage.DurationMs = now.Sub(start).Milliseconds()
	line, err := json.Marshal(usage)
	if err != nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.Ledger.Write(append(line, '\n')); err != nil && p.Log != nil {
		fmt.Fprintf(p.Log, "corvint-gateway: writing the usage ledger: %v\n", err)
	}
}

// requestFacts copies stream, a hashed metadata.user_id, and the size and a
// hashed key of the system prompt and tool list into u (LMG-V0-004,
// LMG-V0-011). Each hash keeps a grouping or comparison key without storing
// the identifier or text itself.
func requestFacts(body []byte, u *Usage) {
	var request struct {
		Stream   bool `json:"stream"`
		Metadata struct {
			UserID string `json:"user_id"`
		} `json:"metadata"`
		System json.RawMessage `json:"system"`
		Tools  json.RawMessage `json:"tools"`
	}
	if json.Unmarshal(body, &request) != nil {
		return
	}
	u.Stream, u.SessionKey = request.Stream, shortKey([]byte(request.Metadata.UserID))
	u.ClaudeSessionKey = claudeSessionKey(request.Metadata.UserID)
	u.SystemBytes, u.SystemKey = len(request.System), shortKey(request.System)
	u.ToolsBytes, u.ToolsKey = len(request.Tools), shortKey(request.Tools)
}

// claudeSessionDomain separates the Claude Code session hash; it must match
// claudeSessionHash in cmd/corvint/host_adapter.go, whose result names the
// session's dogfood enrollment (`corvint dogfood status --session-key`).
const claudeSessionDomain = "corvint-local-completion-session/claude-code/0"

// claudeSessionKey returns the Corvint session key of a Claude Code request,
// whose metadata.user_id is a JSON object carrying session_id, or "" for any
// other user_id (LMG-V0-013).
func claudeSessionKey(userID string) string {
	var id struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal([]byte(userID), &id) != nil || id.SessionID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(claudeSessionDomain + "\x00" + id.SessionID))
	return hex.EncodeToString(sum[:])
}

// FilterSession returns the ledger lines whose claudeSessionKey equals key,
// each with its newline, so Summarise and CacheBreaks can report one Claude
// Code session (LMG-V0-014). Unreadable lines are dropped.
func FilterSession(r io.Reader, key string) ([]byte, error) {
	var out bytes.Buffer
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var u Usage
		if json.Unmarshal(scanner.Bytes(), &u) == nil && u.Profile == UsageProfile && u.ClaudeSessionKey == key {
			out.Write(scanner.Bytes())
			out.WriteByte('\n')
		}
	}
	return out.Bytes(), scanner.Err()
}

// shortKey is the first 8 bytes of SHA-256 in hex, or "" for empty input.
func shortKey(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

// maxMeterLine bounds what the meter buffers for one SSE line or JSON body.
const maxMeterLine = 8 << 20

type rawUsage struct {
	InputTokens         *int64 `json:"input_tokens"`
	CacheReadTokens     *int64 `json:"cache_read_input_tokens"`
	CacheCreationTokens *int64 `json:"cache_creation_input_tokens"`
	OutputTokens        *int64 `json:"output_tokens"`
}

// usageMeter extracts model and usage from a response as it streams past.
type usageMeter struct {
	pending  []byte
	overflow bool
	data     []byte // data lines of the SSE event being assembled
	skip     bool   // the current event outgrew maxMeterLine
	model    string
	usage    rawUsage
}

func (m *usageMeter) feed(chunk []byte, sse bool) {
	if m.overflow {
		if !sse {
			return
		}
		if i := bytes.IndexByte(chunk, '\n'); i >= 0 {
			m.overflow, chunk = false, chunk[i+1:]
		} else {
			return
		}
	}
	m.pending = append(m.pending, chunk...)
	if sse {
		for {
			i := bytes.IndexByte(m.pending, '\n')
			if i < 0 {
				break
			}
			m.line(m.pending[:i])
			m.pending = m.pending[i+1:]
		}
	}
	if len(m.pending) > maxMeterLine {
		m.pending, m.overflow = nil, true
	}
}

func (m *usageMeter) finish(sse bool) {
	if sse {
		m.line(m.pending)
		m.event()
	} else if !m.overflow {
		var body struct {
			Model string   `json:"model"`
			Usage rawUsage `json:"usage"`
		}
		if json.Unmarshal(m.pending, &body) == nil {
			m.model = body.Model
			m.merge(body.Usage)
		}
	}
	m.pending = nil
}

// line takes one SSE line; a blank line ends the event, whose data lines are
// joined with newlines before decoding.
func (m *usageMeter) line(line []byte) {
	line = bytes.TrimRight(line, "\r")
	if len(line) == 0 {
		m.event()
		return
	}
	data, ok := bytes.CutPrefix(line, []byte("data:"))
	if !ok || m.skip {
		return
	}
	if m.data == nil {
		m.data = []byte{}
	} else {
		m.data = append(m.data, '\n')
	}
	m.data = append(m.data, bytes.TrimPrefix(data, []byte(" "))...)
	if len(m.data) > maxMeterLine {
		m.data, m.skip = nil, true
	}
}

func (m *usageMeter) event() {
	data := m.data
	m.data, m.skip = nil, false
	if data == nil {
		return
	}
	var event struct {
		Type    string `json:"type"`
		Message struct {
			Model string   `json:"model"`
			Usage rawUsage `json:"usage"`
		} `json:"message"`
		Usage rawUsage `json:"usage"`
	}
	if json.Unmarshal(data, &event) != nil {
		return
	}
	switch event.Type {
	case "message_start":
		m.model = event.Message.Model
		m.merge(event.Message.Usage)
	case "message_delta":
		m.merge(event.Usage)
	}
}

// merge keeps the latest reported value of each count; message_delta counts
// are cumulative.
func (m *usageMeter) merge(u rawUsage) {
	for _, pair := range []struct {
		dst **int64
		src *int64
	}{
		{&m.usage.InputTokens, u.InputTokens},
		{&m.usage.CacheReadTokens, u.CacheReadTokens},
		{&m.usage.CacheCreationTokens, u.CacheCreationTokens},
		{&m.usage.OutputTokens, u.OutputTokens},
	} {
		if pair.src != nil {
			*pair.dst = pair.src
		}
	}
}

func (m *usageMeter) apply(u *Usage) {
	u.Model = m.model
	u.InputTokens, u.CacheReadTokens = m.usage.InputTokens, m.usage.CacheReadTokens
	u.CacheCreationTokens, u.OutputTokens = m.usage.CacheCreationTokens, m.usage.OutputTokens
}

// Totals summarise a ledger for report (LMG-V0-010).
type Totals struct {
	Requests, Errors                        int
	Input, CacheRead, CacheCreation, Output int64
	RequestBytes, ForwardedBytes, Folds     int64
	// ToolsBytes sums tool-definition bytes over the requests that carried a
	// tool list, whose received bytes sum to ToolsRequestBytes (LMG-V0-011).
	ToolsBytes, ToolsRequestBytes int64
}

// Summarise reads ledger lines; a malformed line is skipped and counted as an error.
func Summarise(r io.Reader) (Totals, error) {
	var t Totals
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var u Usage
		if json.Unmarshal(scanner.Bytes(), &u) != nil || u.Profile != UsageProfile {
			t.Errors++
			continue
		}
		t.Requests++
		if u.Error != "" || u.Status >= 400 {
			t.Errors++
		}
		for _, pair := range []struct {
			dst *int64
			src *int64
		}{
			{&t.Input, u.InputTokens}, {&t.CacheRead, u.CacheReadTokens},
			{&t.CacheCreation, u.CacheCreationTokens}, {&t.Output, u.OutputTokens},
		} {
			if pair.src != nil {
				*pair.dst += *pair.src
			}
		}
		t.RequestBytes += int64(u.RequestBytes)
		t.ForwardedBytes += int64(u.ForwardedBytes)
		t.Folds += int64(u.Folds)
		if u.ToolsBytes > 0 {
			t.ToolsBytes += int64(u.ToolsBytes)
			t.ToolsRequestBytes += int64(u.RequestBytes)
		}
	}
	return t, scanner.Err()
}
