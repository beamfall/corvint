package upgradechanges

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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
		req := Request{root, from, to, []string{"holds  notes.txt"}}
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
			_, e := Compile(context.Background(), Request{root, base, base, []string{path}})
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
		r, e := Compile(context.Background(), Request{root, base, to, []string{"refs"}})
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
		r, err := Compile(context.Background(), Request{root, base, to, []string{"refs"}})
		if err != nil {
			t.Fatal(err)
		}
		if !r.Truncated || len(r.Issues) != maxRows || len(r.Hints) != 0 {
			t.Fatalf("truncated=%v issues=%d hints=%d", r.Truncated, len(r.Issues), len(r.Hints))
		}
		_, err = Compile(context.Background(), Request{root, base, to, make([]string, maxFiles+1)})
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

func TestCompileRefsReadHonoursContext(t *testing.T) {
	t.Run("UCL1142-V0-004 stalled refs read ends at the deadline", func(t *testing.T) {
		root, base := repoTest(t)
		if e := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("holds #7\n"), 0o600); e != nil {
			t.Fatal(e)
		}
		release := make(chan struct{})
		defer close(release)
		entered := make(chan struct{})
		old := readRefsFile
		readRefsFile = func(string) ([]byte, bool, error) { close(entered); <-release; return nil, false, nil }
		defer func() { readRefsFile = old }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, e := Compile(ctx, Request{Root: root, From: base, To: base, Files: []string{"notes.txt"}})
			done <- e
		}()
		select {
		case <-entered:
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
	})
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
