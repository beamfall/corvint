package evidencereach

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const reachDoc = `{"profile":"corvint-evidence-reach/0","shared":["e2e/support/","**/*.config.ts"]}`

func TestParseConfig(t *testing.T) {
	t.Run("PWR-V0-038 a strict declaration is admitted", func(t *testing.T) {
		cfg, err := ParseConfig([]byte(reachDoc))
		if err != nil || !reflect.DeepEqual(cfg.Shared, []string{"e2e/support/", "**/*.config.ts"}) {
			t.Fatalf("got %+v %v", cfg, err)
		}
		if _, err := ParseConfig([]byte(`{"profile":"corvint-evidence-reach/0","shared":[]}`)); err != nil {
			t.Fatalf("empty shared list: %v", err)
		}
	})
	t.Run("PWR-V0-038 an invalid declaration is refused", func(t *testing.T) {
		many := make([]string, MaxPatterns+1)
		for i := range many {
			many[i] = fmt.Sprintf("%q", fmt.Sprintf("p%d", i))
		}
		for name, raw := range map[string]string{
			"profile":      `{"profile":"corvint-evidence-reach/1","shared":[]}`,
			"no shared":    `{"profile":"corvint-evidence-reach/0"}`,
			"null shared":  `{"profile":"corvint-evidence-reach/0","shared":null}`,
			"unknown":      `{"profile":"corvint-evidence-reach/0","shared":[],"extra":1}`,
			"trailing":     reachDoc + `{}`,
			"not json":     `shared: [cmd/]`,
			"absolute":     `{"profile":"corvint-evidence-reach/0","shared":["/etc/"]}`,
			"dotdot":       `{"profile":"corvint-evidence-reach/0","shared":["../x"]}`,
			"backslash":    `{"profile":"corvint-evidence-reach/0","shared":["a\\b"]}`,
			"bad glob":     `{"profile":"corvint-evidence-reach/0","shared":["a/[b"]}`,
			"partial **":   `{"profile":"corvint-evidence-reach/0","shared":["a/b**"]}`,
			"too many **":  `{"profile":"corvint-evidence-reach/0","shared":["**/**/**/**/**"]}`,
			"empty":        `{"profile":"corvint-evidence-reach/0","shared":[""]}`,
			"too many":     `{"profile":"corvint-evidence-reach/0","shared":[` + strings.Join(many, ",") + `]}`,
			"oversize doc": `{"profile":"corvint-evidence-reach/0","shared":[],"x":"` + strings.Repeat("a", MaxConfigBytes) + `"}`,
		} {
			if _, err := ParseConfig([]byte(raw)); err == nil {
				t.Fatalf("%s admitted", name)
			}
		}
	})
	t.Run("PWR-V0-038 patterns match whole segments", func(t *testing.T) {
		for _, c := range []struct {
			pattern, path string
			want          bool
		}{
			{"cmd/", "cmd/corvint/main.go", true},
			{"cmd/", "cmdx/main.go", false},
			{"go.mod", "go.mod", true},
			{"go.mod", "sub/go.mod", false},
			{"**/*.config.ts", "playwright.config.ts", true},
			{"**/*.config.ts", "e2e/deep/vite.config.ts", true},
			{"e2e/*/fixtures/**", "e2e/a/fixtures/x/y.json", true},
			{"e2e/*/fixtures/**", "e2e/a/b/fixtures/y.json", false},
			{"src/*.ts", "src/a/b.ts", false},
		} {
			if got := Match(c.pattern, c.path); got != c.want {
				t.Fatalf("Match(%q, %q) = %v", c.pattern, c.path, got)
			}
		}
	})
}

func TestClassify(t *testing.T) {
	cfg, _ := ParseConfig([]byte(reachDoc))
	t.Run("PWR-V0-039 a cited, shared or declaration change is STALE and named", func(t *testing.T) {
		verdict, deciding, truncated := Classify(
			[]string{"src/app.ts", ConfigPath, "e2e/cart.spec.ts", "e2e/support/login.ts", "README.md"},
			[]string{"e2e/cart.spec.ts"}, cfg)
		want := []Deciding{{ConfigPath, ByReachConfig}, {"e2e/cart.spec.ts", ByCited}, {"e2e/support/login.ts", ByShared}}
		if verdict != Stale || !reflect.DeepEqual(deciding, want) || truncated != 0 {
			t.Fatalf("got %s %+v %d", verdict, deciding, truncated)
		}
	})
	t.Run("PWR-V0-039 only changes outside the reach are FRESH", func(t *testing.T) {
		verdict, deciding, _ := Classify([]string{"src/app.ts", "README.md"}, []string{"e2e/cart.spec.ts"}, cfg)
		if verdict != Fresh || len(deciding) != 0 {
			t.Fatalf("got %s %+v", verdict, deciding)
		}
		if verdict, _, _ := Classify(nil, nil, cfg); verdict != Fresh {
			t.Fatalf("no change: %s", verdict)
		}
	})
	t.Run("PWR-V0-039 deciding paths are capped and the rest counted", func(t *testing.T) {
		changed := []string{}
		for i := 0; i < MaxDecidingPaths+7; i++ {
			changed = append(changed, fmt.Sprintf("e2e/support/f%04d.ts", i))
		}
		verdict, deciding, truncated := Classify(changed, nil, cfg)
		if verdict != Stale || len(deciding) != MaxDecidingPaths || truncated != 7 || deciding[0].Path != "e2e/support/f0000.ts" {
			t.Fatalf("got %s %d %d", verdict, len(deciding), truncated)
		}
	})
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	prefix := []string{"-c", "maintenance.autoDetach=false", "-c", "gc.autoDetach=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-C", root}
	out, err := exec.Command("git", append(prefix, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commit(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	for name, body := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "change")
	return git(t, root, "rev-parse", "HEAD")
}

// snapshot digests every file under root, including .git, so a read can be shown not to write.
func snapshot(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s %v %d %d\n", p, info.Mode(), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestReaderCheck(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "corvint@example.test")
	git(t, root, "config", "user.name", "Corvint Test")
	undeclared := commit(t, root, map[string]string{"src/app.ts": "v0\n", "e2e/cart.spec.ts": "t0\n"})
	declared := commit(t, root, map[string]string{ConfigPath: reachDoc})
	appOnly := commit(t, root, map[string]string{"src/app.ts": "v1\n"})
	support := commit(t, root, map[string]string{"e2e/support/login.ts": "l1\n"})
	spec := commit(t, root, map[string]string{"e2e/cart.spec.ts": "t1\n"})
	invalid := commit(t, root, map[string]string{ConfigPath: `{"profile":"corvint-evidence-reach/0","shared":["../x"]}`})
	cited := []string{"e2e/cart.spec.ts"}
	before := snapshot(t, root)
	r := NewReader(root)

	t.Run("PWR-V0-039 a change outside the reach is FRESH", func(t *testing.T) {
		res := r.Check(ctx, declared, appOnly, cited)
		if res.Verdict != Fresh || res.Reason != "" || res.ChangedPaths != 1 || res.Old != declared || res.New != appOnly {
			t.Fatalf("got %+v", res)
		}
		if same := r.Check(ctx, appOnly, "HEAD~3", cited); same.Verdict != Fresh || same.ChangedPaths != 0 {
			t.Fatalf("same commit: %+v", same)
		}
	})
	t.Run("PWR-V0-039 a cited or shared change is STALE naming the path", func(t *testing.T) {
		res := r.Check(ctx, appOnly, support, cited)
		if res.Verdict != Stale || !reflect.DeepEqual(res.Deciding, []Deciding{{"e2e/support/login.ts", ByShared}}) {
			t.Fatalf("shared: %+v", res)
		}
		res = r.Check(ctx, support, spec, cited)
		if res.Verdict != Stale || !reflect.DeepEqual(res.Deciding, []Deciding{{"e2e/cart.spec.ts", ByCited}}) {
			t.Fatalf("cited: %+v", res)
		}
		if res := r.Check(ctx, support, spec, nil); res.Verdict != Fresh {
			t.Fatalf("uncited: %+v", res)
		}
		res = r.Check(ctx, undeclared, declared, cited)
		if res.Verdict != Stale || res.Deciding[0].By != ByReachConfig {
			t.Fatalf("declaration: %+v", res)
		}
	})
	t.Run("PWR-V0-039 resolution errors are UNRESOLVED with a reason, never FRESH", func(t *testing.T) {
		for name, c := range map[string]struct {
			old, new string
			cited    []string
			reason   string
		}{
			"unknown revision":  {"no-such-rev", appOnly, cited, ReasonRevisionUnresolved},
			"option revision":   {"--all", appOnly, cited, ReasonRevisionUnresolved},
			"empty revision":    {declared, "", cited, ReasonRevisionUnresolved},
			"blob revision":     {declared, appOnly + ":src/app.ts", cited, ReasonRevisionUnresolved},
			"undeclared reach":  {undeclared, undeclared, cited, ReasonReachUndeclared},
			"invalid reach":     {spec, invalid, cited, ReasonReachInvalid},
			"invalid cited":     {declared, appOnly, []string{"../e2e/cart.spec.ts"}, ReasonCitedPathInvalid},
			"too many cited":    {declared, appOnly, make([]string, MaxCited+1), ReasonCitedPathInvalid},
			"absolute cited":    {declared, appOnly, []string{"/e2e/cart.spec.ts"}, ReasonCitedPathInvalid},
			"declared old only": {declared, undeclared, cited, ReasonReachUndeclared},
		} {
			res := r.Check(ctx, c.old, c.new, c.cited)
			if res.Verdict != Unresolved || res.Reason != c.reason || len(res.Deciding) != 0 {
				t.Fatalf("%s: got %+v", name, res)
			}
		}
		if res := NewReader(t.TempDir()).Check(ctx, "HEAD", "HEAD", cited); res.Verdict != Unresolved || res.Reason != ReasonRevisionUnresolved {
			t.Fatalf("not a repository: %+v", res)
		}
	})
	t.Run("PWR-V0-040 the summary form drops lists and keeps the verdict", func(t *testing.T) {
		res := r.Check(ctx, appOnly, support, cited)
		s := res.Summary()
		if s.Verdict != Stale || s.Deciding != nil || s.Shared != nil || s.Cited != nil || s.ChangedPaths != 1 || res.Deciding == nil {
			t.Fatalf("summary %+v", s)
		}
	})
	t.Run("PWR-V0-039 the read does not write the repository", func(t *testing.T) {
		if after := snapshot(t, root); after != before {
			t.Fatal("repository changed during reads")
		}
	})
}
