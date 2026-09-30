// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/goplsclient"
	"github.com/Beamfall/corvint/internal/lspsnapshot"
)

// SemanticConfig is operator-owned admission, never client-provided execution configuration.
type SemanticConfig struct {
	Executable, Root    string
	WorkspaceDriftGuard bool
}
type overlayStore struct {
	mu      sync.Mutex
	session *lspsnapshot.Session
	next    uint64
	docs    map[string]captured
}
type captured struct {
	overlay  lspsnapshot.Overlay
	snapshot goplsclient.Snapshot
}

func (s *overlayStore) snapshot(uri string) (goplsclient.Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.docs[uri]
	return c.snapshot, ok
}
func (s *overlayStore) current(v goplsclient.Snapshot) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.docs[v.URI]
	return ok && c.snapshot == v && s.session.IsCurrent(c.overlay)
}
func (s *overlayStore) put(uri string, version int64, text string, open bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if version < 0 || version > 2147483647 || s.next == math.MaxUint64 {
		return lspsnapshot.ErrBounds
	}
	var o lspsnapshot.Overlay
	var err error
	if open {
		o, err = s.session.Open(uri, version, text)
	} else {
		o, err = s.session.Change(uri, version, text)
	}
	if err != nil {
		return err
	}
	s.next++
	s.docs[uri] = captured{o, goplsclient.Snapshot{URI: uri, Identity: strconv.FormatUint(s.next, 10), Text: o.Content(), Version: int(version), Overlay: true}}
	return nil
}
func (s *overlayStore) close(uri string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.session.Close(uri); err != nil {
		return err
	}
	delete(s.docs, uri)
	return nil
}

// convertPosition rejects mid-codepoint and mid-surrogate coordinates rather than rounding.
func convertPosition(text string, p goplsclient.Position, from, to string) (goplsclient.Position, error) {
	if p.Line < 0 || p.Character < 0 || !utf8.ValidString(text) {
		return p, ErrFraming
	}
	lines := strings.Split(text, "\n")
	if p.Line >= len(lines) {
		return p, ErrFraming
	}
	line := strings.TrimSuffix(lines[p.Line], "\r")
	width := func(r rune, e string) int {
		switch e {
		case "utf-8":
			return utf8.RuneLen(r)
		case "utf-16":
			if r > 0xffff {
				return 2
			}
			return 1
		case "utf-32":
			return 1
		}
		return -1
	}
	a, b := 0, 0
	for _, r := range line {
		if a == p.Character {
			return goplsclient.Position{Line: p.Line, Character: b}, nil
		}
		x, y := width(r, from), width(r, to)
		if x < 0 || y < 0 {
			return p, ErrFraming
		}
		a += x
		b += y
		if a > p.Character {
			return p, ErrFraming
		}
	}
	if a == p.Character {
		return goplsclient.Position{Line: p.Line, Character: b}, nil
	}
	return p, ErrFraming
}

type semanticFrame struct {
	body []byte
	err  error
}
type semanticResult struct {
	id             json.RawMessage
	rows           []goplsclient.Location
	source         goplsclient.Snapshot
	err            error
	core           json.RawMessage
	contextRequest bool
	request        context.Context
}

// ServeSemantic owns streams, a bounded gopls session and one pending request.
// Definitions observe open overlays; context preserves separately bound Core
// evidence and never promotes the overlay observation to Git authority.
func ServeSemantic(parent context.Context, in io.ReadCloser, out io.WriteCloser, config SemanticConfig) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	frames := make(chan semanticFrame)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		r := bufio.NewReaderSize(in, maxHeader+1)
		for n := 0; n < maxFrames; n++ {
			b, e := readFrame(r)
			if e != nil {
				cancel()
			}
			select {
			case frames <- semanticFrame{b, e}:
			case <-ctx.Done():
				return
			}
			if e != nil {
				return
			}
		}
		select {
		case frames <- semanticFrame{err: ErrLimit}:
		case <-ctx.Done():
		}
	}()
	stopIO := make(chan struct{})
	ioStopped := make(chan struct{})
	go func() {
		defer close(ioStopped)
		select {
		case <-ctx.Done():
			in.Close()
			out.Close()
		case <-stopIO:
		}
	}()
	var client *goplsclient.Client
	var backendDone <-chan struct{}
	stopBackendWatch := func() {}
	var pending context.CancelFunc
	var result chan semanticResult
	defer func() {
		cancel()
		stopBackendWatch()
		in.Close()
		out.Close()
		if pending != nil {
			pending()
			<-result
		}
		if client != nil {
			<-client.Done()
		}
		<-readerDone
		close(stopIO)
		<-ioStopped
	}()
	worker, err := captureWorkerIdentity()
	if err != nil {
		return err
	}
	rootURI := (&url.URL{Scheme: "file", Path: config.Root}).String()
	var sessionBytes [16]byte
	if _, err := rand.Read(sessionBytes[:]); err != nil {
		return err
	}
	sessionID := hex.EncodeToString(sessionBytes[:])
	session, err := lspsnapshot.NewSession(sessionID, rootURI, lspsnapshot.Bounds{Documents: 32, DocumentBytes: 256 << 10, TotalBytes: 8 << 20})
	if err != nil {
		return err
	}
	store := &overlayStore{session: session, docs: map[string]captured{}}
	var guard *workspaceGuard
	var observationRemaining time.Duration
	var pendingID json.RawMessage
	state := 0
	encoding := "utf-16"
	invalidated := false
	invalidateCaptures := func() error {
		if invalidated {
			return nil
		}
		store.mu.Lock()
		defer store.mu.Unlock()
		if err := store.session.Reset(rootURI); err != nil {
			return err
		}
		store.docs = map[string]captured{}
		invalidated = true
		return nil
	}
	finish := func(r semanticResult, cancelled bool) error {
		if r.contextRequest {
			if r.request.Err() != nil {
				r.err = r.request.Err()
			}
			return contextResponse(out, r.id, r.core, r.source, sessionID, r.err, store.current(r.source), cancelled)
		}
		if guard != nil && !cancelled && !guard.stale {
			_ = guard.check(ctx, config.Root, &observationRemaining)
		}
		code, msg := 0, ""
		var wire any
		if guard != nil && guard.stale {
			code, msg = -32801, "Overlay definition unavailable"
			if err := invalidateCaptures(); err != nil {
				return err
			}
		} else if cancelled || errors.Is(r.err, context.Canceled) {
			code, msg = -32800, "Request cancelled"
		} else if r.err != nil || !store.current(r.source) {
			code, msg = -32801, "Overlay definition unavailable"
		} else {
			rows := make([]map[string]any, 0, len(r.rows))
			for _, loc := range r.rows {
				if !store.current(loc.Snapshot) {
					code, msg = -32801, "Overlay definition unavailable"
					break
				}
				a, e := convertPosition(loc.Snapshot.Text, loc.Range.Start, "utf-16", encoding)
				b, f := convertPosition(loc.Snapshot.Text, loc.Range.End, "utf-16", encoding)
				if e != nil || f != nil {
					code, msg = -32801, "Overlay definition unavailable"
					break
				}
				rows = append(rows, map[string]any{"uri": loc.Snapshot.URI, "range": goplsclient.Range{Start: a, End: b}})
			}
			wire = rows
		}
		return reply(out, r.id, wire, code, msg)
	}
	cancelPending := func() error {
		if pending == nil {
			return nil
		}
		pending()
		r := <-result
		pending = nil
		result = nil
		return finish(r, true)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-backendDone:
			return goplsclient.ErrSession
		case r := <-result:
			stop := pending
			pending = nil
			result = nil
			if err := finish(r, false); err != nil {
				stop()
				return err
			}
			stop()
		case f := <-frames:
			if f.err == io.EOF {
				return nil
			}
			if f.err != nil {
				return f.err
			}
			var m map[string]json.RawMessage
			var method, version string
			if !utf8.Valid(f.body) || json.Unmarshal(f.body, &m) != nil || m == nil {
				if err := reply(out, nil, nil, -32700, "Parse error"); err != nil {
					return err
				}
				continue
			}
			json.Unmarshal(m["method"], &method)
			json.Unmarshal(m["jsonrpc"], &version)
			id, hasID := m["id"]
			if version != "2.0" || method == "" || (hasID && !validID(id)) {
				if err := reply(out, nil, nil, -32600, "Invalid Request"); err != nil {
					return err
				}
				continue
			}
			if method == "corvint/context" && hasID && len(id) > 4096 {
				if err := reply(out, nil, nil, -32602, "Invalid params"); err != nil {
					return err
				}
				continue
			}
			if !hasID {
				if method == "exit" {
					if state == 3 {
						return nil
					}
					return ErrExit
				}
				if method == "initialized" && state == 1 {
					state = 2
					continue
				}
				if method == "$/cancelRequest" {
					var p struct {
						ID json.RawMessage `json:"id"`
					}
					json.Unmarshal(m["params"], &p)
					if pending != nil && string(p.ID) == string(pendingID) {
						if err := cancelPending(); err != nil {
							return err
						}
					}
					continue
				}
				if method == "textDocument/didSave" && guard != nil {
					var p struct {
						TextDocument *struct {
							URI *string `json:"uri"`
						} `json:"textDocument"`
					}
					if jsonv2.Unmarshal(m["params"], &p, jsonv2.RejectUnknownMembers(true)) != nil || p.TextDocument == nil || p.TextDocument.URI == nil {
						return ErrFraming
					}
					if _, ok := store.snapshot(*p.TextDocument.URI); !ok {
						return ErrFraming
					}
					guard.stale = true
					if err := cancelPending(); err != nil {
						return err
					}
					if err := invalidateCaptures(); err != nil {
						return err
					}
					continue
				}
				if method == "textDocument/didOpen" || method == "textDocument/didChange" || method == "textDocument/didClose" {
					if state != 2 {
						return ErrFraming
					}
					if err := cancelPending(); err != nil {
						return err
					}
					var p struct {
						TextDocument struct {
							URI        string  `json:"uri"`
							LanguageID string  `json:"languageId"`
							Version    *int64  `json:"version"`
							Text       *string `json:"text"`
						} `json:"textDocument"`
						Changes []struct {
							Text        *string         `json:"text"`
							Range       json.RawMessage `json:"range"`
							RangeLength json.RawMessage `json:"rangeLength"`
						} `json:"contentChanges"`
					}
					if json.Unmarshal(m["params"], &p) != nil {
						return ErrFraming
					}
					d := p.TextDocument
					switch method {
					case "textDocument/didOpen":
						if d.Version == nil || d.Text == nil || d.LanguageID != "go" {
							return ErrFraming
						}
						err = store.put(d.URI, *d.Version, *d.Text, true)
						if err == nil {
							err = client.Open(ctx, d.URI)
						}
					case "textDocument/didChange":
						if d.Version == nil || len(p.Changes) != 1 || p.Changes[0].Text == nil || p.Changes[0].Range != nil || p.Changes[0].RangeLength != nil {
							return ErrFraming
						}
						err = store.put(d.URI, *d.Version, *p.Changes[0].Text, false)
						if err == nil {
							err = client.Change(ctx, d.URI)
						}
					case "textDocument/didClose":
						err = store.close(d.URI)
						if err == nil {
							err = client.CloseDocument(ctx, d.URI)
						}
					default:
						continue
					}
					if err != nil {
						return err
					}
				}
				continue
			}
			code, msg := 0, ""
			var response any
			switch {
			case state == 3:
				code, msg = -32600, "Invalid Request"
			case method == "initialize" && state == 0:
				n, e := initialize(m["params"])
				if e != nil || len(n.roots) != 1 || n.roots[0] != rootURI {
					code, msg = -32602, "Invalid params"
					break
				}
				if config.WorkspaceDriftGuard {
					observeCtx, stop := context.WithTimeout(ctx, workspaceAdmissionBudget)
					baseline, observeErr := admitWorkspace(observeCtx, config.Root)
					stop()
					if observeErr != nil {
						code, msg = -32001, "Backend unavailable"
						break
					}
					guard = &workspaceGuard{baseline: baseline}
				}
				client, e = goplsclient.Start(ctx, goplsclient.Config{Executable: config.Executable, Root: config.Root, WorkspaceOnlyGuard: config.WorkspaceDriftGuard, Snapshot: store.snapshot, Current: store.current})
				if e != nil {
					code, msg = -32001, "Backend unavailable"
					break
				}
				backendDone = client.Done()
				// A reply can block the dispatcher under editor backpressure.
				// Observe backend death independently so it closes owned I/O.
				watchStop, watchDone := make(chan struct{}), make(chan struct{})
				done := client.Done()
				go func() {
					defer close(watchDone)
					select {
					case <-done:
						cancel()
					case <-ctx.Done():
					case <-watchStop:
					}
				}()
				stopBackendWatch = func() { close(watchStop); <-watchDone; stopBackendWatch = func() {} }
				encoding = n.encoding
				state = 1
				response = map[string]any{"capabilities": map[string]any{"positionEncoding": encoding, "textDocumentSync": map[string]any{"openClose": true, "change": 1}, "experimental": map[string]any{"corvintDefinitionProbe": true, "corvintContext": map[string]string{"method": "corvint/context", "schema": "corvint-editor-context/0"}}}, "serverInfo": map[string]string{"name": "corvint-lsp-experimental", "version": "0"}}
				if guard != nil {
					caps := response.(map[string]any)["capabilities"].(map[string]any)
					caps["experimental"].(map[string]any)["corvintWorkspaceDriftGuard"] = map[string]any{"profile": "whole-root-observation/0", "scope": "workspace-only", "externalInputsPinned": false}
					caps["textDocumentSync"].(map[string]any)["save"] = map[string]any{"includeText": false}
				}

			case method == "initialize":
				code, msg = -32600, "Invalid Request"
			case state == 0 || state == 1:
				code, msg = -32002, "Server not initialized"
			case method == "shutdown":
				if err := cancelPending(); err != nil {
					return err
				}
				// Expected backend shutdown must not race its normal reply by
				// closing stdout. Disarm and join before asking it to exit.
				stopBackendWatch()
				backendDone = nil
				if err := client.Shutdown(ctx); err != nil {
					return err
				}
				state = 3
			case method == "corvint/context":
				if pending != nil {
					code, msg = -32000, "Request already pending"
					break
				}
				input, uri, e := parseContextParams(m["params"], config.Root)
				source, ok := store.snapshot(uri)
				if e != nil || !ok {
					code, msg = -32602, "Invalid params"
					break
				}
				request, stop := context.WithTimeout(ctx, contextBudget)
				pending = stop
				pendingID = append(json.RawMessage(nil), id...)
				result = make(chan semanticResult, 1)
				ch := result
				go func() {
					core, e := editorContext(request, worker, config.Root, input)
					ch <- semanticResult{id: id, source: source, err: e, core: core, contextRequest: true, request: request}
				}()
				continue
			case method == "textDocument/definition":
				if guard != nil && guard.stale {
					code, msg = -32801, "Overlay definition unavailable"
					break
				}
				if pending != nil {
					code, msg = -32001, "Request already pending"
					break
				}
				var p struct {
					TextDocument struct {
						URI string `json:"uri"`
					} `json:"textDocument"`
					Position *struct {
						Line      *int `json:"line"`
						Character *int `json:"character"`
					} `json:"position"`
				}
				if json.Unmarshal(m["params"], &p) != nil || p.Position == nil || p.Position.Line == nil || p.Position.Character == nil {
					code, msg = -32602, "Invalid params"
					break
				}
				s, ok := store.snapshot(p.TextDocument.URI)
				pos, e := convertPosition(s.Text, goplsclient.Position{Line: *p.Position.Line, Character: *p.Position.Character}, encoding, "utf-16")
				if !ok || e != nil {
					code, msg = -32602, "Invalid params"
					break
				}
				if guard != nil {
					observationRemaining = workspaceObservationBudget
					if err := guard.check(ctx, config.Root, &observationRemaining); err != nil {
						if err := invalidateCaptures(); err != nil {
							return err
						}
						code, msg = -32801, "Overlay definition unavailable"
						break
					}
				}
				request, stop := context.WithTimeout(ctx, 20*time.Second)
				pending = stop
				pendingID = append(json.RawMessage(nil), id...)
				result = make(chan semanticResult, 1)
				ch := result
				go func() {
					rows, e := client.Definition(request, s.URI, pos)
					ch <- semanticResult{id: id, rows: rows, source: s, err: e}
				}()
				continue
			default:
				code, msg = -32601, "Method not found"
			}
			if err := reply(out, id, response, code, msg); err != nil {
				return err
			}
		}
	}
}
