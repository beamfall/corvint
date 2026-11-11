// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// commitAndBuild commits one more page under docs and builds the corpus at the new HEAD, in the
// paged /2 schema when v2 is set and the /1 schema otherwise.
func commitAndBuild(t *testing.T, root, page string, v2 bool) []byte {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "docs", page+".md"), []byte("# "+page+"\n\nCheckout "+page+".\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("add", "docs")
	git("-c", "user.name=test", "-c", "user.email=test@invalid", "commit", "-qm", page)
	m, err := doccorpus.Inventory(context.Background(), root, git("rev-parse", "HEAD"), "docs", "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if v2 {
		m.Schema = doccorpus.ManifestSchemaV2
	}
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// answeredDigest is searchDigest for a goroutine other than the test's: it returns its failure
// instead of failing the test.
func answeredDigest(r *Registry) (string, error) {
	_, text, failure, protocol := r.Call(context.Background(), "corvint.docs_search", []byte(`{"query":"Checkout"}`))
	if protocol != nil {
		return "", fmt.Errorf("protocol error %v", protocol.Code)
	}
	if failure != nil {
		return "", fmt.Errorf("tool failure %s: %s", failure.Code, failure.Message)
	}
	var receipt struct {
		ArtifactSHA256 string `json:"artifact_sha256"`
	}
	if err := json.Unmarshal([]byte(text), &receipt); err != nil {
		return "", err
	}
	return receipt.ArtifactSHA256, nil
}

// TestDMFV1011StaleReloadNeverReplacesNewerArtifact: a call that read rebuilt bytes A and then
// waited for the load slot while another call verified and installed the later rebuild B never
// installs A over B or answers from A: it reads the file again under the slot. N parallel calls
// across an A to B swap never answer from A once B is installed, and B stays installed.
func TestDMFV1011StaleReloadNeverReplacesNewerArtifact(t *testing.T) {
	root, original := lazyFixture(t)
	rebuiltA := commitAndBuild(t, root, "pa", false)
	rebuiltB := commitAndBuild(t, root, "pb", false)
	// The receipt digest each artifact answers with, from a server started on it.
	receiptOf := func(data []byte) string {
		t.Helper()
		replaceArtifact(t, root, data)
		fresh, e := New(root, "corpus.json")
		if e != nil {
			t.Fatal(e.Code)
		}
		digest, err := answeredDigest(fresh)
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}
	digestA, digestB := receiptOf(rebuiltA), receiptOf(rebuiltB)
	if digestA == digestB {
		t.Fatal("rebuilds A and B answer with one digest")
	}
	replaceArtifact(t, root, original)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e.Code)
	}
	if _, err := answeredDigest(r); err != nil {
		t.Fatalf("first call %v", err)
	}
	fileA, fileB := doccorpus.Digest(rebuiltA), doccorpus.Digest(rebuiltB)

	reads := make(chan string, 16)
	saved := readArtifactFile
	t.Cleanup(func() { readArtifactFile = saved })
	readArtifactFile = func(root, path string) ([]byte, error) {
		raw, err := saved(root, path)
		reads <- doccorpus.Digest(raw)
		return raw, err
	}
	readOf := func(want string) {
		t.Helper()
		select {
		case got := <-reads:
			if got != want {
				t.Fatalf("call read %s, want %s", got, want)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("call never read the artifact")
		}
		// Let the call reach the load slot the test holds.
		time.Sleep(100 * time.Millisecond)
	}
	type answer struct {
		digest string
		err    error
	}
	call := func() chan answer {
		done := make(chan answer, 1)
		go func() {
			digest, err := answeredDigest(r)
			done <- answer{digest, err}
		}()
		return done
	}

	// The test holds the load slot while a call reads B and queues for it, then a call reads A
	// and queues behind it, then B is renamed over the file again. The B call reloads first; the
	// A call holds bytes older than what is installed when it gets the slot.
	r.load <- struct{}{}
	replaceArtifact(t, root, rebuiltB)
	first := call()
	readOf(fileB)
	replaceArtifact(t, root, rebuiltA)
	stale := call()
	readOf(fileA)
	replaceArtifact(t, root, rebuiltB)
	<-r.load
	for name, done := range map[string]chan answer{"B call": first, "A call": stale} {
		got := <-done
		if got.err != nil || got.digest != digestB {
			t.Fatalf("%s answered %q (A is %q, B is %q): %v", name, got.digest, digestA, digestB, got.err)
		}
	}
	if r.ArtifactSHA256() != fileB {
		t.Fatalf("installed %q after the stale reload, want B %q", r.ArtifactSHA256(), fileB)
	}
	readArtifactFile = saved

	// N parallel calls across an A to B swap: none that starts after B is installed answers from A.
	replaceArtifact(t, root, rebuiltA)
	if digest, err := answeredDigest(r); err != nil || digest != digestA {
		t.Fatalf("serving A: %q %v", digest, err)
	}
	var installed atomic.Bool
	var wrong atomic.Value
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 8 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				after := installed.Load()
				digest, err := answeredDigest(r)
				if err != nil {
					wrong.CompareAndSwap(nil, err)
					return
				}
				if after && digest != digestB {
					wrong.CompareAndSwap(nil, errors.New("a call that started after B was installed answered "+digest))
					return
				}
			}
		})
	}
	time.Sleep(50 * time.Millisecond)
	replaceArtifact(t, root, rebuiltB)
	deadline := time.Now().Add(30 * time.Second)
	for r.ArtifactSHA256() != fileB && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	installed.Store(true)
	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
	if err, _ := wrong.Load().(error); err != nil {
		t.Fatal(err)
	}
	if r.ArtifactSHA256() != fileB {
		t.Fatalf("final installed digest %q, want B %q", r.ArtifactSHA256(), fileB)
	}
}

// TestDMFV1011ArgumentsJudgedByReloadedHeader: after a /1 artifact is rebuilt as /2, the next call
// judges its arguments and tool by the /2 header: an `offset` argument and docs_inventory, both
// /2 only, are answered instead of refused by the header served before the reload.
func TestDMFV1011ArgumentsJudgedByReloadedHeader(t *testing.T) {
	root, _ := lazyFixture(t)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e.Code)
	}
	if _, _, failure, protocol := r.Call(t.Context(), "corvint.docs_search", []byte(`{"query":"Checkout","offset":0}`)); protocol == nil || protocol.Code != "invalid-arguments" || failure != nil {
		t.Fatalf("offset on a /1 artifact: %+v %+v", protocol, failure)
	}
	replaceArtifact(t, root, commitAndBuild(t, root, "p9", true))
	if _, text, failure, protocol := r.Call(t.Context(), "corvint.docs_search", []byte(`{"query":"Checkout","offset":0}`)); protocol != nil || failure != nil || !strings.Contains(text, `"artifact_sha256"`) {
		t.Fatalf("offset after the /2 rebuild: %+v %+v", protocol, failure)
	}
	if r.currentHeader().Schema != doccorpus.SchemaV2 {
		t.Fatalf("served schema %q", r.currentHeader().Schema)
	}

	root, _ = lazyFixture(t)
	r, e = New(root, "corpus.json")
	if e != nil {
		t.Fatal(e.Code)
	}
	if _, _, _, protocol := r.Call(t.Context(), "corvint.docs_inventory", []byte(`{}`)); protocol == nil || protocol.Code != "unsupported-tool" {
		t.Fatalf("docs_inventory on a /1 artifact: %+v", protocol)
	}
	replaceArtifact(t, root, commitAndBuild(t, root, "p9", true))
	if _, _, failure, protocol := r.Call(t.Context(), "corvint.docs_inventory", []byte(`{}`)); protocol != nil || failure != nil {
		t.Fatalf("docs_inventory after the /2 rebuild: %+v %+v", protocol, failure)
	}
}
