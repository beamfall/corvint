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
	"io"
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

// stableReadTupleRoot holds one sealed packet per structural tuple admitted
// after the frozen packet's release (decision 0522, CEM-V1-050).
const stableReadTupleRoot = "../../../protocol/cem-1.0/structural-tuples"

// admittedAcceptGo returns the running release when its sealed tuple packet
// substitutes exactly the frozen packet's goVersion, so the public ACCEPT
// result holds there too; otherwise it returns the frozen release unchanged.
func admittedAcceptGo(frozen string) string {
	running := runtime.Version()
	data, err := os.ReadFile(filepath.Join(stableReadTupleRoot, running, "TUPLE.json"))
	if err != nil {
		return frozen
	}
	var tuple struct {
		Tuple            string `json:"tuple"`
		PredecessorTuple string `json:"predecessorTuple"`
		Substitution     struct {
			Field, From, To string
		} `json:"substitution"`
	}
	if json.Unmarshal(data, &tuple) != nil || tuple.Tuple != running || tuple.PredecessorTuple != frozen ||
		tuple.Substitution.Field != "expectedResult.runtime.goVersion" || tuple.Substitution.From != frozen || tuple.Substitution.To != running {
		return frozen
	}
	return running
}

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
	fixture.acceptGo = admittedAcceptGo(fixture.acceptGo)
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
	exit = Run(context.Background(), root, arguments, &stdout, &stderr)
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
	exit, handled := RunStableInvocation(context.Background(), arguments, &out, io.Discard)
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
// the same exit, the complete unmodified 23-field result, and no 0.x
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
	if len(got) != 23 || !reflect.DeepEqual(got, want) {
		t.Errorf("%s stableVerification differs from verify-stable:\n got %v\nwant %v", action, got, want)
	}
	if envelope["ok"] != (wantExit == 0) || envelope["mutates"] != false || envelope["tool"] != "cem-"+action {
		t.Errorf("%s envelope head = ok %v mutates %v tool %v", action, envelope["ok"], envelope["mutates"], envelope["tool"])
	}
	for _, key := range []string{"verification", "worklist", "policyIssues", "selfModifiedAuthority"} {
		if _, present := envelope[key]; present {
			t.Errorf("%s synthesized the 0.x field %q for a stable map", action, key)
		}
	}
	// Status always carries the map's disposition counts beside the result;
	// verify never does (CCP-V0-010).
	if _, present := envelope["counts"]; present != (action == "status") {
		t.Errorf("%s counts present = %v", action, present)
	}
	keys := []string{"mutates", "ok", "stableVerification", "tool"}
	if action == "status" {
		keys = append(keys, "counts", "map", "state")
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

// CEM-V1-018: a stable map refuses the 0.x-only inputs, and a retired 0.x map
// refuses unsupported-spec (CEM-V1-215), all before any verifier or Git work
// and with exit 2.
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
	// report itself is read since V1-0632 S2 (CEM-V1-034..036).
	session, err := workflow.Open(f.repo)
	if err != nil {
		t.Fatal(err)
	}
	_, err = session.Read(context.Background(), workflow.ActionReportPreview, workflow.ReadOptions{MapPath: "change.cem.json", PatchGiven: true, ExpectedBase: f.expectedBase, Target: f.target})
	if cemcode.CodeOf(err) != cemcode.InvalidArguments || !strings.Contains(err.Error(), "cem/1.0 report-preview does not accept --patch") {
		t.Fatalf("report preview refusal = %v", err)
	}
	// A retired 0.x map refuses unsupported-spec with the fix line whatever
	// else is passed (CEM-V1-215).
	exit, envelope, refusal := runCoreRead(t, f.repo, "verify", "--map", "legacy.cem.json", "--artifacts", f.artifacts)
	if exit != 2 || envelope != nil || refusal["code"] != cemcode.UnsupportedSpec || !strings.Contains(refusal["error"].(string), wire.RetiredSpecFix) {
		t.Fatalf("0.x map with --artifacts: exit %d refusal %v, want unsupported-spec with the fix line", exit, refusal)
	}
}

// TestCEMV1145StatusCeilings checks the stable status policy ceilings: counts
// and policyIssues sit beside the unmodified verifier result, a ceiling the map
// meets keeps exit 0, an exceeded one exits 1 with ok false, and verify still
// refuses ceilings (CEM-V1-145).
func TestCEMV1145StatusCeilings(t *testing.T) {
	f := materializeStableRead(t)
	// Without ceilings the status keeps the CEM-V1-017 shape plus counts (CCP-V0-010).
	baseline, verified := requireCoreMatchesVerifyStable(t, "status", f, f.artifacts)
	if baseline != 0 {
		t.Fatalf("baseline exit %d", baseline)
	}
	met := append(coreStableArguments("status", f, f.artifacts), "--max-unknown", "0", "--max-mechanical", "1")
	exit, envelope, refusal := runCoreRead(t, f.repo, met...)
	if exit != 0 || refusal != nil || envelope["ok"] != true || envelope["state"] != "accepted" {
		t.Fatalf("met ceilings: exit %d envelope %v refusal %v", exit, envelope, refusal)
	}
	counts, _ := envelope["counts"].(map[string]any)
	if counts["mechanical"] != float64(1) || counts["unknown"] != float64(0) {
		t.Fatalf("counts = %v", counts)
	}
	if issues, _ := envelope["policyIssues"].([]any); len(issues) != 0 {
		t.Fatalf("policyIssues = %v", issues)
	}
	if !reflect.DeepEqual(envelope["stableVerification"], verified) {
		t.Fatal("ceilings changed the embedded verifier result")
	}
	exceeded := append(coreStableArguments("status", f, f.artifacts), "--max-mechanical", "0")
	exit, envelope, refusal = runCoreRead(t, f.repo, exceeded...)
	if exit != 1 || refusal != nil || envelope["ok"] != false || envelope["state"] != "accepted" {
		t.Fatalf("exceeded ceiling: exit %d envelope %v refusal %v", exit, envelope, refusal)
	}
	if issues, _ := envelope["policyIssues"].([]any); len(issues) != 1 {
		t.Fatalf("policyIssues = %v", issues)
	}
	exit, envelope, refusal = runCoreRead(t, f.repo, append(coreStableArguments("verify", f, f.artifacts), "--max-unknown", "0")...)
	if exit != 2 || envelope != nil || refusal["code"] != cemcode.InvalidArguments {
		t.Fatalf("verify ceiling: exit %d envelope %v refusal %v", exit, envelope, refusal)
	}
}

// TestCEMV1215ReadsRefuseEveryNonStableSpec checks CEM-V1-215: verify, status
// and report admit only an exact cem/1.0 root. A retired cem/0.x map refuses
// unsupported-spec with the RetiredSpecFix line, with or without --artifacts,
// and every other spec value refuses unsupported-spec; none reads Git.
func TestCEMV1215ReadsRefuseEveryNonStableSpec(t *testing.T) {
	root := t.TempDir()
	documents := map[string]string{
		"cem01.json":     `{"spec":"cem/0.1"}`,
		"cem02.json":     `{"spec":"cem/0.2"}`,
		"cem03.json":     `{"spec":"cem/0.3","hunks":[]}`,
		"candidate.json": `{"spec":"cem/1.0-experimental.1"}`,
		"other.json":     `{"spec":"cem/9"}`,
		"padded.json":    `{"spec":"cem/1.0 "}`,
		"number.json":    `{"spec":1}`,
	}
	for name, document := range documents {
		if err := os.WriteFile(filepath.Join(root, name), []byte(document), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, document := range documents {
		retired := strings.HasPrefix(document, `{"spec":"cem/0.`)
		for _, arguments := range [][]string{
			{"verify", "--map", name}, {"status", "--map", name}, {"report", "--map", name, "--format", "json"},
			{"verify", "--map", name, "--artifacts", "/artifacts"}, {"report", "--map", name, "--format", "markdown", "--artifacts", "/artifacts"},
		} {
			exit, envelope, refusal := runCoreRead(t, root, arguments...)
			if exit != 2 || envelope != nil || refusal["code"] != cemcode.UnsupportedSpec {
				t.Fatalf("%v: exit %d refusal %v, want unsupported-spec", arguments, exit, refusal)
			}
			message, _ := refusal["error"].(string)
			if retired != strings.Contains(message, wire.RetiredSpecFix) {
				t.Fatalf("%v: refusal %q, retired %v", arguments, message, retired)
			}
		}
	}
}

// TestCEMV1216CommandSpecRefusals drives `cem begin` and `cem prepare` --spec
// through the command parser: a retired label reaches the producer and refuses
// unsupported-spec with the recovery line, any other non-stable value refuses
// invalid-arguments, and neither writes a map (CEM-V1-216).
func TestCEMV1216CommandSpecRefusals(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	for _, c := range []struct{ spec, code string }{
		{"cem/0.1", cemcode.UnsupportedSpec},
		{"cem/0.2", cemcode.UnsupportedSpec},
		{"cem/0.3", cemcode.UnsupportedSpec},
		{"cem/1.0 ", cemcode.InvalidArguments},
		{wire.CandidateSpec, cemcode.InvalidArguments},
	} {
		for _, arguments := range [][]string{
			{"begin", "--patch", "change.patch", "--output", "refused.cem.json", "--spec", c.spec},
			{"prepare", "--base", "HEAD", "--target", "HEAD", "--spec", c.spec},
		} {
			exit, envelope, refusal := runCoreRead(t, root, arguments...)
			message, _ := refusal["error"].(string)
			if exit != 2 || envelope != nil || refusal["code"] != c.code ||
				(c.code == cemcode.UnsupportedSpec) != strings.Contains(message, wire.RetiredSpecFix) {
				t.Fatalf("%s --spec %q: exit %d envelope %v refusal %v", arguments[0], c.spec, exit, envelope, refusal)
			}
		}
	}
	for _, written := range []string{"refused.cem.json", filepath.FromSlash(wire.ExcludedCEMPath)} {
		if _, err := os.Lstat(filepath.Join(root, written)); !os.IsNotExist(err) {
			t.Fatalf("a refused --spec wrote %s (%v)", written, err)
		}
	}
}
