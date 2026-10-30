package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/corpusindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// assertListenerRetired fails with what while the listener run opened still
// accepts, or while a server at address answers with this run's digest.
// Ownership, not the port number, decides retirement: the self-hosted CI
// runners share one kernel whose ephemeral range holds 4096 ports, so a
// co-tenant can bind the freed port and answer before this probe (V1-2626).
// WSL2 mirrored networking also accepts some dials to a just-closed loopback
// port and resets them without a byte. The probe still catches the owned
// server answering through any socket, because only it serves digest.
func assertListenerRetired(t *testing.T, owned net.Listener, address, digest, what string) {
	t.Helper()
	tcp, ok := owned.(*net.TCPListener)
	if !ok {
		t.Fatalf("%s: owned listener %T", what, owned)
	}
	if err := tcp.SetDeadline(time.Now()); !errors.Is(err, net.ErrClosed) {
		t.Fatal(what, "(owned listener still open)", err)
	}
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		return
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(conn, "GET /capabilities HTTP/1.0\r\nHost: localhost\r\n\r\n"); err != nil {
		return
	}
	response, _ := io.ReadAll(conn)
	if bytes.Contains(response, []byte(digest)) {
		t.Fatal(what, "(the owned server still answers)")
	}
}

// ownListener records the listener each run opens.
func ownListener(t *testing.T) <-chan net.Listener {
	t.Helper()
	owned := make(chan net.Listener, 1)
	t.Cleanup(func() { listen = net.Listen })
	listen = func(network, address string) (net.Listener, error) {
		l, err := net.Listen(network, address)
		if err == nil {
			owned <- l
		}
		return l, err
	}
	return owned
}

// uniqueRevision keeps each run's artifact digest distinct from a co-tenant
// running the same test, so only this run's server can match the probe.
func uniqueRevision() string { return fmt.Sprintf("%040x", time.Now().UnixNano()) }

type listenerNotice struct{ line chan string }

func (w listenerNotice) Write(p []byte) (int, error) {
	select {
	case w.line <- string(p):
	default:
	}
	return len(p), nil
}

func TestCorpusHTTPActiveRequestCancellation(t *testing.T) {
	t.Run("DCP-V1-042 listener and admitted body retire on interruption", func(t *testing.T) {
		a := &doccorpus.Artifact{Schema: doccorpus.SchemaV2, Manifest: doccorpus.Manifest{Repository: doccorpus.Repository{ID: strings.Repeat("a", 40), Revision: uniqueRevision()}, BuiltAt: "2026-09-30T00:00:00Z"}, Capabilities: []doccorpus.Capability{{Name: "info", State: "present"}}}
		raw, err := doccorpus.Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		a.SHA256 = doccorpus.Digest(raw)
		index, err := doccorpus.BuildQueryIndex(context.Background(), a)
		if err != nil {
			t.Fatal(err)
		}
		embedded, err := corpusindex.Encode(corpusindex.Artifact{Schema: corpusindex.Schema, Corpus: a, Index: index, ProducerValidation: "source-rederived-at-producer; producer authentication NOT_OBSERVED"})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "index.json")
		if err = os.WriteFile(path, embedded, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		owned := ownListener(t)
		notice := listenerNotice{make(chan string, 8)}
		done := make(chan int, 1)
		go func() { done <- run(ctx, []string{"--index", path, "--sha256", doccorpus.Digest(embedded)}, notice) }()
		t.Cleanup(func() {
			cancel()
			select {
			case <-done:
			case <-time.After(7 * time.Second):
				t.Error("owned server survived cleanup")
			}
		})
		var address string
		select {
		case line := <-notice.line:
			parts := strings.Fields(line)
			if len(parts) < 6 {
				t.Fatal(line)
			}
			address = parts[5]
		case <-time.After(5 * time.Second):
			t.Fatal("listener not observed")
		}
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if err = conn.SetDeadline(time.Now().Add(12 * time.Second)); err != nil {
			t.Fatal(err)
		}
		_, err = io.WriteString(conn, "POST /query HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: 64\r\nExpect: 100-continue\r\n\r\n")
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "POST"})
		// 100 Continue is emitted when the admitted handler reads this body;
		// no sleep or mere successful TCP dial establishes active admission.
		if err != nil || response.StatusCode != 100 {
			t.Fatal("active body not admitted", response, err)
		}
		cancel()
		select {
		case code := <-done:
			if code != 0 {
				t.Fatal("shutdown failed", code)
			}
			done <- code
		case <-time.After(7 * time.Second):
			t.Fatal("active body outlived shutdown bound")
		}
		assertListenerRetired(t, <-owned, address, doccorpus.Digest(embedded), "listener survived cancellation")
		// Close aborts the body read after bounded graceful shutdown. A
		// refusal may have been written; eventually the connection must close.
		_, err = io.Copy(io.Discard, conn)
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			t.Fatal("active connection survived shutdown", err)
		}
	})
}
func TestCorpusHTTPListenerCancellation(t *testing.T) {
	t.Run("DCP-V1-042 owned listener retires on interruption", func(t *testing.T) {
		a := &doccorpus.Artifact{Schema: doccorpus.SchemaV2, Manifest: doccorpus.Manifest{Repository: doccorpus.Repository{ID: strings.Repeat("a", 40), Revision: uniqueRevision()}, BuiltAt: "2026-09-30T00:00:00Z"}, Capabilities: []doccorpus.Capability{{Name: "info", State: "present"}}}
		raw, e := doccorpus.Encode(a)
		if e != nil {
			t.Fatal(e)
		}
		a.SHA256 = doccorpus.Digest(raw)
		index, e := doccorpus.BuildQueryIndex(context.Background(), a)
		if e != nil {
			t.Fatal(e)
		}
		embedded, e := corpusindex.Encode(corpusindex.Artifact{Schema: corpusindex.Schema, Corpus: a, Index: index, ProducerValidation: "source-rederived-at-producer; producer authentication NOT_OBSERVED"})
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "index.json")
		if e = os.WriteFile(path, embedded, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		owned := ownListener(t)
		notice := listenerNotice{make(chan string, 8)}
		done := make(chan int, 1)
		go func() { done <- run(ctx, []string{"--index", path, "--sha256", doccorpus.Digest(embedded)}, notice) }()
		var address string
		select {
		case line := <-notice.line:
			parts := strings.Fields(line)
			if len(parts) < 6 {
				t.Fatal(line)
			}
			address = parts[5]
		case <-time.After(5 * time.Second):
			t.Fatal("server failed to start")
		}
		conn, e := net.DialTimeout("tcp", address, time.Second)
		if e != nil {
			t.Fatal(e)
		}
		conn.Close()
		cancel()
		select {
		case code := <-done:
			if code != 0 {
				t.Fatal(code)
			}
		case <-time.After(7 * time.Second):
			t.Fatal("server survived cancellation")
		}
		assertListenerRetired(t, <-owned, address, doccorpus.Digest(embedded), "listener survived shutdown")
		if code := run(context.Background(), []string{"--index", path, "--sha256", doccorpus.Digest(embedded), "--listen", "0.0.0.0:0"}, notice); code != 2 {
			t.Fatal("implicit remote exposure accepted")
		}
	})
}
