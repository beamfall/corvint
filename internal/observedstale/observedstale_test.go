package observedstale

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fakeHistory struct {
	resolve   map[string]string
	ancestor  map[string]bool
	distance  map[string]int
	changed   map[string][]string
	untrusted string
}

func (f fakeHistory) Resolve(rev string) (string, error) {
	if c, ok := f.resolve[rev]; ok {
		return c, nil
	}
	return "", errors.New("unresolved")
}

func (f fakeHistory) IsAncestor(a, b string) (bool, error) { return f.ancestor[a], nil }

func (f fakeHistory) Distance(a, b string) (int, error) { return f.distance[a], nil }

func (f fakeHistory) Changed(a, b string) ([]string, error) {
	c, ok := f.changed[a]
	if !ok {
		return nil, errors.New("no diff")
	}
	return c, nil
}

func (f fakeHistory) Untrusted() string { return f.untrusted }

// OUE-V0-030..034: every state, every UNKNOWN reason, and the denominator.
func TestEvaluateStates(t *testing.T) {
	h := fakeHistory{
		resolve:  map[string]string{"c1": "c1", "c2": "c2", "c3": "c3", "side": "side"},
		ancestor: map[string]bool{"c1": true, "c2": true, "c3": true},
		distance: map[string]int{"c1": 3, "c2": 12},
		changed:  map[string][]string{"c1": {"src/a.ts", "src/dir/b.ts"}, "c2": {"src/a.ts"}},
	}
	recs := []Record{
		{ID: "current", Revision: "c1", Paths: []string{"src/other.ts"}},
		{ID: "stale", Revision: "c1", Paths: []string{"src/a.ts", "src/other.ts"}},
		{ID: "stale-dir", Revision: "c1", Paths: []string{"src/dir"}},
		{ID: "retired", Revision: "c2", Paths: []string{"src/a.ts"}},
		{ID: "old-current", Revision: "c2", Paths: []string{"src/z.ts"}},
		{ID: "no-paths", Revision: "c1"},
		{ID: "aliased", Revision: "c1", Paths: []string{"@api/app.rb"}},
		{ID: "unresolved", Revision: "gone", Paths: []string{"src/a.ts"}},
		{ID: "not-ancestor", Revision: "side", Paths: []string{"src/a.ts"}},
		{ID: "missing-diff", Revision: "c3", Paths: []string{"src/a.ts"}},
	}
	rep := Evaluate(h, "head", recs, Options{RetireAfter: 10})
	want := map[string]string{
		"current": Current, "stale": StaleUnconfirmed, "stale-dir": StaleUnconfirmed, "retired": Retired, "old-current": Current,
		"no-paths": Unknown + "/no-linked-paths", "aliased": Unknown + "/linked-path-outside-root", "unresolved": Unknown + "/revision-unresolved",
		"not-ancestor": Unknown + "/not-ancestor", "missing-diff": Unknown + "/missing-diff",
	}
	for _, r := range rep.Records {
		got := r.State
		if r.Reason != "" {
			got += "/" + r.Reason
		}
		if got != want[r.ID] {
			t.Fatalf("%s: got %s want %s", r.ID, got, want[r.ID])
		}
		if r.State == Unknown && r.Distance != nil {
			t.Fatalf("%s: an UNKNOWN record must not claim a distance", r.ID)
		}
	}
	c := rep.Counts
	if c.Total != 10 || c.Current != 2 || c.StaleUnconfirmed != 2 || c.Retired != 1 || c.Unknown != 5 || c.Current+c.StaleUnconfirmed+c.Retired+c.Unknown != c.Total {
		t.Fatalf("counts: %+v", c)
	}
	// Without a retire distance nothing retires.
	if rep := Evaluate(h, "head", recs[3:4], Options{}); rep.Records[0].State != StaleUnconfirmed {
		t.Fatalf("retire-after 0 must never retire: %+v", rep.Records[0])
	}
	// Untrusted ancestry never upgrades a record.
	h.untrusted = "checkout is shallow"
	for _, r := range Evaluate(h, "head", recs[:2], Options{}).Records {
		if r.State != Unknown || !strings.HasPrefix(r.Reason, "ancestry-untrusted") {
			t.Fatalf("untrusted ancestry: %+v", r)
		}
	}
}

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// OUE-V0-035: the Git-backed history answers through a real checkout.
func TestGitHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	dir := t.TempDir()
	gitT(t, dir, "init", "-q")
	write := func(p, s string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("src/a.ts", "1")
	write("src/b.ts", "1")
	gitT(t, dir, "add", ".")
	gitT(t, dir, "commit", "-q", "-m", "one")
	c1 := gitT(t, dir, "rev-parse", "HEAD")
	write("src/a.ts", "2")
	gitT(t, dir, "commit", "-q", "-am", "two")
	c2 := gitT(t, dir, "rev-parse", "HEAD")
	h := GitHistory{Ctx: context.Background(), Root: dir}
	if u := h.Untrusted(); u != "" {
		t.Fatalf("untrusted: %s", u)
	}
	rep := Evaluate(h, c2, []Record{
		{ID: "a", Revision: c1, Paths: []string{"src/a.ts"}},
		{ID: "b", Revision: c1, Paths: []string{"src/b.ts"}},
		{ID: "future", Revision: c2, Paths: []string{"src/a.ts"}},
		{ID: "missing", Revision: strings.Repeat("0", 40), Paths: []string{"src/a.ts"}},
	}, Options{})
	got := map[string]string{}
	for _, r := range rep.Records {
		got[r.ID] = r.State + "/" + r.Reason
	}
	if got["a"] != StaleUnconfirmed+"/" || got["b"] != Current+"/" || got["future"] != Current+"/" || got["missing"] != Unknown+"/revision-unresolved" {
		t.Fatalf("git states: %v", got)
	}
	if d := rep.Records[0].Distance; d == nil || *d != 1 {
		t.Fatalf("distance: %v", d)
	}
	if ok, err := h.IsAncestor(c2, c1); err != nil || ok {
		t.Fatalf("reverse ancestry: %v %v", ok, err)
	}
}
