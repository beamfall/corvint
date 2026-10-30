package corpusbinding

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func mustHistory(t *testing.T) *historyFixture {
	t.Helper()
	h, err := openHistoryFixture(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func requireError(t *testing.T, err error, fragment string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), fragment) {
		t.Fatalf("want %q refusal, got %v", fragment, err)
	}
}

// Initialize the exact fixture before injecting faults. Each assertion names
// the intended refusal, so an earlier fixture/setup failure cannot pass it.
func TestCEMV1201HistoricalFixtureControls(t *testing.T) {
	h := mustHistory(t)
	m, pack, err := readHistoryInputs()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.rootFixtures(committedChanges); err != nil {
		t.Fatal(err)
	}
	t.Run("bounded object output", func(t *testing.T) {
		_, err := fixtureGit(h.repo, nil, 1, "cat-file", "blob", m.Fixtures[0].OID)
		requireError(t, err, "Git output exceeds pinned/read-object bound")
	})
	t.Run("corrupt pack digest", func(t *testing.T) {
		bad := append([]byte(nil), pack...)
		bad[len(bad)/2] ^= 1
		_, err := importHistoryPack(t.TempDir(), m, bad)
		requireError(t, err, "pack digest/size mismatch")
	})
	t.Run("truncated pack", func(t *testing.T) {
		_, err := importHistoryPack(t.TempDir(), m, pack[:len(pack)-1])
		requireError(t, err, "pack digest/size mismatch")
	})
	t.Run("internal pack checksum", func(t *testing.T) {
		bad := append([]byte(nil), pack...)
		bad[len(bad)-1] ^= 1
		injected := m
		injected.PackSHA256 = digestBytes(bad)
		_, err := importHistoryPack(t.TempDir(), injected, bad)
		requireError(t, err, "historical pack import")
	})
	t.Run("missing expected object", func(t *testing.T) {
		omitted := m.Fixtures[0].OID
		var names []string
		limit := 32
		for _, object := range m.Objects {
			limit += object.Size + 64
			if object.OID != omitted {
				names = append(names, object.OID)
			}
		}
		bad, err := fixtureGit(h.repo, []byte(strings.Join(names, "\n")+"\n"), limit, "pack-objects", "--stdout", "--window=0", "--no-reuse-delta", "--no-reuse-object")
		if err != nil {
			t.Fatal(err)
		}
		injected := m
		injected.PackSHA256 = digestBytes(bad)
		injected.PackBytes = len(bad)
		_, err = importHistoryPack(t.TempDir(), injected, bad)
		requireError(t, err, "historical object inventory mismatch")
	})
	t.Run("wrong manifest object type", func(t *testing.T) {
		injected := m
		injected.Objects = append([]fixtureObject(nil), m.Objects...)
		injected.Objects[0].Type = "not-the-recorded-type"
		_, err := importHistoryPack(t.TempDir(), injected, pack)
		requireError(t, err, "historical object inventory mismatch")
	})
	for _, kind := range []string{"name", "hash", "blob"} {
		t.Run("wrong manifest fixture "+kind, func(t *testing.T) {
			injected := *h
			injected.manifest.Fixtures = append([]retainedFixture(nil), h.manifest.Fixtures...)
			want := "historical root fixture inventory mismatch"
			switch kind {
			case "name":
				injected.manifest.Fixtures[0].Name = strings.Repeat("f", 40) + ".cem.json"
			case "hash":
				injected.manifest.Fixtures[0].SHA256 = strings.Repeat("0", 64)
				want = "historical root bytes mismatch"
			case "blob":
				injected.manifest.Fixtures[0].OID = injected.manifest.Fixtures[1].OID
				want = "historical fixture blob mismatch"
			}
			_, err := injected.rootFixtures(committedChanges)
			requireError(t, err, want)
		})
	}
}

func copyCorpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(committedChanges)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".cem.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(committedChanges, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, entry.Name()), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCEMV1201HistoricalDomainAndCurrentProof(t *testing.T) {
	h := mustHistory(t)
	dir := copyCorpus(t)
	counts, err := verifyCorpus(committedChanges, dir, h)
	if err != nil || counts.historicalNative != 10 || counts.historicalMigrated != 1 {
		t.Fatalf("positive corpus initialization: %+v %v", counts, err)
	}
	name := h.manifest.Fixtures[0].Name
	path := filepath.Join(dir, name)
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(append([]byte(nil), good...), '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = verifyCorpus(committedChanges, dir, h)
	requireError(t, err, "historical fixture changed: "+name)
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	_, err = verifyCorpus(committedChanges, dir, h)
	requireError(t, err, "historical fixture missing "+name)
	if err = os.WriteFile(path, good, 0600); err != nil {
		t.Fatal(err)
	}
	// The bytes are available in the history fixture, but an unknown name is
	// never part of that domain and cannot borrow the fixture's binding.
	unknown := strings.Repeat("f", 40) + ".cem.json"
	if err = os.WriteFile(filepath.Join(dir, unknown), good, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = verifyCorpus(committedChanges, dir, h)
	requireError(t, err, "object "+strings.TrimSuffix(unknown, ".cem.json")+":.corvint/change.cem.json unavailable")
	if err = os.Remove(filepath.Join(dir, unknown)); err != nil {
		t.Fatal(err)
	}
	// Find an actual current seal, so wrong-current-bytes cannot be hidden
	// by a failure to initialize its original binding.
	retained, err := h.rootFixtures(committedChanges)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if _, old := retained[entry.Name()]; old || !strings.HasSuffix(entry.Name(), ".cem.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		migrated, err := verifyBinding(committedChanges, entry.Name(), data)
		if err != nil {
			t.Fatal(err)
		}
		if migrated {
			continue
		}
		_, err = verifyBinding(committedChanges, entry.Name(), append(data, '\n'))
		requireError(t, err, "differs from its bind commit's sidecar")
		t.Logf("current native positive and mismatched-byte control: %s", entry.Name())
		return
	}
	t.Fatal("current native binding control NOT_RUN: this candidate has no current seal")
}

func newBare(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	template := filepath.Join(parent, "empty")
	if err := os.Mkdir(template, 0700); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(parent, "product.git")
	if _, err := fixtureGit(parent, nil, 4096, "init", "--bare", "--object-format=sha1", "--template="+template, repo); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestCEMV1201FixtureGitIsolation(t *testing.T) {
	product := newBare(t)
	foreign, err := fixtureGit(product, []byte("unrelated host object"), 128, "hash-object", "-w", "--stdin")
	if err != nil {
		t.Fatal(err)
	}
	before, err := fixtureGit(product, nil, 8192, "cat-file", "--batch-all-objects", "--batch-check")
	if err != nil {
		t.Fatal(err)
	}
	invalidConfig := filepath.Join(t.TempDir(), "bad-config")
	if err = os.WriteFile(invalidConfig, []byte("[invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"GIT_DIR": product, "GIT_WORK_TREE": product, "GIT_COMMON_DIR": product,
		"GIT_OBJECT_DIRECTORY": filepath.Join(product, "objects"), "GIT_ALTERNATE_OBJECT_DIRECTORIES": filepath.Join(product, "objects"),
		"GIT_CONFIG_GLOBAL": invalidConfig, "GIT_CONFIG_SYSTEM": invalidConfig,
		"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "core.repositoryformatversion", "GIT_CONFIG_VALUE_0": "999",
		"GIT_CONFIG_PARAMETERS": "'core.repositoryformatversion'='999'",
	} {
		t.Setenv(key, value)
	}
	h := mustHistory(t)
	if _, err = readGitBlob(h.repo, strings.TrimSpace(string(foreign))); err == nil {
		t.Fatal("fixture borrowed host object")
	}
	name := h.manifest.Fixtures[0].Name
	data, err := readGitBlob(h.repo, h.manifest.Fixtures[0].OID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = verifyBinding(h.repo, name, data); err != nil {
		t.Fatalf("positive historical binding: %v", err)
	}
	_, err = verifyBinding(product, name, data)
	requireError(t, err, "object "+strings.TrimSuffix(name, ".cem.json")+":.corvint/change.cem.json unavailable")
	after, err := fixtureGit(product, nil, 8192, "cat-file", "--batch-all-objects", "--batch-check")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("fixture import changed product object inventory")
	}
}

// Run the real reportBindingError/notRun branch in a child test binary: changing
// V1-1334 to skip under CI would make the child succeed and this assertion fail.
func TestCEMV1201MissingCurrentBindingFailsCI(t *testing.T) {
	h := mustHistory(t)
	dir := copyCorpus(t)
	if _, err := verifyCorpus(committedChanges, dir, h); err != nil {
		t.Fatalf("positive corpus: %v", err)
	}
	data, err := readGitBlob(h.repo, h.manifest.Fixtures[0].OID)
	if err != nil {
		t.Fatal(err)
	}
	unknown := strings.Repeat("f", 40)
	if err = os.WriteFile(filepath.Join(dir, unknown+".cem.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCEMV1201MissingCurrentBindingHelper$")
	cmd.Env = append(os.Environ(), "CI=true", "CORVINT_CORPUS_HELPER="+dir)
	out, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("CI=true: object "+unknown+":.corvint/change.cem.json unavailable")) {
		t.Fatalf("missing current proof did not fail CI at lookup: %v\n%s", err, out)
	}
	t.Logf("observed strict CI failure at current lookup %s", unknown)
}

func TestCEMV1201MissingCurrentBindingHelper(t *testing.T) {
	dir := os.Getenv("CORVINT_CORPUS_HELPER")
	if dir == "" {
		return
	}
	h := mustHistory(t)
	_, err := verifyCorpus(committedChanges, dir, h)
	if err == nil {
		t.Fatal("injected missing current proof unexpectedly passed")
	}
	reportBindingError(t, err)
}
