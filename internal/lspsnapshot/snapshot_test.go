package lspsnapshot

import (
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

func session(t *testing.T) *Session {
	t.Helper()
	s, e := NewSession("one", "file:///repo", Bounds{2, 16, 20})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

// LSO-V0-001: syntactic Git admission confers no verified-evidence claim.
func TestGitIdentity(t *testing.T) {
	g := GitIdentity{"repo", "file:///repo", "sha1", strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40), "a.go"}
	if e := g.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"../a", "/a", "a/../b", "a\\b", "."} {
		bad := g
		bad.Path = p
		if bad.Validate() == nil {
			t.Fatal(p)
		}
	}
	g.Blob = "unsaved"
	if g.Validate() == nil {
		t.Fatal("accepted non-object")
	}
}

// LSO-V0-002: immutable full-text captures reject stale and duplicate updates.
func TestVersionsAndCapture(t *testing.T) {
	s := session(t)
	a, e := s.Open("file:///repo/a.go", 1, "α")
	if e != nil {
		t.Fatal(e)
	}
	if a.Digest() != sha256.Sum256([]byte("α")) {
		t.Fatal("digest")
	}
	if _, e = s.Change(a.URI(), 1, "x"); !errors.Is(e, ErrStale) {
		t.Fatal(e)
	}
	b, e := s.Change(a.URI(), 2, "new")
	if e != nil {
		t.Fatal(e)
	}
	if a.Content() != "α" || s.IsCurrent(a) || !s.IsCurrent(b) {
		t.Fatal("capture mutated or freshness broken")
	}
	if _, e = s.Open(a.URI(), 3, "x"); !errors.Is(e, ErrState) {
		t.Fatal(e)
	}
}

// LSO-V0-003: reuse of version/content across reopen/reset never revives a result.
func TestInvalidationAndIsolation(t *testing.T) {
	s := session(t)
	a, _ := s.Open("file:///repo/a.go", 1, "x")
	s.Close(a.URI())
	b, _ := s.Open(a.URI(), 1, "x")
	if s.IsCurrent(a) || !s.IsCurrent(b) {
		t.Fatal("reopen")
	}
	other := session(t)
	other.Open(a.URI(), 1, "x")
	if other.IsCurrent(b) {
		t.Fatal("session leak")
	}
	if e := s.Reset("file:///next"); e != nil {
		t.Fatal(e)
	}
	if s.IsCurrent(b) {
		t.Fatal("root drift")
	}
	if _, e := s.Open(a.URI(), 1, "x"); !errors.Is(e, ErrIdentity) {
		t.Fatal("old root admitted")
	}
	s.Reset("file:///repo")
	s.Open(a.URI(), 1, "x")
	if s.IsCurrent(b) {
		t.Fatal("epoch replay")
	}
}

// LSO-V0-004: aliases, root-prefix confusion and remote URIs fail closed.
func TestURIAdmission(t *testing.T) {
	s := session(t)
	for _, u := range []string{"file:///repository/a", "file:///repo/../a", "file:///repo/%61", "file:///repo/a%2Fb", "file://host/repo/a", "file:///repo/a?", "file:///repo/a#x", "file:///repo", "file:///repo/a/", "file:///repo/a%00", "https://example/a"} {
		if _, e := s.Open(u, 1, "x"); !errors.Is(e, ErrIdentity) {
			t.Fatalf("%s: %v", u, e)
		}
	}
	if _, e := s.Open("file:///repo/a%20b", 0, "x"); e != nil {
		t.Fatal(e)
	}
}

// LSO-V0-005: failed mutations preserve state and closing frees capacity.
func TestBounds(t *testing.T) {
	s := session(t)
	a, _ := s.Open("file:///repo/a", 1, strings.Repeat("a", 16))
	if _, e := s.Open("file:///repo/b", 1, "12345"); !errors.Is(e, ErrBounds) {
		t.Fatal(e)
	}
	b, _ := s.Open("file:///repo/b", 1, "1234")
	if _, e := s.Change(b.URI(), 2, "12345"); !errors.Is(e, ErrBounds) {
		t.Fatal(e)
	}
	if !s.IsCurrent(a) || !s.IsCurrent(b) {
		t.Fatal("failed update mutated state")
	}
	if _, e := s.Open("file:///repo/c", 1, ""); !errors.Is(e, ErrBounds) {
		t.Fatal(e)
	}
	s.Close(a.URI())
	if _, e := s.Change(b.URI(), 2, strings.Repeat("b", 16)); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Change(b.URI(), 3, strings.Repeat("b", 17)); !errors.Is(e, ErrBounds) {
		t.Fatal(e)
	}
}

// LSO-V0-002: concurrent out-of-order writers cannot replace the maximum version.
func TestConcurrentVersions(t *testing.T) {
	s := session(t)
	s.Open("file:///repo/a", 0, "")
	var wg sync.WaitGroup
	for i := int64(1); i <= 100; i++ {
		wg.Add(1)
		go func(v int64) {
			defer wg.Done()
			_, e := s.Change("file:///repo/a", v, "x")
			if e != nil && !errors.Is(e, ErrStale) {
				t.Error(e)
			}
		}(i)
	}
	wg.Wait()
	o, e := s.Capture("file:///repo/a")
	if e != nil || o.Version() != 100 {
		t.Fatal(o.Version(), e)
	}
}

// LSO-V0-005: tiny admitted strings must not retain large caller allocations.
func TestCallerBackingOwnership(t *testing.T) {
	oversized := func(value string) string { return (value + strings.Repeat("x", 1<<20))[:len(value)] }
	id, root, uri, content := oversized("one"), oversized("file:///repo"), oversized("file:///repo/a"), oversized("x")
	s, err := NewSession(id, root, Bounds{1, 1, 1})
	if err != nil {
		t.Fatal(err)
	}
	independent := func(name, stored, supplied string) {
		t.Helper()
		if stored != supplied {
			t.Errorf("%s changed bytes", name)
		}
		if unsafe.StringData(stored) == unsafe.StringData(supplied) {
			t.Errorf("%s retains caller backing allocation", name)
		}
	}
	independent("session id", s.id, id)
	independent("session root", s.root, root)
	for version := int64(0); version < 2; version++ {
		var o Overlay
		if version == 0 {
			o, err = s.Open(uri, version, content)
		} else {
			uri = oversized(uri)
			o, err = s.Change(uri, version, content)
		}
		if err != nil {
			t.Fatal(err)
		}
		independent("overlay URI", o.URI(), uri)
		independent("overlay content", o.Content(), content)
		for key := range s.documents {
			independent("map key", key, uri)
		}
	}
	nextRoot := oversized("file:///next")
	if err = s.Reset(nextRoot); err != nil {
		t.Fatal(err)
	}
	independent("reset root", s.root, nextRoot)
}

// LSO-V0-003: all public coordinates and generation may match in independent sessions.
func TestEqualSessionCoordinates(t *testing.T) {
	a, b := session(t), session(t)
	first, err := a.Open("file:///repo/a", 1, "x")
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.Open("file:///repo/a", 1, "x")
	if err != nil {
		t.Fatal(err)
	}
	if first.sessionID != second.sessionID || first.rootURI != second.rootURI || first.uri != second.uri || first.epoch != second.epoch || first.generation != second.generation || first.version != second.version || first.digest != second.digest {
		t.Fatal("test requires equal coordinates")
	}
	if b.IsCurrent(first) || a.IsCurrent(second) {
		t.Fatal("cross-session replay")
	}
	if !a.IsCurrent(first) || !b.IsCurrent(second) {
		t.Fatal("own capture rejected")
	}
}
