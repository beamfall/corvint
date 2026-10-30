package testconfine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScopes(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ScopesPath), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAcceptsOnlyTheExactGrammar_AFPV0023(t *testing.T) {
	root := t.TempDir()
	if scopes, err := Load(root); err != nil || scopes != nil {
		t.Fatalf("absent declaration = %v, %v; want nil, nil", scopes, err)
	}
	writeScopes(t, root, `{"profile":"corvint-test-read-scopes/0","packages":{"cmd/a":[],"internal/b":["docs/","go.mod"]}}`)
	scopes, err := Load(root)
	if err != nil || len(scopes) != 2 || strings.Join(scopes["internal/b"], ",") != "docs/,go.mod" || scopes["cmd/a"] == nil {
		t.Fatalf("valid declaration = %v, %v", scopes, err)
	}
	for _, body := range []string{
		`{"profile":"corvint-test-read-scopes/1","packages":{}}`,
		`{"profile":"corvint-test-read-scopes/0"}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{},"extra":1}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{"a":null}}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{"a":[],"a":[]}}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{"../a":[]}}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{".":[]}}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{"a":["b","a"]}}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{"a":["a","a"]}}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{"a":["/etc"]}}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{"a":["x/../y"]}}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{"a":[".git/"]}}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{"a":[".git/HEAD"]}}`,
		`{"profile":"corvint-test-read-scopes/0","packages":{"a":["x//"]}}`,
		`not json`,
	} {
		writeScopes(t, root, body)
		if scopes, err := Load(root); err == nil {
			t.Errorf("Load(%s) = %v, want an error", body, scopes)
		}
	}
	if err := os.Remove(filepath.Join(root, ScopesPath)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(root, ScopesPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Error("a symbolic link declaration loaded")
	}
}

func TestRulesGrantOutsideRootPackageAndEntriesOnly_AFPV0023(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "repo")
	for _, name := range []string{"repo/pkg/sub/f", "repo/docs/d", "repo/secret/s", "repo/go.mod", "sibling/x"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(base, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rules, err := Rules(root, "pkg", []string{"docs/", "go.mod", "missing/"})
	if err != nil {
		t.Fatal(err)
	}
	granted := map[string]bool{}
	for _, rule := range rules {
		granted[rule.Path] = rule.Dir
		if rule.Path == root || strings.HasPrefix(root, rule.Path+string(filepath.Separator)) {
			t.Errorf("rule %s grants the root or one of its ancestors", rule.Path)
		}
	}
	for name, dir := range map[string]bool{"repo/pkg": true, "repo/docs": true, "repo/go.mod": false, "sibling": true} {
		if got, ok := granted[filepath.Join(base, name)]; !ok || got != dir {
			t.Errorf("rule for %s = %v, %v; want dir=%v", name, got, ok, dir)
		}
	}
	for _, name := range []string{"repo/secret", "repo/missing"} {
		if _, ok := granted[filepath.Join(base, name)]; ok {
			t.Errorf("%s is granted", name)
		}
	}
	if err := os.Symlink(filepath.Join(base, "repo"), filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("secret", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	rules, err = Rules(root, "pkg", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if rule.Path == filepath.Join(base, "link") {
			t.Errorf("the link %s into root is granted", rule.Path)
		}
	}
	for _, entries := range [][]string{{"docs"}, {"go.mod/"}, {"alias/"}, {"alias"}} {
		if _, err := Rules(root, "pkg", entries); err == nil {
			t.Errorf("Rules accepted entries %q whose form does not name what they are", entries)
		}
	}
	for _, bad := range []string{"relative", "/", root + "/"} {
		if _, err := Rules(bad, "pkg", nil); err == nil {
			t.Errorf("Rules(%q) accepted", bad)
		}
	}
}

func TestConfinedEnvTurnsOffVCSStamping_AFPV0023(t *testing.T) {
	for _, tc := range []struct {
		env  []string
		want string
	}{
		{[]string{"HOME=/h"}, "HOME=/h,GOFLAGS=-buildvcs=false"},
		{[]string{"GOFLAGS=", "HOME=/h"}, "HOME=/h,GOFLAGS=-buildvcs=false"},
		{[]string{"GOFLAGS=-mod=mod", "HOME=/h"}, "HOME=/h,GOFLAGS=-mod=mod -buildvcs=false"},
	} {
		if got := strings.Join(ConfinedEnv(tc.env), ","); got != tc.want {
			t.Errorf("ConfinedEnv(%q) = %q, want %q", tc.env, got, tc.want)
		}
	}
}
