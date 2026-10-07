package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	js "github.com/Beamfall/corvint/internal/jstestprovider"
)

// appmapCLIRepo commits the internal/appmap fixture application into a fresh repository.
func appmapCLIRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join("..", "..", "internal", "appmap", "testdata", "fixture")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return root
}

// AMAP-V0-009 AMAP-V0-011 AMAP-V0-015: the CLI builds the map to stdout and serves every
// projection within its default cap, without writing to the repository.
func TestAMAPV0FlowsAppmapCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diagnostic := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 || !strings.HasPrefix(out, `{"schema":"application-map/0"`) {
		t.Fatalf("build %d %s", code, diagnostic)
	}
	mapFile := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(mapFile, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args []string
		cap  int
	}{
		{[]string{"screen", "--screen", "/#!/clubs/7/teesheets"}, 4096},
		{[]string{"flow", "--flow", "book-tee-time"}, 6144},
		{[]string{"find", "--text", "teesheet"}, 2048},
		{[]string{"scaffold", "--flow", "book-tee-time"}, 6144},
		{[]string{"screen", "--screen", "app.home", "--budget", "2048"}, 2048},
	} {
		code, out, diagnostic := runFlowsCLI(root, append([]string{"appmap", c.args[0], "--map", mapFile}, c.args[1:]...)...)
		var doc map[string]any
		if code != 0 || len(out) > c.cap || json.Unmarshal([]byte(out), &doc) != nil {
			t.Fatalf("%v: %d %d bytes %s", c.args, code, len(out), diagnostic)
		}
		if _, ok := doc["omitted"].(map[string]any); !ok {
			t.Fatalf("%v: no omitted counts", c.args)
		}
	}
	if code, out, _ := runFlowsCLI(root, "appmap", "screen", "--map", mapFile, "--screen", "app.home", "--full"); code != 0 || !strings.Contains(out, `"full":true`) {
		t.Fatalf("--full %d", code)
	}
	for _, args := range [][]string{
		{"appmap"},
		{"appmap", "plan"},
		{"appmap", "build"},
		{"appmap", "screen", "--map", mapFile},
		{"appmap", "screen", "--map", mapFile, "--screen", "app.home", "--budget", "4096", "--full"},
		{"appmap", "screen", "--map", mapFile, "--screen", "app.home", "--budget", "10"},
		{"appmap", "build", "--manifest", "appmap.json", "--revision", "no-such-rev"},
	} {
		if code, out, _ := runFlowsCLI(root, args...); code != 2 || out != "" {
			t.Errorf("%v: %d %q", args, code, out)
		}
	}
	cmd := exec.Command("git", "status", "--porcelain", "--ignored")
	cmd.Dir = root
	if status, err := cmd.Output(); err != nil || len(status) != 0 {
		t.Fatalf("worktree changed: %s %v", status, err)
	}
}

// rvnReceipt writes a qualified corvint-playwright-external/3 receipt with one passing outcome
// against application revision rev and returns its path and test key.
func rvnReceipt(t *testing.T, rev string) (string, string) {
	t.Helper()
	sum := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	config, build := sum([]byte("module.exports={};\n")), sum([]byte("app.js"))
	r := js.Receipt{Profile: js.AttemptExternalProfile, Kind: "e2e",
		Identity: js.Identity{ConfigFile: "/fixture/config.cjs", ConfigDigest: config, ConfigInputDigests: map[string]string{"/fixture/config.cjs": config},
			TestFileDigests: map[string]string{"/fixture/test.cjs": sum([]byte("test"))}, PackageDigest: sum([]byte("package")), NodeVersion: "v22.23.2",
			RunnerName: "playwright", RunnerVersion: "1.63.0", Environment: map[string]string{}, Argv: []string{"node", "playwright"}},
		External: &js.ExternalLifecycle{ReadyURL: "http://127.0.0.1:8000", DeclaredAppIdentity: rev, Ownership: "external", CleanupResponsibility: "external",
			ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true, ConfigOverride: "synthetic fixture"},
		AppBuildAtStart: js.AppBuildIdentity{Digest: build}, AppBuildAtPublish: js.AppBuildIdentity{Digest: build},
		Schedule: &js.ExecutionSchedule{Workers: 1, Starts: []js.ExecutionStart{{FullName: "t0", File: "/fixture/test.cjs", Line: 1}}}}
	use := json.RawMessage(`{"browserName":"chromium","channel":"","headless":true,"launchOptions":{},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome for Testing 153.0.8010.12","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/portable/cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282","browserRevision":"1243","manifestBrowserVersion":"153.0.8010.12","headlessShellAvailable":true}}`)
	a := &js.Anchor{File: "/fixture/test.cjs", Line: 1}
	o := js.TestOutcome{Name: "t0", FullName: "t0", State: js.StatePassed, Anchor: a, Project: &js.ProjectIdentity{Browser: "chromium", Device: "unknown", Use: use, ConfigDigest: config},
		Attempts: []js.Attempt{{State: js.StatePassed, FailureKind: "none"}}, AttemptDetails: []js.AttemptDetail{{State: js.StatePassed, Anchor: a}}}
	id, _ := json.Marshal(struct {
		Identity js.Identity
		Project  *js.ProjectIdentity
		Anchor   *js.Anchor
		FullName string
	}{r.Identity, o.Project, o.Anchor, o.FullName})
	o.ID = sum(id)
	r.Tests = []js.TestOutcome{o}
	raw, err := js.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(name, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return name, o.ID
}

// RVN-V0-002 RVN-V0-006: `flows appmap screen|flow` print a per-step run-verification learned fact
// from --receipt and --bind within their caps, and refuse malformed or unbindable inputs with exit 2.
func TestRVNV0FlowsAppmapVerificationCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diagnostic := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatalf("build %d %s", code, diagnostic)
	}
	mapFile := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(mapFile, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	receipt, key := rvnReceipt(t, strings.TrimSpace(string(head)))
	bind := "step:book-tee-time/select-slot=" + key
	for _, c := range []struct {
		args []string
		cap  int
	}{
		{[]string{"flow", "--flow", "book-tee-time"}, 6144},
		{[]string{"screen", "--screen", "app.clubs.teesheets"}, 4096},
	} {
		args := append([]string{"appmap", c.args[0], "--map", mapFile}, c.args[1:]...)
		code, out, diagnostic := runFlowsCLI(root, append(args, "--receipt", receipt, "--bind", bind)...)
		if code != 0 || len(out) > c.cap || !strings.Contains(out, `"source":"run-verification","kind":"VERIFIED"`) {
			t.Fatalf("%v: %d %d bytes %s %s", c.args, code, len(out), diagnostic, out)
		}
	}
	garbage := filepath.Join(t.TempDir(), "garbage.json")
	if err := os.WriteFile(garbage, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	flow := []string{"appmap", "flow", "--map", mapFile, "--flow", "book-tee-time"}
	for want, extra := range map[string][]string{
		"appmap-invalid-query":          {"--bind", bind},
		"appmap-verify-invalid-receipt": {"--receipt", garbage},
		"appmap-verify-test-absent":     {"--receipt", receipt, "--bind", "step:book-tee-time/select-slot=" + strings.Repeat("a", 64)},
		"appmap-verify-unknown-step":    {"--receipt", receipt, "--bind", "step:book-tee-time/none=" + key},
	} {
		code, out, diagnostic := runFlowsCLI(root, append(append([]string{}, flow...), extra...)...)
		if code != 2 || out != "" || !strings.Contains(diagnostic, want) {
			t.Errorf("%v: %d %q %s, want %s", extra, code, out, diagnostic, want)
		}
	}
	cmd := exec.Command("git", "status", "--porcelain", "--ignored")
	cmd.Dir = root
	if status, err := cmd.Output(); err != nil || len(status) != 0 {
		t.Fatalf("worktree changed: %s %v", status, err)
	}
}
