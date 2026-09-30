package stepverify

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fixture struct {
	root string
	d    Declaration
	h    Host
}

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func put(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}
func newFixture(t *testing.T) fixture {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("native descriptor observer only qualified on Darwin/Linux")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "init", "-q")
	put(t, filepath.Join(root, "allowed/file"), "baseline")
	put(t, filepath.Join(root, "guarded"), "baseline")
	put(t, filepath.Join(root, "outside"), "baseline")
	put(t, filepath.Join(root, ".gitignore"), "ignored\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-qm", "fixture")
	d := Declaration{Profile: "corvint-step-declaration/0", SessionID: "session", CapabilityID: "authoring", Author: Checkout{"author", root}, ReadOnly: []Checkout{}, WritePaths: []string{"allowed/"}, GuardPaths: []string{"allowed/guard", "guarded"}, Environment: []EnvironmentKey{{"READ_KEY", "READ_ONLY"}}}
	d, err = DecodeDeclaration(Encode(d))
	if err != nil {
		t.Fatal(err)
	}
	binary, err := exec.LookPath("git")
	if runtime.GOOS == "darwin" {
		out, e := exec.Command("/usr/bin/xcrun", "--find", "git").Output()
		if e != nil {
			t.Fatal(e)
		}
		binary = strings.TrimSpace(string(out))
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ReadInput(context.Background(), binary, &d)
	if err != nil {
		t.Fatal(err)
	}
	observed := []EnvironmentKey{{"READ_KEY", "READ_ONLY"}}
	h := Host{Profile: "corvint-step-host/0", ObserverID: "observer", SessionID: d.SessionID, CapabilityID: d.CapabilityID, DeclarationDigest: DeclarationDigest(d), MetadataProtected: true, FilesystemConfined: true, GitBinary: binary, GitBinaryDigest: hash(data), Environment: &observed}
	return fixture{root, d, h}
}
func snapshot(t *testing.T, f fixture) State {
	t.Helper()
	s, err := Snapshot(context.Background(), f.d, f.h)
	if err != nil {
		t.Fatalf("snapshot: %v %+v", err, s)
	}
	if _, err := DecodeState(Encode(s), f.d, f.h); err != nil {
		t.Fatalf("state decode: %v", err)
	}
	return s
}
func has(r Receipt, code, path string) bool {
	for _, f := range r.Findings {
		if f.Code == code && f.Path == path {
			return true
		}
	}
	return false
}
func verify(t *testing.T, f fixture, s State) (Receipt, int) {
	t.Helper()
	r, code, err := Verify(context.Background(), f.d, f.h, s)
	if err != nil {
		t.Fatal(err)
	}
	return r, code
}

func TestStepDeclaration(t *testing.T) {
	f := newFixture(t)
	t.Run("ASS-V0-001 strict shape and scopes", func(t *testing.T) {
		valid := Encode(f.d)
		var object map[string]any
		json.Unmarshal(valid, &object)
		for _, key := range []string{"profile", "session_id", "capability_id", "author", "read_only", "write_paths", "guard_paths", "environment"} {
			copy := map[string]any{}
			for k, v := range object {
				copy[k] = v
			}
			delete(copy, key)
			data, _ := json.Marshal(copy)
			if _, err := DecodeDeclaration(data); err != ErrInput {
				t.Fatalf("missing %s: %v", key, err)
			}
		}
		for _, data := range [][]byte{bytes.Replace(valid, []byte(`"session_id":`), []byte(`"SESSION_ID":`), 1), bytes.Replace(valid, []byte(`"profile":`), []byte(`"profile":"duplicate","profile":`), 1), append(valid, []byte(`{}`)...)} {
			if _, err := DecodeDeclaration(data); err != ErrInput {
				t.Fatalf("malformed accepted: %s", data)
			}
		}
		for _, p := range []string{"../outside", "/absolute", ".git/config", "nested/.GIT/HEAD", "a*", "a?", "a\\b", "a//b", "a\n"} {
			d := f.d
			d.WritePaths = []string{p}
			if _, err := DecodeDeclaration(Encode(d)); err != ErrInput {
				t.Fatalf("path %q: %v", p, err)
			}
		}
		d := f.d
		d.Environment = []EnvironmentKey{{"TOKEN", "OUTWARD_WRITE"}}
		if _, err := DecodeDeclaration(Encode(d)); err != ErrInput {
			t.Fatal("outward declaration accepted")
		}
		d = f.d
		d.ReadOnly = []Checkout{{"readonly", f.root + "/nested"}}
		if _, err := DecodeDeclaration(Encode(d)); err != ErrInput {
			t.Fatal("nested root accepted")
		}
	})
	t.Run("ASS-V0-005 keys only preflight", func(t *testing.T) {
		for _, tc := range []struct {
			keys    *[]EnvironmentKey
			verdict string
			code    string
		}{{nil, "NOT_OBSERVED", ""}, {&[]EnvironmentKey{}, "PASS", ""}, {&[]EnvironmentKey{{"READ_KEY", "READ_ONLY"}}, "PASS", ""}, {&[]EnvironmentKey{{"OTHER", "NONE"}}, "FAIL", "ENVIRONMENT_UNDECLARED"}, {&[]EnvironmentKey{{"READ_KEY", "NONE"}}, "FAIL", "ENVIRONMENT_UNDECLARED"}, {&[]EnvironmentKey{{"TOKEN", "OUTWARD_WRITE"}}, "FAIL", "ENVIRONMENT_PROHIBITED"}, {&[]EnvironmentKey{{"TOKEN", "UNKNOWN"}}, "FAIL", "ENVIRONMENT_PROHIBITED"}} {
			h := f.h
			h.Environment = tc.keys
			v, findings := Environment(f.d, h)
			if v != tc.verdict || tc.code != "" && (len(findings) != 1 || findings[0].Code != tc.code) {
				t.Fatalf("%+v: %s %+v", tc, v, findings)
			}
		}
		h := f.h
		h.Environment = &[]EnvironmentKey{{"READ_KEY", "INVALID"}}
		if v, _ := Environment(f.d, h); v != "UNSUPPORTED" {
			t.Fatal(v)
		}
		h = f.h
		h.SessionID = "other"
		if _, err := DecodeHost(Encode(h), f.d); err != ErrInput {
			t.Fatal("cross session accepted")
		}
		h = f.h
		h.MetadataProtected = false
		if _, err := DecodeHost(Encode(h), f.d); err != ErrUnsupported {
			t.Fatal("unprotected metadata accepted")
		}
	})
}

func TestStepWrites(t *testing.T) {
	cases := []struct {
		name, code, path string
		change           func(*testing.T, fixture)
	}{
		{"allowed", "", "", func(t *testing.T, f fixture) {
			put(t, f.root+"/allowed/file", "changed")
			put(t, f.root+"/allowed/new", "new")
		}},
		{"guard", "GUARDED_WRITE", "allowed/guard", func(t *testing.T, f fixture) { put(t, f.root+"/allowed/guard", "blocked") }},
		{"outside edit", "OUT_OF_SCOPE_WRITE", "outside", func(t *testing.T, f fixture) { put(t, f.root+"/outside", "changed") }},
		{"outside add", "OUT_OF_SCOPE_WRITE", "new", func(t *testing.T, f fixture) { put(t, f.root+"/new", "changed") }},
		{"ignored", "OUT_OF_SCOPE_WRITE", "ignored", func(t *testing.T, f fixture) { put(t, f.root+"/ignored", "secret contents withheld") }},
		{"delete", "OUT_OF_SCOPE_WRITE", "outside", func(t *testing.T, f fixture) {
			if err := os.Remove(f.root + "/outside"); err != nil {
				t.Fatal(err)
			}
		}},
		{"rename", "OUT_OF_SCOPE_WRITE", "renamed", func(t *testing.T, f fixture) {
			if err := os.Rename(f.root+"/outside", f.root+"/renamed"); err != nil {
				t.Fatal(err)
			}
		}},
		{"mode", "OUT_OF_SCOPE_WRITE", "outside", func(t *testing.T, f fixture) {
			if err := os.Chmod(f.root+"/outside", 0700); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink", "OUT_OF_SCOPE_WRITE", "link", func(t *testing.T, f fixture) {
			if err := os.Symlink("outside", f.root+"/link"); err != nil {
				t.Fatal(err)
			}
		}},
		{"empty directory", "OUT_OF_SCOPE_WRITE", "empty", func(t *testing.T, f fixture) {
			if err := os.Mkdir(f.root+"/empty", 0700); err != nil {
				t.Fatal(err)
			}
		}},
		{"scope anchor", "SCOPE_ANCESTOR_CHANGED", "allowed", func(t *testing.T, f fixture) {
			if err := os.Chmod(f.root+"/allowed", 0755); err != nil {
				t.Fatal(err)
			}
		}},
		{"index", "INDEX_CHANGED", ".git/index", func(t *testing.T, f fixture) {
			put(t, f.root+"/allowed/file", "changed")
			gitTest(t, f.root, "add", "allowed/file")
		}},
		{"commit", "COMMIT_CHANGED", ".git/HEAD", func(t *testing.T, f fixture) {
			put(t, f.root+"/allowed/file", "changed")
			gitTest(t, f.root, "add", "allowed/file")
			gitTest(t, f.root, "commit", "-qm", "changed")
		}},
		{"admin", "ADMIN_CHANGED", "git/config", func(t *testing.T, f fixture) { gitTest(t, f.root, "config", "user.name", "Changed") }},
	}
	var vectors []struct {
		Operation string `json:"operation"`
		Code      string `json:"expected_code"`
		Path      string `json:"expected_path"`
		Exit      int    `json:"expected_exit"`
	}
	data, err := os.ReadFile("../../protocol/step/vectors/write-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(data, &vectors) != nil || len(vectors) != len(cases) {
		t.Fatal("violation vector matrix mismatch")
	}
	for i, vector := range vectors {
		if vector.Operation != cases[i].name || vector.Code != cases[i].code || vector.Path != cases[i].path || vector.Exit != (map[bool]int{true: 0, false: 1}[cases[i].code == ""]) {
			t.Fatalf("vector %d mismatch", i)
		}
	}
	for _, tc := range cases {
		t.Run("ASS-V0-003 "+tc.name, func(t *testing.T) {
			f := newFixture(t)
			put(t, f.root+"/outside", "initial dirt")
			s := snapshot(t, f)
			tc.change(t, f)
			r, code := verify(t, f, s)
			if tc.code == "" {
				if code != 0 || r.WriteScopeVerdict != "PASS" {
					t.Fatalf("allowed: %d %+v", code, r)
				}
			} else if code != 1 || !has(r, tc.code, tc.path) {
				t.Fatalf("expected %s %s: %d %+v", tc.code, tc.path, code, r)
			}
			if tc.name == "rename" && !has(r, "OUT_OF_SCOPE_WRITE", "outside") {
				t.Fatal("missing rename deletion")
			}
			if strings.Contains(string(Encode(r)), "secret contents withheld") {
				t.Fatal("raw file leaked")
			}
			again, code2 := verify(t, f, s)
			if code != code2 || !bytes.Equal(Encode(r), Encode(again)) {
				t.Fatal("nondeterministic receipt")
			}
		})
	}
	t.Run("ASS-V0-002 read only checkout", func(t *testing.T) {
		f := newFixture(t)
		ro := newFixture(t)
		f.d.ReadOnly = []Checkout{{"readonly", ro.root}}
		f.h.DeclarationDigest = DeclarationDigest(f.d)
		s := snapshot(t, f)
		put(t, ro.root+"/allowed/file", "changed")
		r, code := verify(t, f, s)
		if code != 1 || !has(r, "READ_ONLY_CHANGED", "allowed/file") {
			t.Fatalf("%d %+v", code, r)
		}
	})
	t.Run("ASS-V0-004 incomplete post retains writes", func(t *testing.T) {
		f := newFixture(t)
		s := snapshot(t, f)
		put(t, f.root+"/outside", "changed")
		put(t, f.root+"/.git/unsupported-admin", "unknown")
		r, code := verify(t, f, s)
		if code != 2 || r.AfterDigest != nil || r.WriteScopeVerdict != "UNSUPPORTED" || !has(r, "OUT_OF_SCOPE_WRITE", "outside") || !r.After[0].ContentComplete || r.After[0].ContentDigest == "" {
			t.Fatalf("%d %+v", code, r)
		}
	})
}

func TestStepStateAndUnsafe(t *testing.T) {
	t.Run("ASS-V0-004 corrupt binding refusal", func(t *testing.T) {
		f := newFixture(t)
		s := snapshot(t, f)
		for _, change := range []func(*State){func(s *State) { s.SessionID = "other" }, func(s *State) { s.DeclarationDigest = hash(nil) }, func(s *State) { s.Checkouts[0].Entries[1].Digest = hash(nil) }, func(s *State) { s.Complete = false }, func(s *State) { s.Checkouts[0].IndexDigest = hash(nil) }} {
			var copy State
			json.Unmarshal(Encode(s), &copy)
			change(&copy)
			if _, err := DecodeState(Encode(copy), f.d, f.h); err != ErrInput {
				t.Fatal("corrupt state accepted")
			}
		}
		f.h.GitBinaryDigest = hash(nil)
		if _, err := Snapshot(context.Background(), f.d, f.h); err != ErrUnsupported {
			t.Fatal("wrong executable digest accepted")
		}
	})
	t.Run("ASS-V0-006 unsupported profile and no execution", func(t *testing.T) {
		f := newFixture(t)
		trap := filepath.Join(t.TempDir(), "executed")
		script := "#!/bin/sh\ntouch " + trap + "\nexit 1\n"
		put(t, f.root+"/.git/hooks/post-checkout", script)
		os.Chmod(f.root+"/.git/hooks/post-checkout", 0700)
		put(t, f.root+"/.git/hooks/fsmonitor", script)
		os.Chmod(f.root+"/.git/hooks/fsmonitor", 0700)
		gitTest(t, f.root, "config", "core.fsmonitor", f.root+"/.git/hooks/fsmonitor")
		gitTest(t, f.root, "config", "filter.trap.clean", f.root+"/.git/hooks/post-checkout")
		put(t, f.root+"/.gitattributes", "* filter=trap\n")
		s := snapshot(t, f)
		r, code := verify(t, f, s)
		if code != 0 {
			t.Fatalf("%d %+v", code, r)
		}
		if _, err := os.Stat(trap); !os.IsNotExist(err) {
			t.Fatal("script executed")
		}
		put(t, f.root+"/.git/info/sparse-checkout", "outside\n")
		if _, err := Snapshot(context.Background(), f.d, f.h); err != ErrUnsupported {
			t.Fatal("sparse state accepted")
		}
	})
	for _, tc := range []struct {
		name   string
		change func(*testing.T, fixture)
	}{
		{"hardlink", func(t *testing.T, f fixture) {
			if err := os.Link(f.root+"/outside", f.root+"/link"); err != nil {
				t.Fatal(err)
			}
		}},
		{"scope symlink ancestor", func(t *testing.T, f fixture) {
			os.RemoveAll(f.root + "/allowed")
			if err := os.Symlink(".", f.root+"/allowed"); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing root", func(t *testing.T, f fixture) { os.RemoveAll(f.root) }},
		{"unsupported index", func(t *testing.T, f fixture) { put(t, f.root+"/.git/index", "DIRC malformed") }},
		{"config include", func(t *testing.T, f fixture) { gitTest(t, f.root, "config", "include.path", "/nonexistent") }},
	} {
		t.Run("ASS-V0-002 "+tc.name, func(t *testing.T) {
			f := newFixture(t)
			tc.change(t, f)
			s, err := Snapshot(context.Background(), f.d, f.h)
			if err != ErrUnsupported || s.Complete {
				t.Fatalf("%v %+v", err, s)
			}
		})
	}
}
