// Package goplsclient provides an experimental explicitly configured local gopls
// session. It supplies observations only; snapshot authority belongs to callers.
package goplsclient

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/procgroup"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

const MaxMessageBytes = 1 << 20
const MaxDocumentBytes = 256 << 10
const MaxDocuments = 32
const SessionLimit = 10 * time.Minute
const RequestLimit = 20 * time.Second

var ErrSession = errors.New("gopls session unavailable")
var ErrUnsupported = errors.New("gopls profile unsupported")
var ErrSnapshot = errors.New("snapshot invalid or stale")
var ErrProtocol = errors.New("gopls protocol invalid")

// Snapshot is opaque caller identity and content, never inferred from disk.
// Identity must change across reopen/reset even for equal version and text.
// Callbacks must be concurrent-safe, bounded and side-effect free.
type Snapshot struct {
	URI, Identity, Text string
	Version             int
	Overlay             bool
}
type Config struct {
	Executable, Root string
	Snapshot         func(string) (Snapshot, bool)
	Current          func(Snapshot) bool
}
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Location struct {
	Snapshot Snapshot
	Range    Range
}
type Profile struct {
	ServerVersion, ExecutableSHA256, PositionEncoding string
	Definition                                        bool
}
type envelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}
type response struct {
	body json.RawMessage
	err  error
}

// Client serializes synchronization and requests. Cancellation sends $/cancelRequest;
// later responses are discarded by ID. Session cancellation joins cleanup.
type syncStamp struct {
	version  int
	identity string
	digest   [32]byte
}

func stamp(s Snapshot) syncStamp {
	return syncStamp{s.Version, s.Identity, sha256.Sum256([]byte(s.Text))}
}

type Client struct {
	config      Config
	profile     Profile
	writer      io.WriteCloser
	cancel      context.CancelFunc
	done        chan struct{}
	gate        chan struct{}
	mu          sync.Mutex
	pending     map[int]chan response
	next        int
	sent        int
	versions    map[string]syncStamp
	observation procgroup.Observation
}

func Start(ctx context.Context, config Config) (*Client, error) {
	if ctx == nil || config.Snapshot == nil || config.Current == nil || !filepath.IsAbs(config.Executable) || !filepath.IsAbs(config.Root) {
		return nil, ErrUnsupported
	}
	root, err := filepath.EvalSymlinks(config.Root)
	if err != nil || root != filepath.Clean(config.Root) {
		return nil, ErrUnsupported
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, ErrUnsupported
	}
	f, err := os.Open(config.Executable)
	if err != nil {
		return nil, ErrSession
	}
	h := sha256.New()
	_, err = io.Copy(h, io.LimitReader(f, 128<<20))
	info, statErr := f.Stat()
	_ = f.Close()
	if err != nil || statErr != nil || !info.Mode().IsRegular() || info.Size() > 128<<20 || info.Mode()&0111 == 0 {
		return nil, ErrUnsupported
	}
	cache, err := os.MkdirTemp("", "corvint-gopls-session-")
	if err != nil {
		return nil, ErrSession
	}
	sessionCtx, cancel := context.WithTimeout(ctx, SessionLimit)
	c := &Client{config: config, cancel: cancel, done: make(chan struct{}), gate: make(chan struct{}, 1), pending: map[int]chan response{}, versions: map[string]syncStamp{}}
	c.profile.ExecutableSHA256 = hex.EncodeToString(h.Sum(nil))
	ready := make(chan struct{})
	go func() {
		defer close(c.done)
		defer cancel()
		defer os.RemoveAll(cache)
		c.observation = procgroup.Run(sessionCtx, procgroup.Spec{Argv: []string{config.Executable, "serve"}, Dir: root, Env: environment(cache), Timeout: SessionLimit, ShutdownTimeout: 2 * time.Second, OutputLimit: 32 << 20, StderrLimit: 64 << 10, ObserveDescendants: true, Dialogue: func(r io.Reader, w io.WriteCloser) error { c.writer = w; close(ready); return c.readLoop(r) }})
	}()
	select {
	case <-ready:
	case <-c.done:
		return nil, ErrSession
	case <-ctx.Done():
		cancel()
		<-c.done
		return nil, ctx.Err()
	}
	c.gate <- struct{}{}
	result, err := c.call(ctx, "initialize", map[string]any{"processId": os.Getpid(), "rootUri": fileURI(root), "capabilities": map[string]any{"general": map[string]any{"positionEncodings": []string{"utf-16"}}, "textDocument": map[string]any{"definition": map[string]any{"linkSupport": false}}}, "initializationOptions": map[string]any{"env": map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local", "GOTELEMETRY": "off"}}})
	if err == nil {
		err = c.negotiate(result)
	}
	if err == nil {
		err = c.notify(ctx, "initialized", map[string]any{})
	}
	if err != nil {
		cancel()
		<-c.done
		return nil, err
	}
	return c, nil
}
func (c *Client) Profile() Profile      { return c.profile }
func (c *Client) Done() <-chan struct{} { return c.done }

// CleanupProven is meaningful after Done closes; it does not claim sandbox containment.
func (c *Client) CleanupProven() bool {
	select {
	case <-c.done:
		return c.observation.OwnedProcessGroupCleanup
	default:
		return false
	}
}
func (c *Client) negotiate(raw json.RawMessage) error {
	var v struct {
		Capabilities struct {
			PositionEncoding string          `json:"positionEncoding"`
			Definition       json.RawMessage `json:"definitionProvider"`
			Sync             json.RawMessage `json:"textDocumentSync"`
		} `json:"capabilities"`
		Server struct{ Name, Version string } `json:"serverInfo"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Server.Name != "gopls" || v.Server.Version == "" || len(v.Server.Version) > 65536 {
		return ErrUnsupported
	}
	if v.Capabilities.PositionEncoding != "" && v.Capabilities.PositionEncoding != "utf-16" {
		return ErrUnsupported
	}
	// An omitted encoding has the protocol-defined UTF-16 meaning.
	var opts struct {
		OpenClose bool `json:"openClose"`
		Change    int  `json:"change"`
	}
	if json.Unmarshal(v.Capabilities.Sync, &opts) != nil || !opts.OpenClose || (opts.Change != 1 && opts.Change != 2) {
		return ErrUnsupported
	}
	var yes bool
	var options map[string]json.RawMessage
	if json.Unmarshal(v.Capabilities.Definition, &yes) != nil {
		if json.Unmarshal(v.Capabilities.Definition, &options) != nil || options == nil {
			return ErrUnsupported
		}
		yes = true
	}
	if !yes {
		return ErrUnsupported
	}
	if strings.HasPrefix(v.Server.Version, "{") {
		var build struct{ Version string }
		if json.Unmarshal([]byte(v.Server.Version), &build) != nil {
			return ErrUnsupported
		}
		v.Server.Version = build.Version
	}
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(v.Server.Version) {
		return ErrUnsupported
	}
	c.profile.ServerVersion = v.Server.Version
	c.profile.PositionEncoding = "utf-16"
	c.profile.Definition = true
	return nil
}
func (c *Client) acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return ErrSession
	case <-c.gate:
		return nil
	}
}
func (c *Client) release() { c.gate <- struct{}{} }
func (c *Client) snapshot(uri string) (Snapshot, error) {
	if !validURI(c.config.Root, uri) {
		return Snapshot{}, ErrSnapshot
	}
	s, ok := c.config.Snapshot(uri)
	if !ok || s.URI != uri || s.Identity == "" || len(s.Identity) > 512 || len(s.Text) > MaxDocumentBytes || !utf8.ValidString(s.Text) || !c.config.Current(s) {
		return Snapshot{}, ErrSnapshot
	}
	return s, nil
}
func (c *Client) Open(ctx context.Context, uri string) error   { return c.syncDocument(ctx, uri, false) }
func (c *Client) Change(ctx context.Context, uri string) error { return c.syncDocument(ctx, uri, true) }
func (c *Client) syncDocument(ctx context.Context, uri string, change bool) error {
	if err := c.acquire(ctx); err != nil {
		return err
	}
	defer c.release()
	s, err := c.snapshot(uri)
	if err != nil || !s.Overlay || s.Version < 0 || s.Version > 2147483647 {
		return ErrSnapshot
	}
	old, exists := c.versions[uri]
	if change {
		if !exists || s.Version <= old.version {
			return ErrSnapshot
		}
	} else if exists || len(c.versions) >= MaxDocuments {
		return ErrSnapshot
	}
	method := "textDocument/didOpen"
	params := map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "go", "version": s.Version, "text": s.Text}}
	if change {
		method = "textDocument/didChange"
		params = map[string]any{"textDocument": map[string]any{"uri": uri, "version": s.Version}, "contentChanges": []any{map[string]any{"text": s.Text}}}
	}
	if err = c.notify(ctx, method, params); err != nil {
		return err
	}
	c.versions[uri] = stamp(s)
	if !c.config.Current(s) {
		return ErrSnapshot
	}
	return nil
}
func (c *Client) CloseDocument(ctx context.Context, uri string) error {
	if err := c.acquire(ctx); err != nil {
		return err
	}
	defer c.release()
	if _, ok := c.versions[uri]; !ok {
		return ErrSnapshot
	}
	if err := c.notify(ctx, "textDocument/didClose", map[string]any{"textDocument": map[string]string{"uri": uri}}); err != nil {
		return err
	}
	delete(c.versions, uri)
	return nil
}
func (c *Client) Definition(ctx context.Context, uri string, p Position) ([]Location, error) {
	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer c.release()
	s, err := c.snapshot(uri)
	if err != nil || !s.Overlay || !validPosition(s.Text, p) {
		return nil, ErrSnapshot
	}
	if s.Overlay {
		if v, ok := c.versions[uri]; !ok || v != stamp(s) {
			return nil, ErrSnapshot
		}
	}
	raw, err := c.call(ctx, "textDocument/definition", map[string]any{"textDocument": map[string]string{"uri": uri}, "position": p})
	if err != nil {
		return nil, err
	}
	type wirePosition struct {
		Line      *int `json:"line"`
		Character *int `json:"character"`
	}
	type wireRange struct {
		Start *wirePosition `json:"start"`
		End   *wirePosition `json:"end"`
	}
	type wireLocation struct {
		URI   string     `json:"uri"`
		Range *wireRange `json:"range"`
	}
	var rows []wireLocation
	if string(raw) != "null" {
		if json.Unmarshal(raw, &rows) != nil {
			var row wireLocation
			if json.Unmarshal(raw, &row) != nil {
				return nil, ErrProtocol
			}
			rows = []wireLocation{row}
		}
	}
	if len(rows) > 256 {
		return nil, ErrProtocol
	}
	out := make([]Location, 0, len(rows))
	for _, row := range rows {
		if row.Range == nil || row.Range.Start == nil || row.Range.End == nil || row.Range.Start.Line == nil || row.Range.Start.Character == nil || row.Range.End.Line == nil || row.Range.End.Character == nil {
			return nil, ErrProtocol
		}
		span := Range{Position{*row.Range.Start.Line, *row.Range.Start.Character}, Position{*row.Range.End.Line, *row.Range.End.Character}}
		target, e := c.snapshot(row.URI)
		if e != nil || !target.Overlay || !validRange(target.Text, span) {
			return nil, ErrSnapshot
		}
		if target.Overlay {
			if v, ok := c.versions[row.URI]; !ok || v != stamp(target) {
				return nil, ErrSnapshot
			}
		}
		out = append(out, Location{target, span})
	}
	if !c.config.Current(s) {
		return nil, ErrSnapshot
	}
	for _, row := range out {
		if !c.config.Current(row.Snapshot) {
			return nil, ErrSnapshot
		}
	}
	return out, nil
}
func (c *Client) Shutdown(ctx context.Context) error {
	if err := c.acquire(ctx); err != nil {
		c.cancel()
		<-c.done
		return err
	}
	defer c.release()
	_, err := c.call(ctx, "shutdown", nil)
	if err == nil {
		err = c.notify(ctx, "exit", nil)
	}
	c.cancel()
	<-c.done
	if !c.CleanupProven() {
		return ErrSession
	}
	return err
}
func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, RequestLimit)
	defer cancel()
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan response, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	if err := c.send(ctx, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	select {
	case r := <-ch:
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return r.body, r.err
	case <-c.done:
		return nil, ErrSession
	case <-ctx.Done():
		cancelCtx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_ = c.notify(cancelCtx, "$/cancelRequest", map[string]int{"id": id})
		return nil, ctx.Err()
	}
}
func (c *Client) notify(ctx context.Context, method string, params any) error {
	return c.send(ctx, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}
func (c *Client) send(ctx context.Context, v any) error {
	body, err := json.Marshal(v)
	if err != nil || len(body) > MaxMessageBytes {
		return ErrProtocol
	}
	c.sent += len(body)
	if c.sent > 32<<20 {
		c.cancel()
		return ErrSession
	}
	done := make(chan error, 1)
	go func() {
		_, e := fmt.Fprintf(c.writer, "Content-Length: %d\r\n\r\n", len(body))
		if e == nil {
			_, e = c.writer.Write(body)
		}
		done <- e
	}()
	select {
	case err := <-done:
		if err != nil {
			return ErrSession
		}
		return nil
	case <-ctx.Done():
		c.cancel()
		<-done
		return ctx.Err()
	case <-c.done:
		<-done
		return ErrSession
	}
}
func (c *Client) readLoop(r io.Reader) error {
	br := bufio.NewReaderSize(r, 4096)
	for {
		raw, err := readFrame(br)
		if err != nil {
			return ErrProtocol
		}
		var m envelope
		if json.Unmarshal(raw, &m) != nil || m.JSONRPC != "2.0" {
			return ErrProtocol
		}
		// No dynamic registration/configuration callbacks are advertised.
		if m.Method != "" {
			if len(m.ID) > 0 {
				return ErrUnsupported
			}
			continue
		}
		var id int
		if json.Unmarshal(m.ID, &id) != nil || id <= 0 {
			return ErrProtocol
		}
		c.mu.Lock()
		ch := c.pending[id]
		c.mu.Unlock()
		if ch == nil {
			continue
		}
		result := response{body: m.Result}
		if len(m.Error) > 0 && string(m.Error) != "null" {
			result.err = ErrSession
		}
		select {
		case ch <- result:
		default:
			return ErrProtocol
		}
	}
}
func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	size := 0
	for {
		chunk, err := r.ReadSlice('\n')
		line := string(chunk)
		size += len(line)
		if err != nil || size > 4096 {
			return nil, ErrProtocol
		}
		if line == "\r\n" {
			break
		}
		if !strings.HasSuffix(line, "\r\n") {
			return nil, ErrProtocol
		}
		name, value, ok := strings.Cut(strings.TrimSuffix(line, "\r\n"), ":")
		if !ok {
			return nil, ErrProtocol
		}
		if strings.EqualFold(name, "Content-Length") {
			if length >= 0 {
				return nil, ErrProtocol
			}
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil || length < 0 || length > MaxMessageBytes {
				return nil, ErrProtocol
			}
		}
	}
	if length < 0 {
		return nil, ErrProtocol
	}
	b := make([]byte, length)
	_, err := io.ReadFull(r, b)
	return b, err
}
func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}
func validURI(root, uri string) bool {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || u.Opaque != "" || strings.ContainsRune(u.Path, 0) {
		return false
	}
	p := filepath.FromSlash(u.Path)
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && filepath.IsAbs(p) && filepath.Clean(p) == p && fileURI(p) == uri
}
func validPosition(text string, p Position) bool {
	if p.Line < 0 || p.Character < 0 {
		return false
	}
	lines := strings.Split(text, "\n")
	if p.Line >= len(lines) {
		return false
	}
	line := strings.TrimSuffix(lines[p.Line], "\r")
	n := 0
	for _, r := range line {
		if n == p.Character {
			return true
		}
		n += utf16.RuneLen(r)
	}
	return n == p.Character
}
func validRange(text string, r Range) bool {
	return validPosition(text, r.Start) && validPosition(text, r.End) && (r.Start.Line < r.End.Line || r.Start.Line == r.End.Line && r.Start.Character <= r.End.Character)
}
func environment(cache string) []string {
	env := []string{"GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off", "GOPLSCACHE=" + cache, "GOCACHE=" + filepath.Join(cache, "go-build"), "LANG=C", "LC_ALL=C"}
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "GOROOT", "GOPATH", "GOMODCACHE"} {
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
		}
	}
	return env
}
