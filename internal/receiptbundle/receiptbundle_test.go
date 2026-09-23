package receiptbundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

const mapPath = ".corvint/changes/fixture.cem.json"

type fixture struct {
	root, base, target, cem string
}

func fixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newFixture commits a base and a target and writes a CEM for base..target.
func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	fixtureGit(t, root, "init", "-q", "-b", "main")
	writeFixture(t, filepath.Join(root, "a.txt"), "base\n")
	fixtureGit(t, root, "add", ".")
	fixtureGit(t, root, "commit", "-qm", "base")
	base := fixtureGit(t, root, "rev-parse", "HEAD")
	writeFixture(t, filepath.Join(root, "a.txt"), "target\n")
	fixtureGit(t, root, "commit", "-qam", "target")
	target := fixtureGit(t, root, "rev-parse", "HEAD")
	cem := `{"spec":"cem/0.2","baseRevision":"` + base + `",` +
		`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",` +
		`"excludedPath":".corvint/change.cem.json","evidence":[],"hunks":[]}`
	writeFixture(t, filepath.Join(root, mapPath), cem)
	return fixture{root: root, base: base, target: target, cem: cem}
}

func sum(data string) string {
	digest := sha256.Sum256([]byte(data))
	return hex.EncodeToString(digest[:])
}

func export(t *testing.T, f fixture, witness string) (string, map[string]any) {
	t.Helper()
	output := filepath.Join(t.TempDir(), "bundle")
	envelope, err := Export(context.Background(), f.root, Options{
		MapPath: mapPath, Target: f.target, Output: output, Witness: witness,
	})
	if err != nil {
		t.Fatal(err)
	}
	return output, envelope
}

// manifestReceipts parses the manifest and checks its line shape: a header
// line, one receipt per line, and a closing line (RCB-V0-002).
func manifestReceipts(t *testing.T, bundle string) (map[string]any, []map[string]any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(bundle, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Profile, Base, Target string
		Receipts              []map[string]any
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != len(manifest.Receipts)+2 || lines[len(lines)-1] != "]}" {
		t.Fatalf("manifest is not one receipt per line:\n%s", data)
	}
	byKind := map[string]any{}
	for _, receipt := range manifest.Receipts {
		byKind[receipt["kind"].(string)] = receipt
	}
	byKind["profile"], byKind["base"], byKind["target"] = manifest.Profile, manifest.Base, manifest.Target
	return byKind, manifest.Receipts
}

func TestExportCopiesBoundReceiptsAndListsTheRestAbsent(t *testing.T) {
	f := newFixture(t)
	dogfood := `{"profile":"corvint-dogfood-change/0","base":"` + f.base + `","target":"` + f.target +
		`","steps":[{"name":"affected","status":"PRODUCED"},{"name":"witness","status":"NOT_PRODUCED","reason":"x"}]}`
	writeFixture(t, filepath.Join(f.root, ".corvint", "dogfood-report.json"), dogfood)
	writeFixture(t, filepath.Join(f.root, ".git", "corvint", "release-gate-receipt"),
		"corvint-gate-receipt/0 "+f.base+" tree digest\n")

	bundle, envelope := export(t, f, "")
	manifest, receipts := manifestReceipts(t, bundle)
	if manifest["profile"] != Profile || manifest["base"] != f.base || manifest["target"] != f.target {
		t.Fatalf("manifest header = %v", manifest)
	}
	kinds := []string{}
	for _, receipt := range receipts {
		kinds = append(kinds, receipt["kind"].(string))
	}
	if !reflect.DeepEqual(kinds, []string{"cem", "witness", "dogfood", "gate-receipt"}) {
		t.Fatalf("receipt order = %v", kinds)
	}
	cem := manifest["cem"].(map[string]any)
	if cem["state"] != "present" || cem["sha256"] != sum(f.cem) || cem["source"] != mapPath ||
		cem["base"] != f.base || cem["target"] != f.target || len(cem["notRunOrNotProduced"].([]any)) != 0 {
		t.Fatalf("cem entry = %v", cem)
	}
	wantDogfood := []any{map[string]any{"pointer": "/steps/1/status", "value": "NOT_PRODUCED"}}
	entry := manifest["dogfood"].(map[string]any)
	if entry["sha256"] != sum(dogfood) || !reflect.DeepEqual(entry["notRunOrNotProduced"], wantDogfood) {
		t.Fatalf("dogfood entry = %v", entry)
	}
	absent := map[string]string{"witness": ReasonNotSupplied, "gate-receipt": ReasonOtherRevision}
	for kind, reason := range absent {
		got := manifest[kind].(map[string]any)
		if got["state"] != "absent" || got["reason"] != reason || got["sha256"] != nil {
			t.Fatalf("%s entry = %v, want absent %s", kind, got, reason)
		}
	}
	for file, want := range map[string]string{"receipts/cem.json": f.cem, "receipts/dogfood-report.json": dogfood} {
		got, err := os.ReadFile(filepath.Join(bundle, file))
		if err != nil || string(got) != want {
			t.Fatalf("%s is not an exact copy: %v", file, err)
		}
	}
	if _, err := os.Stat(filepath.Join(bundle, "receipts", "gate-receipt.txt")); !os.IsNotExist(err) {
		t.Fatalf("an absent receipt was written: %v", err)
	}
	manifestBytes, _ := os.ReadFile(filepath.Join(bundle, ManifestName))
	if envelope["manifestSha256"] != sum(string(manifestBytes)) || envelope["mutates"] != false {
		t.Fatalf("envelope = %v", envelope)
	}
}

func TestExportBindsTheWitnessAndGateReceiptToTheTarget(t *testing.T) {
	f := newFixture(t)
	witness := func(head string) string {
		path := filepath.Join(t.TempDir(), "witness.json")
		writeFixture(t, path, `{"profile":"corvint-witness/0","range":{"base":"`+f.base[:12]+`","head":"`+head+
			`"},"summary":{"verdict":"NOT_RUN"}}`)
		return path
	}
	writeFixture(t, filepath.Join(f.root, ".git", "corvint", "release-gate-receipt"),
		"corvint-gate-receipt/0 "+f.target+" tree digest\n")

	bundle, _ := export(t, f, witness(f.target))
	manifest, _ := manifestReceipts(t, bundle)
	entry := manifest["witness"].(map[string]any)
	want := []any{map[string]any{"pointer": "/summary/verdict", "value": "NOT_RUN"}}
	if entry["state"] != "present" || !reflect.DeepEqual(entry["notRunOrNotProduced"], want) {
		t.Fatalf("witness entry = %v", entry)
	}
	gate := manifest["gate-receipt"].(map[string]any)
	if gate["state"] != "present" || gate["base"] != nil || gate["target"] != f.target {
		t.Fatalf("gate entry = %v", gate)
	}

	bundle, _ = export(t, f, witness(f.base))
	manifest, _ = manifestReceipts(t, bundle)
	if got := manifest["witness"].(map[string]any)["reason"]; got != ReasonOtherRevision {
		t.Fatalf("witness for another head reason = %v", got)
	}
	bundle, _ = export(t, f, filepath.Join(t.TempDir(), "absent.json"))
	manifest, _ = manifestReceipts(t, bundle)
	if got := manifest["witness"].(map[string]any)["reason"]; got != ReasonNotFound {
		t.Fatalf("absent witness reason = %v", got)
	}

	outside := filepath.Join(t.TempDir(), "dogfood.json")
	writeFixture(t, outside, `{"profile":"corvint-dogfood-change/0","base":"`+f.base+`","target":"`+f.target+`"}`)
	if err := os.Symlink(outside, filepath.Join(f.root, ".corvint", "dogfood-report.json")); err != nil {
		t.Fatal(err)
	}
	bundle, _ = export(t, f, "")
	manifest, _ = manifestReceipts(t, bundle)
	if got := manifest["dogfood"].(map[string]any)["reason"]; got != ReasonUnreadable {
		t.Fatalf("symlinked dogfood report reason = %v", got)
	}
}

func TestExportRefusesOutputsItMustNotWrite(t *testing.T) {
	f := newFixture(t)
	existing := t.TempDir()
	cases := map[string]string{
		"relative":      "bundle",
		"existing":      existing,
		"worktree":      filepath.Join(f.root, "bundle"),
		"git directory": filepath.Join(f.root, ".git", "bundle"),
		"no parent":     filepath.Join(existing, "missing", "bundle"),
	}
	for name, output := range cases {
		_, err := Export(context.Background(), f.root, Options{MapPath: mapPath, Target: f.target, Output: output})
		if cemcode.CodeOf(err) != CodeOutputRefused {
			t.Fatalf("%s: %v, want %s", name, err, CodeOutputRefused)
		}
	}
	if _, err := os.Stat(filepath.Join(f.root, "bundle")); !os.IsNotExist(err) {
		t.Fatalf("a refused output was created: %v", err)
	}
}

func TestExportRequiresAValidMap(t *testing.T) {
	f := newFixture(t)
	for name, content := range map[string]string{"missing": "", "invalid": `{"spec":"cem/0.2"}`} {
		os.Remove(filepath.Join(f.root, mapPath))
		if content != "" {
			writeFixture(t, filepath.Join(f.root, mapPath), content)
		}
		output := filepath.Join(t.TempDir(), "bundle")
		_, err := Export(context.Background(), f.root, Options{MapPath: mapPath, Target: f.target, Output: output})
		if err == nil {
			t.Fatalf("%s map exported", name)
		}
		if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
			t.Fatalf("%s map left a bundle: %v", name, statErr)
		}
	}
	os.Remove(filepath.Join(f.root, mapPath))
	_, err := Export(context.Background(), f.root, Options{MapPath: mapPath, Target: f.target, Output: filepath.Join(t.TempDir(), "b")})
	if cemcode.CodeOf(err) != cemcode.MapUnavailable {
		t.Fatalf("missing map: %v", err)
	}
}
