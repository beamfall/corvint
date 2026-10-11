package cli

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// stableReadPacket is the public S0E repository-envelope packet the Core read
// tests materialize: one ordinary sha1 repository, its artifacts and its sealed
// positive cem/1.0 map.
const stableReadPacket = "../../../protocol/cem-1.0/stable/repository-envelope-packet"

const stableReadAcceptCase = "normal-exit-lingering-descendant-contained"

type stableReadFixture struct {
	repo, artifacts, mapPath string
	expectedBase, target     string
	acceptGo                 string // the Go release whose public result is ACCEPT
}

// materializeStableRead builds the packet's repository under a fresh directory
// and writes the sealed map into its worktree as change.cem.json.
func materializeStableRead(t *testing.T) stableReadFixture {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("NOT_RUN: the stable verifier's owned Git lifecycle is observed on Darwin and Linux only")
	}
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(stableReadPacket, name))
		if err != nil {
			t.Fatalf("public packet: %v", err)
		}
		return data
	}
	var cases struct {
		Cases []struct {
			ID             string         `json:"id"`
			ExpectedResult map[string]any `json:"expectedResult"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(read("FULL-RESULT-CASES.json"), &cases); err != nil {
		t.Fatal(err)
	}
	fixture := stableReadFixture{}
	for _, c := range cases.Cases {
		if c.ID == stableReadAcceptCase {
			fixture.expectedBase, _ = c.ExpectedResult["expectedBase"].(string)
			fixture.target, _ = c.ExpectedResult["targetRevision"].(string)
			runtimeField, _ := c.ExpectedResult["runtime"].(map[string]any)
			fixture.acceptGo, _ = runtimeField["goVersion"].(string)
		}
	}
	if fixture.expectedBase == "" || fixture.target == "" || fixture.acceptGo == "" {
		t.Fatalf("public packet has no complete case %s", stableReadAcceptCase)
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture.repo, fixture.artifacts = filepath.Join(base, "repo"), filepath.Join(base, "artifacts")
	command := exec.Command("git", "init", "-q", "--object-format=sha1", fixture.repo)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.MkdirAll(fixture.artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, packName := range []string{"inherited/fixtures/FIXTURES.pack.json", "inherited/fixtures/FIXTURES-ledger-boundary.pack.json"} {
		var pack struct {
			Records []struct{ Path, Type, Sha256, Base64 string } `json:"records"`
		}
		if err := json.Unmarshal(read(packName), &pack); err != nil {
			t.Fatal(err)
		}
		for _, record := range pack.Records {
			if record.Type != "regular" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(record.Base64)
			if digest := sha256.Sum256(data); err != nil || hex.EncodeToString(digest[:]) != record.Sha256 {
				t.Fatalf("fixture record %s does not match its digest", record.Path)
			}
			switch {
			case strings.HasPrefix(record.Path, "fixtures/git-objects/sha1/"):
				oid := strings.TrimSuffix(filepath.Base(record.Path), ".object")
				if sum := sha1.Sum(data); hex.EncodeToString(sum[:]) != oid {
					t.Fatalf("object record %s is not the raw object of its name", record.Path)
				}
				var compressed bytes.Buffer
				writer := zlib.NewWriter(&compressed)
				writer.Write(data)
				writer.Close()
				path := filepath.Join(fixture.repo, ".git", "objects", oid[:2], oid[2:])
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, compressed.Bytes(), 0o444); err != nil {
					t.Fatal(err)
				}
			case strings.HasPrefix(record.Path, "fixtures/artifacts/"):
				if err := os.WriteFile(filepath.Join(fixture.artifacts, filepath.Base(record.Path)), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	fixture.mapPath = filepath.Join(fixture.repo, "change.cem.json")
	if err := os.WriteFile(fixture.mapPath, read("inherited/fixtures/maps/sha1-positive-sealed.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	return fixture
}

// runCoreRead runs one Core read through the CLI and decodes its stdout
// envelope, or its stderr refusal when the command produced no envelope.
func runCoreRead(t *testing.T, root string, arguments ...string) (exit int, envelope, refusal map[string]any) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exit = RunWithOCM(context.Background(), root, arguments, &stdout, &stderr, nil)
	if stdout.Len() != 0 {
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatalf("stdout is not one JSON envelope: %v: %s", err, stdout.Bytes())
		}
	}
	if stderr.Len() != 0 {
		if err := json.Unmarshal(stderr.Bytes(), &refusal); err != nil {
			t.Fatalf("stderr is not one JSON refusal: %v: %s", err, stderr.Bytes())
		}
	}
	return exit, envelope, refusal
}

// runVerifyStable runs the dedicated `cem verify-stable` command over the same
// inputs; it is the reference a Core read must reproduce exactly.
func runVerifyStable(t *testing.T, f stableReadFixture, artifacts string) (int, map[string]any) {
	t.Helper()
	arguments := []string{"cem", "verify-stable", "--repository", f.repo, "--map", f.mapPath, "--expected-base", f.expectedBase, "--target", f.target}
	if artifacts != "" {
		arguments = append(arguments, "--artifacts", artifacts)
	}
	var out bytes.Buffer
	exit, handled := RunStableInvocation(context.Background(), arguments, &out)
	if !handled {
		t.Fatal("verify-stable was not dispatched")
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("verify-stable output: %v: %s", err, out.Bytes())
	}
	return exit, result
}

func coreStableArguments(action string, f stableReadFixture, artifacts string) []string {
	arguments := []string{action, "--map", "change.cem.json", "--expected-base", f.expectedBase, "--target", f.target}
	if artifacts != "" {
		arguments = append(arguments, "--artifacts", artifacts)
	}
	return arguments
}

// requireCoreMatchesVerifyStable checks one Core read against verify-stable:
// the same exit, the complete unmodified 21-field result, and no 0.x
// verification, counts, worklist or ready-for-ci state (CEM-V1-017).
func requireCoreMatchesVerifyStable(t *testing.T, action string, f stableReadFixture, artifacts string) (int, map[string]any) {
	t.Helper()
	wantExit, want := runVerifyStable(t, f, artifacts)
	exit, envelope, refusal := runCoreRead(t, f.repo, coreStableArguments(action, f, artifacts)...)
	if refusal != nil {
		t.Fatalf("%s refused before the verifier: %v", action, refusal)
	}
	if exit != wantExit {
		t.Errorf("%s exit = %d, verify-stable exit = %d", action, exit, wantExit)
	}
	got, _ := envelope["stableVerification"].(map[string]any)
	if len(got) != 21 || !reflect.DeepEqual(got, want) {
		t.Errorf("%s stableVerification differs from verify-stable:\n got %v\nwant %v", action, got, want)
	}
	if envelope["ok"] != (wantExit == 0) || envelope["mutates"] != false || envelope["tool"] != "cem-"+action {
		t.Errorf("%s envelope head = ok %v mutates %v tool %v", action, envelope["ok"], envelope["mutates"], envelope["tool"])
	}
	for _, key := range []string{"verification", "counts", "worklist", "policyIssues", "selfModifiedAuthority"} {
		if _, present := envelope[key]; present {
			t.Errorf("%s synthesized the 0.x field %q for a stable map", action, key)
		}
	}
	keys := []string{"mutates", "ok", "stableVerification", "tool"}
	if action == "status" {
		keys = append(keys, "map", "state")
		if envelope["map"] != f.mapPath {
			t.Errorf("status map = %v, want %s", envelope["map"], f.mapPath)
		}
		states := map[string]string{"ACCEPT": "accepted", "REJECT": "rejected", "UNSUPPORTED": "unsupported"}
		if outcome, _ := want["outcome"].(string); envelope["state"] != states[outcome] {
			t.Errorf("status state = %v for outcome %s", envelope["state"], outcome)
		}
	}
	if len(envelope) != len(keys) {
		t.Errorf("%s envelope keys = %d, want exactly %v", action, len(envelope), keys)
	}
	return exit, got
}

// CEM-V1-017: a Core status or verify over an exact cem/1.0 map reproduces the
// stable verifier's complete result and exit, and a positive sealed map is
// accepted on the release its public result names.
func TestCoreStableReadMatchesVerifyStable(t *testing.T) {
	f := materializeStableRead(t)
	for _, action := range []string{"verify", "status"} {
		exit, result := requireCoreMatchesVerifyStable(t, action, f, f.artifacts)
		if runtime.Version() != f.acceptGo {
			t.Logf("NOT_RUN: ACCEPT outcome requires %s, running %s; equality with verify-stable still checked", f.acceptGo, runtime.Version())
			continue
		}
		if exit != 0 || result["outcome"] != "ACCEPT" || result["sidecar"] != "EXACT" {
			t.Errorf("%s = exit %d outcome %v sidecar %v, want ACCEPT EXACT exit 0", action, exit, result["outcome"], result["sidecar"])
		}
	}
}

// CEM-V1-017: the verifier's own refusal (exit 2) and rejection (exit 1) reach
// the Core caller unchanged, never re-derived from the envelope.
func TestCoreStableReadKeepsVerifierExit(t *testing.T) {
	f := materializeStableRead(t)
	for _, action := range []string{"verify", "status"} {
		exit, result := requireCoreMatchesVerifyStable(t, action, f, "")
		if exit != 2 || result["code"] != "artifacts-required" || result["stage"] != "authority-arguments" {
			t.Errorf("%s without --artifacts = exit %d code %v stage %v", action, exit, result["code"], result["stage"])
		}
	}
	entries, err := os.ReadDir(f.artifacts)
	if err != nil || len(entries) == 0 {
		t.Fatalf("fixture has no artifact to alter: %v", err)
	}
	path := filepath.Join(f.artifacts, entries[0].Name())
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		t.Fatalf("artifact %s: %v", path, err)
	}
	data[0] ^= 0x01
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"verify", "status"} {
		exit, result := requireCoreMatchesVerifyStable(t, action, f, f.artifacts)
		if runtime.Version() != f.acceptGo {
			t.Logf("NOT_RUN: artifact REJECT requires %s, running %s; equality with verify-stable still checked", f.acceptGo, runtime.Version())
			continue
		}
		issues, _ := result["issueCodes"].([]any)
		if exit != 1 || result["outcome"] != "REJECT" || len(issues) != 1 || issues[0] != "artifact-digest-mismatch" {
			t.Errorf("%s with an altered artifact = exit %d outcome %v issues %v", action, exit, result["outcome"], issues)
		}
	}
}

// CEM-V1-018: a stable map refuses the 0.x-only inputs, and a 0.x map refuses
// --artifacts, all before any verifier or Git work and with exit 2.
func TestCoreStableReadExplicitRefusals(t *testing.T) {
	f := materializeStableRead(t)
	stable := func(action string, extra ...string) []string {
		return append(coreStableArguments(action, f, f.artifacts), extra...)
	}
	legacy := filepath.Join(f.repo, "legacy.cem.json")
	if err := os.WriteFile(legacy, []byte(`{"spec":"cem/0.3"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name      string
		arguments []string
		code      string
		message   string
	}{
		{"verify patch", stable("verify", "--patch", "x.patch"), cemcode.InvalidArguments, "cem/1.0 verify does not accept --patch"},
		{"status patch", stable("status", "--patch", "x.patch"), cemcode.InvalidArguments, "cem/1.0 status does not accept --patch"},
		{"verify ceiling", stable("verify", "--max-unknown", "0"), cemcode.InvalidArguments, "cem/1.0 verify does not accept policy ceilings"},
		{"status ceiling", stable("status", "--max-mechanical", "1"), cemcode.InvalidArguments, "cem/1.0 status does not accept policy ceilings"},
	} {
		t.Run(c.name, func(t *testing.T) {
			exit, envelope, refusal := runCoreRead(t, f.repo, c.arguments...)
			if exit != 2 || envelope != nil || refusal["code"] != c.code {
				t.Fatalf("exit %d envelope %v refusal %v, want exit 2 code %s", exit, envelope, refusal, c.code)
			}
			if message, _ := refusal["error"].(string); !strings.Contains(message, c.message) {
				t.Fatalf("refusal %q does not name %q", message, c.message)
			}
		})
	}
	// The MCP report preview reads through the same workflow refusal; cem
	// report itself is read since V1-0632 S2 (CEM-V1-029..031).
	session, err := workflow.Open(f.repo)
	if err != nil {
		t.Fatal(err)
	}
	_, err = session.Read(context.Background(), workflow.ActionReportPreview, workflow.ReadOptions{MapPath: "change.cem.json", PatchGiven: true, ExpectedBase: f.expectedBase, Target: f.target})
	if cemcode.CodeOf(err) != cemcode.InvalidArguments || !strings.Contains(err.Error(), "cem/1.0 report-preview does not accept --patch") {
		t.Fatalf("report preview refusal = %v", err)
	}
	// A 0.x map never takes --artifacts; the refusal follows the map read, so a
	// malformed 0.x map keeps its own earlier failure.
	exit, envelope, refusal := runCoreRead(t, f.repo, "verify", "--map", "legacy.cem.json", "--artifacts", f.artifacts)
	if exit != 2 || envelope != nil || refusal["code"] == cemcode.InvalidArguments {
		t.Fatalf("malformed 0.x map with --artifacts: exit %d refusal %v, want its own parse failure", exit, refusal)
	}
}

// CEM-V1-018: an otherwise valid canonical 0.x map refuses --artifacts at
// stage 3, before expected-base and target are required.
func TestCoreStableReadRefusesArtifactsFor0x(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "legacy.cem.json"), legacyCanonicalMap(t), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"verify", "status"} {
		exit, envelope, refusal := runCoreRead(t, root, action, "--map", "legacy.cem.json", "--artifacts", "/artifacts")
		if exit != 2 || envelope != nil || refusal["code"] != cemcode.InvalidArguments {
			t.Fatalf("%s: exit %d refusal %v, want invalid-arguments", action, exit, refusal)
		}
		if message, _ := refusal["error"].(string); message != "cem/0.3 "+action+" does not accept --artifacts" {
			t.Fatalf("%s refusal = %q", action, message)
		}
	}
}

// CEM-V1-019: the stable route opens only for an exact cem/1.0 root; the
// candidate and every other refused spec keep their 0.x code and message.
func TestCoreStableRead0xRefusalsUnchanged(t *testing.T) {
	root := t.TempDir()
	for name, document := range map[string]string{
		"candidate.json": `{"spec":"cem/1.0-experimental.1"}`,
		"other.json":     `{"spec":"cem/9"}`,
		"padded.json":    `{"spec":"cem/1.0 "}`,
		"number.json":    `{"spec":1}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(document), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"verify", "status"} {
			exit, envelope, refusal := runCoreRead(t, root, action, "--map", name)
			if exit != 2 || envelope != nil || refusal["code"] != cemcode.UnsupportedSpec {
				t.Fatalf("%s %s: exit %d refusal %v", action, name, exit, refusal)
			}
			if refusal["error"] != `spec must be "cem/0.1", "cem/0.2", or "cem/0.3"` {
				t.Fatalf("%s %s: refusal message changed: %v", action, name, refusal["error"])
			}
		}
	}
}

// legacyCanonicalMap returns one parseable cem/0.3 map with a single unknown
// hunk; the refusal under test precedes every repository read.
func legacyCanonicalMap(t *testing.T) []byte {
	t.Helper()
	data := []byte(`{"spec":"cem/0.3","baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",` +
		`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c","excludedPath":".corvint/change.cem.json",` +
		`"evidence":[],"hunks":[{"id":"hunk:sha256:07461a992e03e064986720e365dc4bb477da7cefe73da853f51bcb70e2c3100c",` +
		`"path":"src/a.go","oldRange":{"start":1,"count":2},"newRange":{"start":4,"count":3},` +
		`"disposition":"unknown","reason":"no-evidence","basis":[]}]}`)
	if document, err := wire.ParseMap(data); err != nil || document.Spec != wire.Spec03 {
		t.Fatalf("legacy fixture does not parse as cem/0.3: %v", err)
	}
	return data
}
