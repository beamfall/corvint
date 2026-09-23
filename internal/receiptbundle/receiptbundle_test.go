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
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

const mapPath = wire.ExcludedCEMPath

type fixture struct {
	root, base, target, tree, cem string
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

// newFixture commits a base and a change, prepares the canonical CEM for
// them, and commits it: the bind commit is the export target (RCB-V0-004).
func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "init", "-q", "-b", "main")
	writeFixture(t, filepath.Join(root, "a.txt"), "base\n")
	fixtureGit(t, root, "add", ".")
	fixtureGit(t, root, "commit", "-qm", "base")
	base := fixtureGit(t, root, "rev-parse", "HEAD")
	writeFixture(t, filepath.Join(root, "a.txt"), "target\n")
	fixtureGit(t, root, "commit", "-qam", "target")
	change := fixtureGit(t, root, "rev-parse", "HEAD")
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(context.Background(), workflow.PrepareOptions{Base: base, Target: change}); err != nil {
		t.Fatal(err)
	}
	cem, err := os.ReadFile(filepath.Join(root, mapPath))
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "add", mapPath)
	fixtureGit(t, root, "commit", "-qm", "bind")
	target := fixtureGit(t, root, "rev-parse", "HEAD")
	tree := fixtureGit(t, root, "rev-parse", target+"^{tree}")
	return fixture{root: root, base: base, target: target, tree: tree, cem: string(cem)}
}

// gateReceipt is the canonical GOC-V0-010 line for commit and tree.
func gateReceipt(commit, tree string) string {
	return "corvint-gate-receipt/0 " + commit + " " + tree + " " + strings.Repeat("0", 64) + "\n"
}

func sum(data string) string {
	digest := sha256.Sum256([]byte(data))
	return hex.EncodeToString(digest[:])
}

func export(t *testing.T, f fixture, witness string) (string, map[string]any) {
	t.Helper()
	output := filepath.Join(t.TempDir(), "bundle")
	envelope, err := Export(context.Background(), f.root, Options{
		MapPath: mapPath, ExpectedBase: f.base, Target: f.target, Output: output, Witness: witness,
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
	writeFixture(t, filepath.Join(f.root, ".git", "corvint", "release-gate-receipt"), gateReceipt(f.base, f.tree))

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
	witness := func(base, head string) string {
		path := filepath.Join(t.TempDir(), "witness.json")
		writeFixture(t, path, `{"profile":"corvint-witness/0","range":{"base":"`+base+`","head":"`+head+
			`"},"summary":{"verdict":"NOT_RUN"}}`)
		return path
	}
	gatePath := filepath.Join(f.root, ".git", "corvint", "release-gate-receipt")
	writeFixture(t, gatePath, gateReceipt(f.target, f.tree))

	bundle, _ := export(t, f, witness(f.base, f.target))
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

	// Receipt revisions bind only as full object IDs, never resolved (RCB-V0-004).
	witnesses := map[string]string{
		"other head": witness(f.base, f.base), "symbolic head": witness(f.base, "HEAD"),
		"short base": witness(f.base[:12], f.target), "absent": filepath.Join(t.TempDir(), "absent.json"),
	}
	witnessReasons := map[string]string{
		"other head": ReasonOtherRevision, "symbolic head": ReasonOtherRevision,
		"short base": ReasonOtherRevision, "absent": ReasonNotFound,
	}
	for name, path := range witnesses {
		bundle, _ = export(t, f, path)
		manifest, _ = manifestReceipts(t, bundle)
		if got := manifest["witness"].(map[string]any)["reason"]; got != witnessReasons[name] {
			t.Fatalf("%s witness reason = %v, want %s", name, got, witnessReasons[name])
		}
	}

	// The gate receipt must be the exact canonical line for the target and its tree.
	gates := map[string]string{
		"leading space":  " " + gateReceipt(f.target, f.tree),
		"crlf":           strings.TrimSuffix(gateReceipt(f.target, f.tree), "\n") + "\r\n",
		"no final lf":    strings.TrimSuffix(gateReceipt(f.target, f.tree), "\n"),
		"symbolic":       gateReceipt("HEAD", f.tree),
		"other tree":     gateReceipt(f.target, f.target),
		"other revision": gateReceipt(f.base, f.tree),
	}
	gateReasons := map[string]string{"other tree": ReasonOtherRevision, "other revision": ReasonOtherRevision}
	for name, content := range gates {
		writeFixture(t, gatePath, content)
		bundle, _ = export(t, f, "")
		manifest, _ = manifestReceipts(t, bundle)
		want := gateReasons[name]
		if want == "" {
			want = ReasonUnreadable
		}
		if got := manifest["gate-receipt"].(map[string]any)["reason"]; got != want {
			t.Fatalf("%s gate receipt reason = %v, want %s", name, got, want)
		}
	}

	dogfoodPath := filepath.Join(f.root, ".corvint", "dogfood-report.json")
	dogfoods := map[string]string{
		"other revision": `{"profile":"corvint-dogfood-change/0","base":"` + f.base + `","target":"` + f.base + `"}`,
		"symbolic":       `{"profile":"corvint-dogfood-change/0","base":"` + f.base + `","target":"HEAD"}`,
		"wrong profile":  `{"profile":"corvint-witness/0","base":"` + f.base + `","target":"` + f.target + `"}`,
		"oversize": `{"profile":"corvint-dogfood-change/0","base":"` + f.base + `","target":"` + f.target + `"}` +
			strings.Repeat(" ", MaxReceiptBytes),
	}
	dogfoodReasons := map[string]string{
		"other revision": ReasonOtherRevision, "symbolic": ReasonOtherRevision,
		"wrong profile": ReasonUnreadable, "oversize": ReasonUnreadable,
	}
	for name, content := range dogfoods {
		writeFixture(t, dogfoodPath, content)
		bundle, _ = export(t, f, "")
		manifest, _ = manifestReceipts(t, bundle)
		if got := manifest["dogfood"].(map[string]any)["reason"]; got != dogfoodReasons[name] {
			t.Fatalf("%s dogfood reason = %v, want %s", name, got, dogfoodReasons[name])
		}
	}

	os.Remove(dogfoodPath)
	outside := filepath.Join(t.TempDir(), "dogfood.json")
	writeFixture(t, outside, `{"profile":"corvint-dogfood-change/0","base":"`+f.base+`","target":"`+f.target+`"}`)
	if err := os.Symlink(outside, dogfoodPath); err != nil {
		t.Fatal(err)
	}
	bundle, _ = export(t, f, "")
	manifest, _ = manifestReceipts(t, bundle)
	if got := manifest["dogfood"].(map[string]any)["reason"]; got != ReasonUnreadable {
		t.Fatalf("symlinked dogfood report reason = %v", got)
	}
}

// TestNotRunAxesEscapePointersAndSortKeys: axes are listed in sorted-key
// order with RFC 6901 escaping, and include the discrimination not-run value
// (RCB-V0-003).
func TestNotRunAxesEscapePointersAndSortKeys(t *testing.T) {
	axes, err := notRunAxes([]byte(`{"z":"NOT_RUN","a/b":{"c~d":"not-run"},"m":["NOT_PRODUCED","PASS"]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []Axis{{"/a~1b/c~0d", "not-run"}, {"/m/0", "NOT_PRODUCED"}, {"/z", "NOT_RUN"}}
	if !reflect.DeepEqual(axes, want) {
		t.Fatalf("axes = %v, want %v", axes, want)
	}
}

func TestExportRefusesOutputsItMustNotWrite(t *testing.T) {
	f := newFixture(t)
	existing := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(f.root, link); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"relative":         "bundle",
		"existing":         existing,
		"worktree":         filepath.Join(f.root, "bundle"),
		"git directory":    filepath.Join(f.root, ".git", "bundle"),
		"no parent":        filepath.Join(existing, "missing", "bundle"),
		"symlinked parent": filepath.Join(link, "bundle"),
	}
	for name, output := range cases {
		refuse(t, f.root, name, output)
	}
	if _, err := os.Stat(filepath.Join(f.root, "bundle")); !os.IsNotExist(err) {
		t.Fatalf("a refused output was created: %v", err)
	}
}

// refuse asserts that exporting from root into output is refused before any
// other check runs (RCB-V0-005).
func refuse(t *testing.T, root, name, output string) {
	t.Helper()
	_, err := Export(context.Background(), root, Options{MapPath: mapPath, ExpectedBase: "HEAD", Target: "HEAD", Output: output})
	if cemcode.CodeOf(err) != CodeOutputRefused {
		t.Fatalf("%s: %v, want %s", name, err, CodeOutputRefused)
	}
}

// TestExportRefusesACaseVariantOfTheWorktree: the output check compares file
// identity, so a case variant of the worktree is refused where the volume
// folds case.
func TestExportRefusesACaseVariantOfTheWorktree(t *testing.T) {
	f := newFixture(t)
	variant := strings.ToUpper(f.root)
	original, _ := os.Stat(f.root)
	folded, err := os.Stat(variant)
	if variant == f.root || err != nil || !os.SameFile(original, folded) {
		t.Skip("the temporary volume is case-sensitive")
	}
	refuse(t, f.root, "case variant", filepath.Join(variant, "bundle"))
}

// TestExportRefusesEveryWorktreeAndGitDirectory: a Git directory outside the
// worktree, the primary worktree seen from a linked one, and a sibling linked
// worktree are all protected.
func TestExportRefusesEveryWorktreeAndGitDirectory(t *testing.T) {
	f := newFixture(t)
	bare := filepath.Join(t.TempDir(), "bare.git")
	fixtureGit(t, f.root, "clone", "-q", "--bare", f.root, bare)
	linked := filepath.Join(t.TempDir(), "linked")
	fixtureGit(t, bare, "worktree", "add", "-q", linked, "main")
	refuse(t, linked, "common directory outside the worktree", filepath.Join(bare, "bundle"))
	refuse(t, linked, "git directory outside the worktree", filepath.Join(bare, "worktrees", "linked", "bundle"))

	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	fixtureGit(t, f.root, "worktree", "add", "-q", "-b", "first", first)
	fixtureGit(t, f.root, "worktree", "add", "-q", "-b", "second", second)
	refuse(t, first, "primary worktree", filepath.Join(f.root, "bundle"))
	refuse(t, first, "sibling worktree", filepath.Join(second, "bundle"))
	refuse(t, f.root, "linked worktree", filepath.Join(first, "bundle"))
}

func TestExportRequiresAValidMap(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.root, mapPath)
	for name, content := range map[string]string{"missing": "", "invalid": `{"spec":"cem/0.2"}`} {
		os.Remove(path)
		if content != "" {
			writeFixture(t, path, content)
		}
		output := filepath.Join(t.TempDir(), "bundle")
		_, err := Export(context.Background(), f.root, Options{MapPath: mapPath, ExpectedBase: f.base, Target: f.target, Output: output})
		if err == nil {
			t.Fatalf("%s map exported", name)
		}
		if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
			t.Fatalf("%s map left a bundle: %v", name, statErr)
		}
	}
	change := fixtureGit(t, f.root, "rev-parse", f.target+"^")
	edited := strings.Replace(f.cem, "{", "{ ", 1)
	cases := []struct {
		name, content, base, target, mapPath, want string
	}{
		{"symlinked", f.cem, f.base, f.target, "link.cem.json", cemcode.MapUnavailable},
		{"other base", f.cem, change, f.target, mapPath, cemcode.BaseRevisionMismatch},
		{"uncommitted", f.cem, f.base, change, mapPath, CodeMapUncommitted},
		{"differs from the committed map", edited, f.base, f.target, mapPath, cemcode.ExcludedArtifactMismatch},
	}
	if err := os.Symlink(filepath.FromSlash(mapPath), filepath.Join(f.root, "link.cem.json")); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		os.Remove(path)
		if c.content != "" {
			writeFixture(t, path, c.content)
		}
		_, err := Export(context.Background(), f.root, Options{
			MapPath: c.mapPath, ExpectedBase: c.base, Target: c.target, Output: filepath.Join(t.TempDir(), "b"),
		})
		if cemcode.CodeOf(err) != c.want {
			t.Fatalf("%s map: %v, want %s", c.name, err, c.want)
		}
	}
}

// TestWriteFailureRemovesTheBundle: a failed write removes the directory it
// created and nothing else (RCB-V0-005).
func TestWriteFailureRemovesTheBundle(t *testing.T) {
	dir := t.TempDir()
	parent, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	twin := &present{File: "receipts/cem.json", data: []byte("x")}
	if err := write(parent, "bundle", []any{twin, twin}, []byte("m")); cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("write = %v, want %s", err, cemcode.PublishFailed)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("a failed write left %v", entries)
	}
}
