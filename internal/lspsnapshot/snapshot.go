// Package lspsnapshot implements experimental, in-memory LSP snapshot identities.
// It performs no filesystem access or Git verification. Callers must verify Git
// object existence separately; an admitted identity is not evidence of contents.
package lspsnapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"path"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	ErrIdentity = errors.New("invalid snapshot identity")
	ErrBounds   = errors.New("overlay resource bound exceeded")
	ErrState    = errors.New("document state does not admit operation")
	ErrStale    = errors.New("stale document version or snapshot")
)

// GitIdentity identifies immutable content, never an unsaved document.
// Repository is the caller's stable repository identity; RootURI is lexical,
// not symlink-resolved. Object IDs must be lowercase hexadecimal of Format.
type GitIdentity struct{ Repository, RootURI, Format, Commit, Tree, Blob, Path string }

func (g GitIdentity) Validate() error {
	n := 40
	if g.Format == "sha256" {
		n = 64
	} else if g.Format != "sha1" {
		return ErrIdentity
	}
	if g.Repository == "" || len(g.Repository) > 4096 || len(g.Path) > 4096 || !canonicalURI(g.RootURI) || g.Path == "" || strings.HasPrefix(g.Path, "/") || g.Path == "." || g.Path == ".." || strings.HasPrefix(g.Path, "../") || path.Clean(g.Path) != g.Path || strings.ContainsAny(g.Path, "\\\x00") {
		return ErrIdentity
	}
	for _, id := range []string{g.Commit, g.Tree, g.Blob} {
		if len(id) != n || strings.ToLower(id) != id {
			return ErrIdentity
		}
		if _, err := hex.DecodeString(id); err != nil {
			return ErrIdentity
		}
	}
	return nil
}

// Bounds are caller-chosen experimental ceilings, not qualified profile limits.
type Bounds struct{ Documents, DocumentBytes, TotalBytes int }

// Overlay is a captured immutable value. Its private fields cannot be relabelled
// as Git evidence; Content returns the client-supplied string without disk reads.
type Overlay struct {
	session                          *Session
	sessionID, rootURI, uri, content string
	epoch, generation                uint64
	version                          int64
	digest                           [32]byte
}

func (o Overlay) SessionID() string { return o.sessionID }
func (o Overlay) RootURI() string   { return o.rootURI }
func (o Overlay) URI() string       { return o.uri }
func (o Overlay) Version() int64    { return o.version }
func (o Overlay) Digest() [32]byte  { return o.digest }
func (o Overlay) Content() string   { return o.content }

// Session serializes updates; captures remain immutable after later updates.
// A session has one root; multi-root consumers allocate one Session per root.
type Session struct {
	mu                sync.Mutex
	id, root          string
	epoch, generation uint64
	bounds            Bounds
	bytes             int
	documents         map[string]Overlay
}

func NewSession(id, rootURI string, b Bounds) (*Session, error) {
	if id == "" || len(id) > 128 || !canonicalURI(rootURI) {
		return nil, ErrIdentity
	}
	if b.Documents <= 0 || b.DocumentBytes <= 0 || b.TotalBytes <= 0 || b.DocumentBytes > b.TotalBytes {
		return nil, ErrBounds
	}
	return &Session{id: strings.Clone(id), root: strings.Clone(rootURI), bounds: b, documents: make(map[string]Overlay)}, nil
}

func canonicalURI(raw string) bool {
	if len(raw) > 4096 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || !strings.HasPrefix(u.Path, "/") || strings.ContainsAny(u.Path, "\\\x00") || !utf8.ValidString(u.Path) || path.Clean(u.Path) != u.Path {
		return false
	}
	// One spelling per decoded path prevents escaped separators and aliases.
	return (&url.URL{Scheme: "file", Path: u.Path}).String() == raw
}
func (s *Session) inRoot(uri string) bool {
	if !canonicalURI(uri) {
		return false
	}
	u, _ := url.Parse(uri)
	r, _ := url.Parse(s.root)
	return u.Path != r.Path && strings.HasPrefix(u.Path, strings.TrimSuffix(r.Path, "/")+"/")
}

func (s *Session) Open(uri string, version int64, content string) (Overlay, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.put(uri, version, content, true)
}
func (s *Session) Change(uri string, version int64, content string) (Overlay, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.put(uri, version, content, false)
}
func (s *Session) put(uri string, version int64, content string, opening bool) (Overlay, error) {
	if !s.inRoot(uri) || version < 0 || !utf8.ValidString(content) {
		return Overlay{}, ErrIdentity
	}
	old, exists := s.documents[uri]
	if opening == exists {
		return Overlay{}, ErrState
	}
	if exists && version <= old.version {
		return Overlay{}, ErrStale
	}
	if len(content) > s.bounds.DocumentBytes || len(content) > s.bounds.TotalBytes-(s.bytes-len(old.content)) || (!exists && len(s.documents) >= s.bounds.Documents) {
		return Overlay{}, ErrBounds
	}
	generation := old.generation
	if opening {
		if s.generation == ^uint64(0) {
			return Overlay{}, ErrBounds
		}
		s.generation++
		generation = s.generation
	}
	o := Overlay{session: s, sessionID: s.id, rootURI: s.root, uri: strings.Clone(uri), content: strings.Clone(content), epoch: s.epoch, generation: generation, version: version, digest: sha256.Sum256([]byte(content))}
	s.bytes += len(content) - len(old.content)
	s.documents[o.uri] = o
	return o, nil
}
func (s *Session) Capture(uri string) (Overlay, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.documents[uri]
	if !ok {
		return Overlay{}, ErrState
	}
	return o, nil
}
func (s *Session) Close(uri string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.documents[uri]
	if !ok {
		return ErrState
	}
	delete(s.documents, uri)
	s.bytes -= len(o.content)
	return nil
}
func (s *Session) IsCurrent(o Overlay) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.documents[o.uri]
	return ok && o.session == s && current.epoch == o.epoch && current.generation == o.generation && current.version == o.version && current.digest == o.digest
}

// Reset invalidates all overlays on branch, root or externally observed save /
// rename changes. Callers must signal these events; this package does not watch
// Git or disk. Unrelated immutable GitIdentity values remain intact.
func (s *Session) Reset(rootURI string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !canonicalURI(rootURI) {
		return ErrIdentity
	}
	if s.epoch == ^uint64(0) {
		return ErrBounds
	}
	s.epoch++
	s.root = strings.Clone(rootURI)
	s.bytes = 0
	s.documents = make(map[string]Overlay)
	return nil
}
