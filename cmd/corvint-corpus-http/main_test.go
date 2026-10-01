package main

import (
	"bufio"
	"context"
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
		a := &doccorpus.Artifact{Schema: doccorpus.SchemaV2, Manifest: doccorpus.Manifest{Repository: doccorpus.Repository{ID: strings.Repeat("a", 40), Revision: strings.Repeat("b", 40)}, BuiltAt: "2026-09-30T00:00:00Z"}, Capabilities: []doccorpus.Capability{{Name: "info", State: "present"}}}
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
		if fresh, err := net.DialTimeout("tcp", address, time.Second); err == nil {
			fresh.Close()
			t.Fatal("listener survived cancellation")
		}
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
		a := &doccorpus.Artifact{Schema: doccorpus.SchemaV2, Manifest: doccorpus.Manifest{Repository: doccorpus.Repository{ID: strings.Repeat("a", 40), Revision: strings.Repeat("b", 40)}, BuiltAt: "2026-09-30T00:00:00Z"}, Capabilities: []doccorpus.Capability{{Name: "info", State: "present"}}}
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
		if conn, e = net.DialTimeout("tcp", address, time.Second); e == nil {
			conn.Close()
			t.Fatal("listener survived shutdown")
		}
		if code := run(context.Background(), []string{"--index", path, "--sha256", doccorpus.Digest(embedded), "--listen", "0.0.0.0:0"}, notice); code != 2 {
			t.Fatal("implicit remote exposure accepted")
		}
	})
}
