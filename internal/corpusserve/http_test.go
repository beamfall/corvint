package corpusserve

import (
	"bytes"
	"context"
	"github.com/Beamfall/corvint/internal/corpusindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func portable(t *testing.T) *corpusindex.Reader {
	t.Helper()
	root := t.TempDir()
	data := []byte("# Checkout\n\nDeclared checkout documentation.\n")
	if e := os.WriteFile(filepath.Join(root, "docs.md"), data, 0600); e != nil {
		t.Fatal(e)
	}
	git := func(args ...string) string {
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	git("add", "docs.md")
	git("-c", "user.name=test", "-c", "user.email=test@invalid", "commit", "-qm", "docs")
	m, e := doccorpus.Inventory(context.Background(), root, git("rev-parse", "HEAD"), "docs.md", "2026-09-30T00:00:00Z")
	if e != nil {
		t.Fatal(e)
	}
	m.Schema = doccorpus.ManifestSchemaV2
	a, e := doccorpus.Build(context.Background(), root, m)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := doccorpus.Encode(a)
	if e != nil {
		t.Fatal(e)
	}
	embedded, e := corpusindex.Build(context.Background(), root, raw)
	if e != nil {
		t.Fatal(e)
	}
	r, e := corpusindex.Open(context.Background(), embedded, doccorpus.Digest(embedded))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.RemoveAll(root); e != nil {
		t.Fatal(e)
	}
	return r
}
func TestHostedCorpusHTTPConformance(t *testing.T) {
	t.Run("DCP-V1-042 embedded only bounded read transport", func(t *testing.T) {
		r := portable(t)
		server := httptest.NewServer(Handler(r))
		defer server.Close()
		response, e := server.Client().Post(server.URL+"/query", "application/json", strings.NewReader(`{"operation":"search","query":"checkout"}`))
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(response.Body)
		response.Body.Close()
		if e != nil || response.StatusCode != 200 {
			t.Fatal(response.StatusCode, e, string(data))
		}
		native, e := r.Query(context.Background(), doccorpus.Request{Operation: "search", Query: "checkout"})
		if e != nil {
			t.Fatal(e)
		}
		wanted, e := doccorpus.Encode(native)
		if e != nil || !bytes.Equal(data, wanted) {
			t.Fatal("HTTP/native semantic receipt parity lost")
		}
		for _, test := range []struct {
			method, path, body string
			status             int
		}{{"POST", "/query", `{"operation":"search","query":"checkout","root":"/tmp"}`, 400}, {"POST", "/query", strings.Repeat("x", (16<<10)+1), 413}, {"GET", "/query", "", 405}, {"POST", "/write", "{}", 404}, {"POST", "/query", `{"operation":"shell"}`, 400}} {
			req, e := http.NewRequest(test.method, server.URL+test.path, strings.NewReader(test.body))
			if e != nil {
				t.Fatal(e)
			}
			req.Header.Set("Content-Type", "application/json")
			response, e := server.Client().Do(req)
			if e != nil {
				t.Fatal(e)
			}
			data, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode != test.status || !bytes.Contains(data, []byte("trust_envelope")) {
				t.Fatal("refusal boundary", response.StatusCode, string(data))
			}
		}
	})
}

type blockedBody struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockedBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered) })
	<-b.release
	return 0, io.EOF
}
func (b *blockedBody) Close() error { return nil }
func TestHostedCorpusConcurrencyAndCleanup(t *testing.T) {
	t.Run("DCP-V1-042 overload and handler retirement", func(t *testing.T) {
		h := Handler(portable(t))
		done := make(chan struct{}, MaxConcurrent)
		bodies := []*blockedBody{}
		for i := 0; i < MaxConcurrent; i++ {
			b := &blockedBody{entered: make(chan struct{}), release: make(chan struct{})}
			bodies = append(bodies, b)
			req := httptest.NewRequest("POST", "/query", b)
			req.Header.Set("Content-Type", "application/json")
			go func() { h.ServeHTTP(httptest.NewRecorder(), req); done <- struct{}{} }()
			select {
			case <-b.entered:
			case <-time.After(time.Second):
				t.Fatal("handler admission hung")
			}
		}
		over := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/query", strings.NewReader(`{"operation":"info"}`))
		req.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(over, req)
		if over.Code != 503 {
			t.Fatal("concurrency bound not enforced")
		}
		for _, b := range bodies {
			close(b.release)
		}
		for range bodies {
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("owned handler survived release")
			}
		}
		after := httptest.NewRecorder()
		h.ServeHTTP(after, req)
		if after.Code != 200 {
			t.Fatal("slots not retired", after.Code)
		}
	})
}

func TestHostedCorpusConfiguredTimeouts(t *testing.T) {
	t.Run("DCP-V1-042 header body and output timeouts", func(t *testing.T) {
		s := NewServer("127.0.0.1:0", portable(t))
		if s.ReadHeaderTimeout != 5*time.Second || s.ReadTimeout != 10*time.Second || s.WriteTimeout != 10*time.Second || s.IdleTimeout != 30*time.Second || s.MaxHeaderBytes != 16<<10 {
			t.Fatal("bounded server policy changed", s)
		}
	})
}
