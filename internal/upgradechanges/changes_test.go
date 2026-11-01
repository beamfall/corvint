package upgradechanges

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", root}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func repoTest(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	var e error
	root, e = filepath.EvalSymlinks(root)
	if e != nil {
		t.Fatal(e)
	}
	gitTest(t, root, "init", "-q", "-b", "main")
	gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "base")
	return root, gitTest(t, root, "rev-parse", "HEAD")
}
func mergeTest(t *testing.T, root, branch, message, merge string) string {
	t.Helper()
	gitTest(t, root, "checkout", "-q", "-b", branch)
	gitTest(t, root, "commit", "-q", "--allow-empty", "-m", message)
	gitTest(t, root, "checkout", "-q", "main")
	gitTest(t, root, "merge", "-q", "--no-ff", branch, "-m", merge)
	return gitTest(t, root, "rev-parse", "HEAD")
}
func treeBytes(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	if e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, e := os.ReadFile(p)
		if e == nil {
			out[p] = sha256.Sum256(b)
		}
		return e
	}); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestCompileRangeAndRefs(t *testing.T) {
	t.Run("UCL1142-V0-001 UCL1142-V0-002 UCL1142-V0-003 UCL1142-V0-004 local evidence", func(t *testing.T) {
		root, base := repoTest(t)
		mergeTest(t, root, "old", "Closes #99", "Merge pull request #9 from fixture/old\n\nOld fix")
		from := gitTest(t, root, "rev-parse", "HEAD")
		mergeTest(t, root, "fix", "Fixes #12\nRefs #13\nresolves #14", "Merge pull request #20 from fixture/fix\n\nRepair password=should-never-appear")
		to := mergeTest(t, root, "second", "Refs #12\nCloses #15", "Merge pull request #21 from fixture/second\n\nSecond fix")
		if e := os.WriteFile(filepath.Join(root, "holds  notes.txt"), []byte("#12 #13 #99\n#14 and #15 #12\nsecret=adopter-secret"), 0600); e != nil {
			t.Fatal(e)
		}
		before := treeBytes(t, root)
		req := Request{Root: root, From: from, To: to, Files: []string{"holds  notes.txt"}}
		r, e := Compile(context.Background(), req)
		if e != nil {
			t.Fatal(e)
		}
		if r.From != from || r.To != to || len(r.PRs) != 2 || len(r.Issues) != 5 || len(r.Hints) != 4 || r.Truncated {
			t.Fatalf("report: %+v", r)
		}
		if r.Hints[0].Line != 1 || r.Hints[0].Issue != 12 || r.Hints[0].ClosingPR != 20 || r.Hints[0].File != "holds  notes.txt" {
			t.Fatalf("hint: %+v", r.Hints[0])
		}
		for _, h := range r.Hints {
			if h.Issue == 13 || h.Issue == 99 {
				t.Fatalf("invented closure: %+v", h)
			}
		}
		for _, i := range r.Issues {
			if i.Summary == "" {
				t.Fatal("missing source-derived issue summary")
			}
			if i.Number == 13 && i.State != "unknown" {
				t.Fatal(i)
			}
		}
		b, _ := json.Marshal(r)
		if strings.Contains(string(b), "should-never-appear") || strings.Contains(string(b), "adopter-secret") {
			t.Fatalf("secret emitted: %s", b)
		}
		r2, e := Compile(context.Background(), req)
		if e != nil {
			t.Fatal(e)
		}
		b2, _ := json.Marshal(r2)
		if string(b) != string(b2) {
			t.Fatal("nondeterministic output")
		}
		if !reflect.DeepEqual(before, treeBytes(t, root)) {
			t.Fatal("read mutated checkout")
		}
		empty, e := Compile(context.Background(), Request{Root: root, From: base, To: base})
		if e != nil || len(empty.PRs) != 0 || empty.Truncated {
			t.Fatalf("equal: %+v %v", empty, e)
		}
		// A subject masquerading as a merge is not a merged PR.
		gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "Merge pull request #88 from fake\n\nCloses #88")
		r, e = Compile(context.Background(), Request{Root: root, From: to, To: gitTest(t, root, "rev-parse", "HEAD")})
		if e != nil || len(r.PRs) != 0 || len(r.Issues) != 0 {
			t.Fatalf("nonmerge: %+v %v", r, e)
		}
	})
}
func TestCompileRefusals(t *testing.T) {
	t.Run("UCL1142-V0-001 coded refusal", func(t *testing.T) {
		root, base := repoTest(t)
		to := mergeTest(t, root, "fix", "Fixes #1", "Merge pull request #2 from fixture/fix")
		for _, tc := range []struct{ from, to, code string }{{"--all", to, "changes-invalid-arguments"}, {strings.Repeat("a", 40), to, "changes-build-unavailable"}, {to, base, "changes-range-unavailable"}} {
			_, e := Compile(context.Background(), Request{Root: root, From: tc.from, To: tc.to})
			if e == nil || e.Error() != tc.code {
				t.Fatalf("%+v: %v", tc, e)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e := Compile(ctx, Request{Root: root, From: base, To: to}); e == nil {
			t.Fatal("cancel ignored")
		}
	})
}
func TestCompileUnsafeRefs(t *testing.T) {
	t.Run("UCL1142-V0-003 symlinks and nonregular", func(t *testing.T) {
		root, base := repoTest(t)
		if e := os.WriteFile(filepath.Join(root, "file"), []byte("#1"), 0600); e != nil {
			t.Fatal(e)
		}
		if e := os.Symlink("file", filepath.Join(root, "link")); e != nil {
			t.Fatal(e)
		}
		if e := os.Symlink(root, filepath.Join(root, "dirlink")); e != nil {
			t.Fatal(e)
		}
		for _, path := range []string{"link", "dirlink/file", ".", "missing"} {
			_, e := Compile(context.Background(), Request{Root: root, From: base, To: base, Files: []string{path}})
			if e == nil || e.Error() != "changes-refs-unavailable" {
				t.Fatalf("%s: %v", path, e)
			}
		}
	})
}
func TestCompileBounds(t *testing.T) {
	t.Run("UCL1142-V0-004 message file and rows", func(t *testing.T) {
		root, base := repoTest(t)
		to := mergeTest(t, root, "large", strings.Repeat("x", maxMessage)+"\nCloses #9", "Merge pull request #1 from fixture/large\n\nCloses #2")
		content := strings.Repeat("#2\n", maxRows+2) + strings.Repeat("x", maxFileBytes)
		if e := os.WriteFile(filepath.Join(root, "refs"), []byte(content), 0600); e != nil {
			t.Fatal(e)
		}
		r, e := Compile(context.Background(), Request{Root: root, From: base, To: to, Files: []string{"refs"}})
		if e != nil {
			t.Fatal(e)
		}
		if !r.Truncated || len(r.Hints) != maxRows || len(r.Issues) != 1 || r.Issues[0].Number != 2 {
			t.Fatalf("bounds: truncated=%v hints=%d issues=%+v", r.Truncated, len(r.Hints), r.Issues)
		}
	})
	t.Run("UCL1142-V0-004 history cap", func(t *testing.T) {
		root, base := repoTest(t)
		for i := 0; i < maxCommits+1; i++ {
			gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "plain")
		}
		r, e := Compile(context.Background(), Request{Root: root, From: base, To: gitTest(t, root, "rev-parse", "HEAD")})
		if e != nil || !r.Truncated {
			t.Fatalf("history: %+v %v", r, e)
		}
	})
	t.Run("UCL1142-V0-004 total git bytes", func(t *testing.T) {
		root, base := repoTest(t)
		g := gitReader{context.Background(), root, 8}
		b, cut, e := g.read(128, "show", "--format=%B", "--no-patch", base)
		if e != nil || len(b) > 8 {
			t.Fatalf("bounded read: %q %v", b, e)
		}
		_ = cut
		g.remaining = 0
		b, cut, e = g.read(128, "show", base)
		if e != nil || !cut || len(b) != 0 {
			t.Fatal("total bound not retained")
		}
	})
}

func TestCompileGitConfiguration(t *testing.T) {
	t.Run("UCL1142-V0-001 UCL1142-V0-004 signature and encoding isolation", func(t *testing.T) {
		root, base := repoTest(t)
		to := mergeTest(t, root, "signed", "Fixes #71", "Merge pull request #72 from fixture/signed\n\nRésumé")
		raw := gitTest(t, root, "cat-file", "commit", to)
		raw = strings.Replace(raw, "\n\n", "\ngpgsig -----BEGIN PGP SIGNATURE-----\n fake\n -----END PGP SIGNATURE-----\n\n", 1) + "\n"
		c := exec.Command("git", "-C", root, "hash-object", "-t", "commit", "-w", "--stdin")
		c.Stdin = strings.NewReader(raw)
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("signed fixture: %v %s", err, out)
		}
		to = strings.TrimSpace(string(out))
		marker := filepath.Join(root, "verifier-ran")
		verifier := filepath.Join(root, "verifier")
		if err := os.WriteFile(verifier, []byte("#!/bin/sh\nprintf invoked > '"+marker+"'\nexit 1\n"), 0700); err != nil {
			t.Fatal(err)
		}
		gitTest(t, root, "config", "log.showSignature", "true")
		gitTest(t, root, "config", "gpg.program", verifier)
		gitTest(t, root, "config", "i18n.logOutputEncoding", "UTF-16")
		r, err := Compile(context.Background(), Request{Root: root, From: base, To: to})
		if err != nil {
			t.Fatal(err)
		}
		if len(r.PRs) != 1 || r.PRs[0].Summary != "Résumé" || len(r.Issues) != 1 || r.Issues[0].Number != 71 {
			t.Fatalf("config changed evidence: %+v", r)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("signature verifier executed")
		}
		// Prove the hostile fixture reaches the verifier when the suppressor is absent.
		c = exec.Command("git", "-C", root, "show", "--no-patch", to)
		_ = c.Run()
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("verifier control did not execute: %v", err)
		}
	})
}

func TestCappedOutput(t *testing.T) {
	t.Run("UCL1142-V0-004 copy cannot bypass bound", func(t *testing.T) {
		canceled := false
		out := &cappedOutput{limit: 8, cancel: func() { canceled = true }}
		// os/exec copies from a pipe; this also guards against promoted ReadFrom methods.
		if _, err := io.Copy(out, strings.NewReader(strings.Repeat("x", 100))); err != nil {
			t.Fatal(err)
		}
		if out.buffer.Len() != 8 || !out.exceeded || !canceled {
			t.Fatalf("cap len=%d exceeded=%v cancel=%v", out.buffer.Len(), out.exceeded, canceled)
		}
	})
}

func TestCompileReferenceAndIssueCaps(t *testing.T) {
	t.Run("UCL1142-V0-004 reference and issue entry bounds", func(t *testing.T) {
		root, base := repoTest(t)
		var message strings.Builder
		for i := 1; i <= maxRows+1; i++ {
			fmt.Fprintf(&message, "Fixes #%d\n", i)
		}
		to := mergeTest(t, root, "rows", message.String(), "Merge pull request #8 from fixture/rows")
		if err := os.WriteFile(filepath.Join(root, "refs"), []byte(strings.Repeat("#9999 ", maxRefs)+"#1\n"), 0600); err != nil {
			t.Fatal(err)
		}
		r, err := Compile(context.Background(), Request{Root: root, From: base, To: to, Files: []string{"refs"}})
		if err != nil {
			t.Fatal(err)
		}
		if !r.Truncated || len(r.Issues) != maxRows || len(r.Hints) != 0 {
			t.Fatalf("truncated=%v issues=%d hints=%d", r.Truncated, len(r.Issues), len(r.Hints))
		}
		_, err = Compile(context.Background(), Request{Root: root, From: base, To: to, Files: make([]string, maxFiles+1)})
		if err == nil || err.Error() != "changes-invalid-arguments" {
			t.Fatalf("file cap: %v", err)
		}
	})
}

func TestCompileRefusesShallowCheckout(t *testing.T) {
	t.Run("UCL1142-V0-001 shallow history refusal", func(t *testing.T) {
		root, base := repoTest(t)
		mergeTest(t, root, "topic", "Closes #7", "Merge pull request #9 from x/topic")
		shallow := filepath.Join(t.TempDir(), "shallow")
		gitTest(t, root, "clone", "-q", "--depth", "1", "file://"+root, shallow)
		head := gitTest(t, shallow, "rev-parse", "HEAD")
		if _, e := Compile(context.Background(), Request{Root: shallow, From: head, To: head}); e == nil || e.Error() != "changes-range-unavailable" {
			t.Fatalf("shallow checkout accepted: %v", e)
		}
		if _, e := Compile(context.Background(), Request{Root: root, From: base, To: base}); e != nil {
			t.Fatalf("complete checkout refused: %v", e)
		}
	})
}

// EEP-V0-770: info/grafts rewrites the range as a shallow boundary does, so a grafted checkout is
// refused the same way, before the ancestry read; a shallow marker on the full clone is too.
func TestCompileRefusesGraftedCheckout_EEPV0770(t *testing.T) {
	root, base := repoTest(t)
	head := gitTest(t, root, "rev-parse", "HEAD")
	for name, apply := range map[string]func() func(){
		"grafted": func() func() { return ancestrytest.Graft(t, root, "HEAD") },
		"shallow": func() func() { return ancestrytest.Shallow(t, root, "HEAD") },
	} {
		undo := apply()
		if _, e := Compile(context.Background(), Request{Root: root, From: base, To: head}); e == nil || e.Error() != "changes-range-unavailable" {
			t.Errorf("%s checkout accepted: %v", name, e)
		}
		undo()
	}
	if _, e := Compile(context.Background(), Request{Root: root, From: base, To: head}); e != nil {
		t.Fatalf("complete checkout refused: %v", e)
	}
}

// stalledRefs models a descriptor whose read blocks until it is closed.
type stalledRefs struct {
	entered, closed, released chan struct{}
	once                      sync.Once
}

func (s *stalledRefs) Read([]byte) (int, error) {
	close(s.entered)
	<-s.closed
	close(s.released)
	return 0, os.ErrClosed
}

func (s *stalledRefs) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func TestCompileRefsReadHonoursContext(t *testing.T) {
	t.Run("UCL1142-V0-004 V1-1155 stalled refs read ends at the deadline and releases its reader", func(t *testing.T) {
		root, base := repoTest(t)
		if e := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("holds #7\n"), 0o600); e != nil {
			t.Fatal(e)
		}
		stalled := &stalledRefs{entered: make(chan struct{}), closed: make(chan struct{}), released: make(chan struct{})}
		defer stalled.Close()
		old := openRefsFile
		openRefsFile = func(string) (io.ReadCloser, error) { return stalled, nil }
		defer func() { openRefsFile = old }()
		baseline := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, e := Compile(ctx, Request{Root: root, From: base, To: base, Files: []string{"notes.txt"}})
			done <- e
		}()
		select {
		case <-stalled.entered:
		case <-time.After(30 * time.Second):
			t.Fatal("reader never entered")
		}
		cancel()
		select {
		case e := <-done:
			if e == nil || e.Error() != "changes-git-unavailable" {
				t.Fatalf("error=%v", e)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Compile outlived its context")
		}
		// Cancellation closed the descriptor; the stalled read returned without
		// any further release from the test.
		select {
		case <-stalled.released:
		case <-time.After(5 * time.Second):
			t.Fatal("stalled read goroutine was not released by cancellation")
		}
		deadline := time.Now().Add(5 * time.Second)
		for runtime.NumGoroutine() > baseline {
			if time.Now().After(deadline) {
				t.Fatalf("goroutines=%d, baseline %d: refs reader leaked", runtime.NumGoroutine(), baseline)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	t.Run("V1-1155 cancellation cleanup survives delayed callback dispatch", func(t *testing.T) {
		old := openRefsFile
		defer func() { openRefsFile = old }()
		for i := 0; i < 200; i++ {
			stalled := &stalledRefs{entered: make(chan struct{}), closed: make(chan struct{}), released: make(chan struct{})}
			openRefsFile = func(string) (io.ReadCloser, error) { return stalled, nil }
			ctx, cancel := context.WithCancel(context.Background())
			result := make(chan error, 1)
			go func() {
				_, _, e := readRefsContext(ctx, "/unused")
				result <- e
			}()
			<-stalled.entered
			cancel()
			if e := <-result; !errors.Is(e, context.Canceled) {
				t.Fatalf("iteration %d error=%v", i, e)
			}
			select {
			case <-stalled.released:
			case <-time.After(5 * time.Second):
				stalled.Close()
				t.Fatalf("iteration %d: cancellation did not close the stalled descriptor", i)
			}
		}
	})
	t.Run("V1-1411 cancellation wins over the close error when both are ready", func(t *testing.T) {
		oldOpen, oldWait := openRefsFile, beforeRefsWait
		defer func() { openRefsFile, beforeRefsWait = oldOpen, oldWait }()
		for i := 0; i < 200; i++ {
			stalled := &stalledRefs{entered: make(chan struct{}), closed: make(chan struct{}), released: make(chan struct{})}
			openRefsFile = func(string) (io.ReadCloser, error) { return stalled, nil }
			ctx, cancel := context.WithCancel(context.Background())
			ready := false
			beforeRefsWait = func(done <-chan refsRead) {
				select {
				case <-stalled.entered:
				case <-time.After(30 * time.Second):
					stalled.Close()
					return
				}
				cancel()
				deadline := time.Now().Add(5 * time.Second)
				for len(done) == 0 && time.Now().Before(deadline) {
					time.Sleep(100 * time.Microsecond)
				}
				ready = len(done) == 1
			}
			_, _, e := readRefsContext(ctx, "/unused")
			cancel()
			if !ready {
				t.Fatalf("iteration %d: the read result and the cancellation were not both ready", i)
			}
			if !errors.Is(e, context.Canceled) {
				t.Fatalf("iteration %d error=%v", i, e)
			}
		}
	})
	t.Run("V1-1155 a completed read closes its descriptor", func(t *testing.T) {
		closed := false
		old := openRefsFile
		openRefsFile = func(string) (io.ReadCloser, error) {
			return closeRecorder{strings.NewReader("holds #7\n"), &closed}, nil
		}
		defer func() { openRefsFile = old }()
		data, cut, err := readRefsContext(context.Background(), "/unused")
		if err != nil || cut || string(data) != "holds #7\n" || !closed {
			t.Fatalf("data=%q cut=%v err=%v closed=%v", data, cut, err, closed)
		}
	})
}

type closeRecorder struct {
	io.Reader
	closed *bool
}

func (c closeRecorder) Close() error {
	*c.closed = true
	return nil
}

func TestCompileShallowCheckCancellationKeepsGitUnavailable(t *testing.T) {
	t.Run("UCL1142-V0-004 cancelled shallow probe is git-unavailable", func(t *testing.T) {
		root, base := repoTest(t)
		real, e := exec.LookPath("git")
		if e != nil {
			t.Skip("git unavailable")
		}
		shim := t.TempDir()
		marker := filepath.Join(shim, "entered")
		script := "#!/bin/sh\nfor a in \"$@\"; do if [ \"$a\" = --is-shallow-repository ]; then : > '" + marker + "'; sleep 30; exit 1; fi; done\nexec '" + real + "' \"$@\"\n"
		if e := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0o700); e != nil {
			t.Fatal(e)
		}
		t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, e := Compile(ctx, Request{Root: root, From: base, To: base})
			done <- e
		}()
		deadline := time.Now().Add(30 * time.Second)
		for {
			if _, e := os.Stat(marker); e == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("shallow probe never started")
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
		select {
		case e := <-done:
			if e == nil || e.Error() != "changes-git-unavailable" {
				t.Fatalf("error=%v", e)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Compile outlived its context")
		}
	})
}

func commitFile(t *testing.T, root, name, content, message string) {
	t.Helper()
	path := filepath.Join(root, name)
	if e := os.MkdirAll(filepath.Dir(path), 0o700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte(content), 0o600); e != nil {
		t.Fatal(e)
	}
	gitTest(t, root, "add", "--", name)
	gitTest(t, root, "commit", "-q", "-m", message)
}

func TestCompileNewFlags(t *testing.T) {
	t.Run("UCL1142-V0-007 per-PR new flags from the local source diff", func(t *testing.T) {
		root, _ := repoTest(t)
		commitFile(t, root, "cmd/corvint/old.go", "package main\n\nfunc old() { f.Bool(\"moved\", false, \"\") }\n", "seed")
		from := gitTest(t, root, "rev-parse", "HEAD")
		gitTest(t, root, "checkout", "-q", "-b", "flags")
		commitFile(t, root, "cmd/corvint/new.go", "package main\n\nfunc add() {\n\tf.Bool(\"dry-run\", false, \"\")\n\tf.StringVar(&x, \"out-file\", \"\", \"\")\n\tf.Bool(\"snake_name\", false, \"\")\n\tf.StringVar(&y, \"dotted.name\", \"\", \"\")\n\tswitch a {\n\tcase \"--host\", \"--limit=\":\n\t}\n}\n", "Add flags\n\nCloses #3")
		commitFile(t, root, "cmd/corvint/new_test.go", "package main\n\nfunc t() { f.Bool(\"test-only\", false, \"\") }\n", "test flag")
		commitFile(t, root, "cmd/corvint/testdata/x.go", "package x\n\nfunc t() { f.Bool(\"fixture-only\", false, \"\") }\n", "fixture flag")
		commitFile(t, root, "internal/other.go", "package other\n\nfunc t() { f.Bool(\"elsewhere\", false, \"\") }\n", "other flag")
		gitTest(t, root, "rm", "-q", "cmd/corvint/old.go")
		commitFile(t, root, "cmd/corvint/moved.go", "package main\n\nfunc old() { f.Bool(\"moved\", false, \"\") }\n", "move flag")
		gitTest(t, root, "checkout", "-q", "main")
		gitTest(t, root, "merge", "-q", "--no-ff", "flags", "-m", "Merge pull request #4 from fixture/flags")
		to := mergeTest(t, root, "none", "No flags", "Merge pull request #5 from fixture/none")
		r, e := Compile(context.Background(), Request{Root: root, From: from, To: to})
		if e != nil || len(r.PRs) != 2 || r.Truncated {
			t.Fatalf("report: %+v %v", r, e)
		}
		want := []string{"--dotted.name", "--dry-run", "--host", "--limit", "--out-file", "--snake_name"}
		if r.PRs[0].FlagsBasis != "cmd-corvint-go-source-diff" || !reflect.DeepEqual(r.PRs[0].NewFlags, want) || r.PRs[0].Commands != "NOT_OBSERVED" {
			t.Fatalf("flags PR: %+v", r.PRs[0])
		}
		if r.PRs[1].FlagsBasis != "cmd-corvint-go-source-diff" || len(r.PRs[1].NewFlags) != 0 || r.PRs[1].NewFlags == nil {
			t.Fatalf("empty PR: %+v", r.PRs[1])
		}
		b, _ := json.Marshal(r.PRs[1])
		if !strings.Contains(string(b), `"new_flags_basis":"cmd-corvint-go-source-diff","new_flags":[],"new_commands":"NOT_OBSERVED"`) {
			t.Fatalf("wire: %s", b)
		}
	})
	t.Run("UCL1142-V0-007 quoted and spaced paths and working-tree attributes", func(t *testing.T) {
		root, _ := repoTest(t)
		from := gitTest(t, root, "rev-parse", "HEAD")
		gitTest(t, root, "checkout", "-q", "-b", "names")
		commitFile(t, root, "cmd/corvint/space name.go", "package main\n\nfunc s() { f.Bool(\"spaced\", false, \"\") }\n", "spaced")
		commitFile(t, root, "cmd/corvint/caf\u00e9.go", "package main\n\nfunc q() { f.Bool(\"quoted\", false, \"\") }\n", "quoted")
		commitFile(t, root, "cmd/corvint/tab\tname.go", "package main\n\nfunc u() { f.Bool(\"tabbed\", false, \"\") }\n", "tabbed")
		gitTest(t, root, "checkout", "-q", "main")
		gitTest(t, root, "merge", "-q", "--no-ff", "names", "-m", "Merge pull request #7 from fixture/names")
		to := gitTest(t, root, "rev-parse", "HEAD")
		want := []string{"--quoted", "--spaced", "--tabbed"}
		for _, attributes := range []string{"", "cmd/corvint/*.go -diff\n", "cmd/corvint/*.go binary\n"} {
			if e := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte(attributes), 0o600); e != nil {
				t.Fatal(e)
			}
			r, e := Compile(context.Background(), Request{Root: root, From: from, To: to})
			if e != nil || len(r.PRs) != 1 || r.PRs[0].FlagsBasis != "cmd-corvint-go-source-diff" || !reflect.DeepEqual(r.PRs[0].NewFlags, want) {
				t.Fatalf("attributes %q: %+v %v", attributes, r, e)
			}
		}
	})
	t.Run("UCL1142-V0-007 a diff over its bound is NOT_OBSERVED and truncated", func(t *testing.T) {
		root, _ := repoTest(t)
		from := gitTest(t, root, "rev-parse", "HEAD")
		gitTest(t, root, "checkout", "-q", "-b", "big")
		commitFile(t, root, "cmd/corvint/big.go", "package main\n\n// "+strings.Repeat("x", maxFlagDiff)+"\nfunc b() { f.Bool(\"late\", false, \"\") }\n", "big")
		gitTest(t, root, "checkout", "-q", "main")
		gitTest(t, root, "merge", "-q", "--no-ff", "big", "-m", "Merge pull request #6 from fixture/big")
		to := gitTest(t, root, "rev-parse", "HEAD")
		r, e := Compile(context.Background(), Request{Root: root, From: from, To: to})
		if e != nil || len(r.PRs) != 1 || !r.Truncated || r.PRs[0].FlagsBasis != "NOT_OBSERVED" || len(r.PRs[0].NewFlags) != 0 {
			t.Fatalf("bounded: %+v %v", r, e)
		}
	})
}

func TestCompileIssueStateFile(t *testing.T) {
	root, _ := repoTest(t)
	from := gitTest(t, root, "rev-parse", "HEAD")
	to := mergeTest(t, root, "fix", "Fixes #12\nCloses #14\nRefs #15", "Merge pull request #20 from fixture/fix")
	if e := os.WriteFile(filepath.Join(root, "holds.txt"), []byte("#12\n#14\n"), 0o600); e != nil {
		t.Fatal(e)
	}
	write := func(name, body string) string {
		if e := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); e != nil {
			t.Fatal(e)
		}
		return name
	}
	t.Run("UCL1142-V0-006 operator-supplied state annotates issues and hints", func(t *testing.T) {
		state := write("issues.json", `[{"number":12,"state":"OPEN","title":"secret=never-emit"},{"number":14,"state":"CLOSED"},{"number":99,"state":"closed"}]`)
		before := treeBytes(t, root)
		r, e := Compile(context.Background(), Request{Root: root, From: from, To: to, Files: []string{"holds.txt"}, IssueStateFile: state})
		if e != nil || len(r.Issues) != 3 || len(r.Hints) != 2 {
			t.Fatalf("report: %+v %v", r, e)
		}
		got := map[int64]string{}
		for _, issue := range r.Issues {
			got[issue.Number] = issue.GitHubState
		}
		if !reflect.DeepEqual(got, map[int64]string{12: "OPEN", 14: "CLOSED", 15: "NOT_OBSERVED"}) {
			t.Fatalf("states: %v", got)
		}
		if r.Hints[0].Issue != 12 || r.Hints[0].GitHubState != "OPEN" || r.Hints[0].Message == "workaround may be removable" {
			t.Fatalf("open hint: %+v", r.Hints[0])
		}
		if r.Hints[1].Issue != 14 || r.Hints[1].GitHubState != "CLOSED" || r.Hints[1].Message != "workaround may be removable" {
			t.Fatalf("closed hint: %+v", r.Hints[1])
		}
		b, _ := json.Marshal(r)
		if strings.Contains(string(b), "never-emit") || !strings.Contains(string(b), "operator-supplied file, not verified by Corvint") || !strings.Contains(string(b), "reopen/revert semantics: NOT_OBSERVED") {
			t.Fatalf("wire: %s", b)
		}
		if !reflect.DeepEqual(before, treeBytes(t, root)) {
			t.Fatal("read mutated checkout")
		}
	})
	t.Run("UCL1142-V0-006 without a file every state stays NOT_OBSERVED", func(t *testing.T) {
		r, e := Compile(context.Background(), Request{Root: root, From: from, To: to, Files: []string{"holds.txt"}})
		if e != nil {
			t.Fatal(e)
		}
		for _, issue := range r.Issues {
			if issue.GitHubState != "NOT_OBSERVED" {
				t.Fatalf("invented state: %+v", issue)
			}
		}
		if r.Unknowns[0] != "GitHub issue state: NOT_OBSERVED" || r.Hints[0].Message != "workaround may be removable" {
			t.Fatalf("report: %+v", r)
		}
	})
	t.Run("UCL1142-V0-006 malformed, conflicting, unsafe or missing files refuse", func(t *testing.T) {
		if e := os.Symlink("issues.json", filepath.Join(root, "link.json")); e != nil {
			t.Fatal(e)
		}
		for _, name := range []string{
			write("bad-state.json", `[{"number":12,"state":"MERGED"}]`),
			write("bad-number.json", `[{"number":-1,"state":"OPEN"}]`),
			write("missing-state.json", `[{"number":12}]`),
			write("conflict.json", `[{"number":12,"state":"OPEN"},{"number":12,"state":"CLOSED"}]`),
			write("object.json", `{"number":12,"state":"OPEN"}`),
			write("null.json", " null\n"),
			write("duplicate-member.json", `[{"number":12,"state":"OPEN","state":"CLOSED"}]`),
			"link.json", "absent.json",
		} {
			if _, e := Compile(context.Background(), Request{Root: root, From: from, To: to, IssueStateFile: name}); e == nil || e.Error() != "changes-issue-state-unavailable" {
				t.Fatalf("%s: %v", name, e)
			}
		}
		if _, e := Compile(context.Background(), Request{Root: root, From: from, To: to, IssueStateFile: write("empty.json", "[]")}); e != nil {
			t.Fatalf("empty array: %v", e)
		}
	})
}

func TestCompileRangeBoundAndTimeout(t *testing.T) {
	t.Run("UCL1142-V0-008 range over the bound refuses up front with limit and commits", func(t *testing.T) {
		root, base := repoTest(t)
		mergeTest(t, root, "one", "Closes #1", "Merge pull request #1 from fixture/one")
		to := mergeTest(t, root, "two", "Closes #2", "Merge pull request #2 from fixture/two")
		real, e := exec.LookPath("git")
		if e != nil {
			t.Skip("git unavailable")
		}
		shim := t.TempDir()
		log := filepath.Join(shim, "calls")
		script := "#!/bin/sh\necho \"$*\" >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
		if e := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0o700); e != nil {
			t.Fatal(e)
		}
		t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
		saved := maxRangeCommits
		maxRangeCommits = 3
		defer func() { maxRangeCommits = saved }()
		_, err := Compile(context.Background(), Request{Root: root, From: base, To: to})
		var tooLarge *RangeTooLargeError
		if !errors.As(err, &tooLarge) || err.Error() != "changes-range-too-large" || tooLarge.Limit != 3 || tooLarge.Commits != 4 {
			t.Fatalf("error=%v %+v", err, tooLarge)
		}
		calls, e := os.ReadFile(log)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(calls), "show ") || strings.Contains(string(calls), "--first-parent") {
			t.Fatalf("expensive reads ran before the bound refused:\n%s", calls)
		}
		maxRangeCommits = 4
		if _, err := Compile(context.Background(), Request{Root: root, From: base, To: to}); err != nil {
			t.Fatalf("range at the bound refused: %v", err)
		}
	})
	t.Run("UCL1142-V0-008 own deadline refuses as changes-timeout with elapsed time", func(t *testing.T) {
		root, base := repoTest(t)
		to := mergeTest(t, root, "one", "Closes #1", "Merge pull request #1 from fixture/one")
		real, e := exec.LookPath("git")
		if e != nil {
			t.Skip("git unavailable")
		}
		shim := t.TempDir()
		script := "#!/bin/sh\nfor a in \"$@\"; do if [ \"$a\" = --first-parent ]; then sleep 30; exit 1; fi; done\nexec '" + real + "' \"$@\"\n"
		if e := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0o700); e != nil {
			t.Fatal(e)
		}
		t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
		saved := compileDeadline
		compileDeadline = 500 * time.Millisecond
		defer func() { compileDeadline = saved }()
		_, err := Compile(context.Background(), Request{Root: root, From: base, To: to})
		var timeout *TimeoutError
		if !errors.As(err, &timeout) || err.Error() != "changes-timeout" || timeout.LimitMS != 500 || timeout.ElapsedMS < 500 || timeout.ElapsedMS > 20000 {
			t.Fatalf("error=%v %+v", err, timeout)
		}
		// The caller's deadline expires after the command's own deadline fired but before Compile
		// returns (the shim's sleep holds Git's pipes for the one-second WaitDelay): the deadline
		// that fired first still classifies the refusal.
		parent, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
		defer cancel()
		_, err = Compile(parent, Request{Root: root, From: base, To: to})
		if parent.Err() == nil {
			t.Fatal("the caller's deadline had not expired when Compile returned; the race was not exercised")
		}
		if !errors.As(err, &timeout) || err.Error() != "changes-timeout" {
			t.Fatalf("later caller expiry: error=%v", err)
		}
		// A caller whose own deadline comes first is never classified as changes-timeout.
		early, cancelEarly := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelEarly()
		if _, err := Compile(early, Request{Root: root, From: base, To: to}); err == nil || errors.As(err, &timeout) {
			t.Fatalf("earlier caller expiry: error=%v", err)
		}
	})
}
