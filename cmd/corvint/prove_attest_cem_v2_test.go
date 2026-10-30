package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/attest"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// cemV2Case is the public receipt-authentication case: a cem/1.0 stable map
// over a two-commit recipe repository with three declared artifacts.
const cemV2Case = "../../protocol/cem-1.0/stable/receipt-authentication"

type cemV2Fixture struct {
	repo, artifacts, base, target string
}

// materializeCEMV2Case builds the case's recipe repository under a
// symlink-free directory, checks the target out, writes the map into the
// worktree as stable.json (untracked) and copies the case's artifacts beside
// it, so the stable verifier accepts the map with full OIDs.
func materializeCEMV2Case(t *testing.T) cemV2Fixture {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("NOT_RUN: the stable verifier's owned Git lifecycle is observed on Darwin and Linux only")
	}
	var recipe struct {
		Commits []struct {
			Name    string            `json:"name"`
			Message string            `json:"message"`
			Files   map[string]string `json:"files"`
		} `json:"commits"`
	}
	data, err := os.ReadFile(filepath.Join(cemV2Case, "repository.recipe.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &recipe); err != nil {
		t.Fatal(err)
	}
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := cemV2Fixture{repo: filepath.Join(directory, "repo"), artifacts: filepath.Join(directory, "case")}
	if err := os.Mkdir(f.repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(stdin string, args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		command.Dir = f.repo
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@invalid",
			"GIT_AUTHOR_DATE=2026-10-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T00:00:00Z")
		command.Stdin = strings.NewReader(stdin)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("", "init", "-q", "--object-format=sha1")
	for _, commit := range recipe.Commits {
		names := make([]string, 0, len(commit.Files))
		for name := range commit.Files {
			names = append(names, name)
		}
		sort.Strings(names)
		var tree strings.Builder
		for _, name := range names {
			tree.WriteString("100644 blob " + git(commit.Files[name], "hash-object", "-w", "--stdin") + "\t" + name + "\n")
		}
		args := []string{"commit-tree", git(tree.String(), "mktree"), "-m", commit.Message}
		if f.target != "" {
			args = append(args, "-p", f.target)
		}
		f.target = git("", args...)
		if commit.Name == "base" {
			f.base = f.target
		}
	}
	git("", "update-ref", "refs/heads/main", f.target)
	git("", "symbolic-ref", "HEAD", "refs/heads/main")
	git("", "reset", "-q", "--hard", f.target)
	if err := os.MkdirAll(filepath.Join(f.artifacts, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(cemV2Case, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(cemV2Case, "artifacts", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.artifacts, "artifacts", entry.Name()), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mapBytes, err := os.ReadFile(filepath.Join(cemV2Case, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.repo, "stable.json"), mapBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

// emitCEMV2Attestation runs `prove --attest-cem-v2` on the case and returns
// both stdout lines plus the CEM envelope and public key saved outside the
// repository.
func emitCEMV2Attestation(t *testing.T, f cemV2Fixture) (lines []string, envelopePath, publicKeyPath string) {
	t.Helper()
	keyPath, publicKey := proveTestKey(t)
	stdout, stderr, code := runProveArguments(t, "--root", f.repo, "prove", "--cem", "stable.json",
		"--expected-base", f.base, "--target", f.target, "--artifacts", f.artifacts,
		"--attest-key", keyPath, "--attest-cem-v2")
	lines = strings.Split(strings.TrimSuffix(string(stdout), "\n"), "\n")
	if code != 0 || len(lines) != 2 {
		t.Fatalf("emit exit %d, %d lines: %s", code, len(lines), stderr)
	}
	directory := t.TempDir()
	envelopePath = filepath.Join(directory, "cem.dsse.json")
	if err := os.WriteFile(envelopePath, []byte(lines[1]), 0o600); err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyPath = filepath.Join(directory, "signer.pub")
	if err := os.WriteFile(publicKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return lines, envelopePath, publicKeyPath
}

// TestCEMV1065ProveEmitsAndVerifiesTheV2Statement checks CEM-V1-065: the
// second line is a DSSE envelope over the cem/v2 statement whose subjects are
// the map then the declared artifacts in map order and whose predicate names
// the base, the patch and the accepted target commit and tree; the
// verification mode reports the map, the artifacts and the target VERIFIED
// without mutating the repository.
func TestCEMV1065ProveEmitsAndVerifiesTheV2Statement(t *testing.T) {
	t.Parallel()
	f := materializeCEMV2Case(t)
	lines, envelopePath, publicKeyPath := emitCEMV2Attestation(t, f)
	var envelope struct{ Payload string }
	if err := json.Unmarshal([]byte(lines[1]), &envelope); err != nil {
		t.Fatal(err)
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var statement struct {
		PredicateType string
		Subject       []struct {
			Name   string
			Digest map[string]string
		}
		Predicate struct {
			BaseRevision string
			Target       struct{ GitCommit, GitTree string }
		}
	}
	if err := json.Unmarshal(payload, &statement); err != nil {
		t.Fatal(err)
	}
	mapBytes, err := os.ReadFile(filepath.Join(f.repo, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseStable(mapBytes)
	if err != nil {
		t.Fatal(err)
	}
	mapSum := sha256.Sum256(mapBytes)
	want := []string{"stable.json=" + hex.EncodeToString(mapSum[:])}
	for _, artifact := range document.Artifacts {
		want = append(want, artifact.Path+"="+artifact.Sha256)
	}
	var got []string
	for _, subject := range statement.Subject {
		got = append(got, subject.Name+"="+subject.Digest["sha256"])
	}
	if statement.PredicateType != attest.CEMPredicateTypeV2 || strings.Join(got, ",") != strings.Join(want, ",") ||
		statement.Predicate.BaseRevision != f.base || statement.Predicate.Target.GitCommit != f.target || len(statement.Predicate.Target.GitTree) != 40 {
		t.Fatalf("statement %+v, want subjects %v", statement, want)
	}
	before := treeDigest(t, f.repo)
	receipt, _, stderr, code := runProveCLI(t, f.repo, "--verify-cem-attestation", envelopePath,
		"--attest-public-key", publicKeyPath, "--cem", "stable.json", "--artifacts", f.artifacts, "--target", f.target)
	if code != 0 {
		t.Fatalf("verify exit %d: %s", code, stderr)
	}
	if receipt["status"] != attest.CEMVerified || receipt["artifactsStatus"] != attest.CEMVerified ||
		receipt["targetStatus"] != attest.CEMVerified || receipt["predicateType"] != attest.CEMPredicateTypeV2 {
		t.Fatalf("verification receipt: %v", receipt)
	}
	if treeDigest(t, f.repo) != before {
		t.Fatal("verification changed the repository")
	}
}

// TestCEMV1065RefusesAReceiptSwapUnderTheOldEnvelope checks CEM-V1-065's
// receipt-swap case: a map re-issued over a different receipt attestation does
// not verify under the old envelope (attest-cem-mismatch), and the old map
// with the swapped artifact file is refused because that file is not a signed
// subject (attest-cem-subject-mismatch).
func TestCEMV1065RefusesAReceiptSwapUnderTheOldEnvelope(t *testing.T) {
	t.Parallel()
	f := materializeCEMV2Case(t)
	_, envelopePath, publicKeyPath := emitCEMV2Attestation(t, f)
	original, err := os.ReadFile(filepath.Join(f.artifacts, "artifacts", "receipt-attestation.json"))
	if err != nil {
		t.Fatal(err)
	}
	swapped, err := os.ReadFile(filepath.Join(f.artifacts, "artifacts", "attestation-other-key.json"))
	if err != nil {
		t.Fatal(err)
	}
	oldSum, newSum := sha256.Sum256(original), sha256.Sum256(swapped)
	mapBytes, err := os.ReadFile(filepath.Join(f.repo, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	reissued := bytes.ReplaceAll(mapBytes, []byte(hex.EncodeToString(oldSum[:])), []byte(hex.EncodeToString(newSum[:])))
	if bytes.Equal(reissued, mapBytes) {
		t.Fatal("the fixture map does not name the receipt attestation digest")
	}
	if err := os.WriteFile(filepath.Join(f.repo, "reissued.json"), reissued, 0o644); err != nil {
		t.Fatal(err)
	}
	swappedRoot := filepath.Join(filepath.Dir(f.artifacts), "swapped")
	if err := os.MkdirAll(filepath.Join(swappedRoot, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"runner-plan.json", "runner-receipt.json", "receipt-attestation.json"} {
		content, err := os.ReadFile(filepath.Join(f.artifacts, "artifacts", name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "receipt-attestation.json" {
			content = swapped
		}
		if err := os.WriteFile(filepath.Join(swappedRoot, "artifacts", name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	common := []string{"--verify-cem-attestation", envelopePath, "--attest-public-key", publicKeyPath, "--target", f.target}
	verifyCEMAttestationRefusal(t, f.repo, "attest-cem-mismatch", append(append([]string{}, common...), "--cem", "reissued.json", "--artifacts", swappedRoot)...)
	verifyCEMAttestationRefusal(t, f.repo, "attest-cem-subject-mismatch", append(append([]string{}, common...), "--cem", "stable.json", "--artifacts", swappedRoot)...)
}

// TestCEMV1065RefusesATargetThatIsNotTheSubject checks CEM-V1-065: a --target
// naming any commit other than the signed target exits 2 with
// attest-cem-target-mismatch and prints nothing.
func TestCEMV1065RefusesATargetThatIsNotTheSubject(t *testing.T) {
	t.Parallel()
	f := materializeCEMV2Case(t)
	_, envelopePath, publicKeyPath := emitCEMV2Attestation(t, f)
	verifyCEMAttestationRefusal(t, f.repo, "attest-cem-target-mismatch", "--verify-cem-attestation", envelopePath,
		"--attest-public-key", publicKeyPath, "--cem", "stable.json", "--target", f.base)
}

// TestCEMV1065RefusesALegacyMap checks CEM-V1-065: --attest-cem-v2 over a
// cem/0.x map (the argument fixture relabelled, CEM-V1-214) exits 2 with
// attest-cem-v2-requires-stable-map, names the retired-spec recovery line
// (CEM-V1-210) and prints nothing; v1 is not widened to stable maps.
func TestCEMV1065RefusesALegacyMap(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	mapPath := filepath.Join(root, ".corvint", "change.cem.json")
	mapBytes, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mapPath, bytes.Replace(mapBytes, []byte(`"spec":"`+wire.StableSpec+`"`), []byte(`"spec":"cem/0.2"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	keyPath, _ := proveTestKey(t)
	arguments := []string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD", "--attest-key", keyPath, "--attest-cem-v2"}
	verifyCEMAttestationRefusal(t, root, "attest-cem-v2-requires-stable-map", arguments...)
	if _, _, stderr, _ := runProveCLI(t, root, arguments...); !strings.Contains(stderr, wire.RetiredSpecFix) {
		t.Fatalf("refusal names no recovery line: %s", stderr)
	}
}

// TestCEMV1065RefusesConflictingFlags checks CEM-V1-065's grammar:
// --attest-cem-v2 with a value, repeated, or combined with the retired
// --attest-cem or --attest-cem-v1 (unrecognized, CEM-V1-218) in either order,
// and --artifacts without it, exit 2 with invalid-arguments and print nothing.
func TestCEMV1065RefusesConflictingFlags(t *testing.T) {
	t.Parallel()
	root, base := proveCEMRepository(t)
	common := []string{"--cem", ".corvint/change.cem.json", "--expected-base", base, "--target", "HEAD", "--attest"}
	with := func(extra ...string) []string { return append(append([]string{}, common...), extra...) }
	for _, arguments := range [][]string{
		with("--attest-cem-v2=yes"),
		with("--attest-cem-v2", "--attest-cem-v2"),
		with("--attest-cem", "--attest-cem-v2"),
		with("--attest-cem-v2", "--attest-cem"),
		with("--attest-cem-v1", "--attest-cem-v2"),
		with("--attest-cem-v2", "--attest-cem-v1"),
		with("--artifacts", root),
		with("--attest-cem", "--artifacts", root),
		{"--attest-cem-v2", "docs/rule.txt"},
	} {
		verifyCEMAttestationRefusal(t, root, "invalid-arguments", arguments...)
	}
}
