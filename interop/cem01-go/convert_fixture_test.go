package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// convertTestdata holds the native converter's committed source and golden
// outputs (V1-1275); the portable consumer reads only the golden bytes.
const convertTestdata = "../../internal/cem/convert/testdata"

func readConvertFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(convertTestdata, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func stableCode(r stableResult) string {
	if r.Code == nil {
		return ""
	}
	return *r.Code
}

// TestCEMV1104PortableConvertedFixtures verifies the converter's golden
// outputs with the portable consumer: each is admitted by the stable reader,
// the 0.3 vector converts to ACCEPT at its manifest repository, the 0.1
// conformance map rejects patch-digest-mismatch at its reconstructed target,
// and with no pinned object the result is an explicit repository refusal.
// The reader check runs everywhere; the verification runs need the qualified
// process containment of the canonical engine and are NOT_RUN elsewhere.
func TestCEMV1104PortableConvertedFixtures(t *testing.T) {
	for _, name := range []string{"reachable-a79ab231", "cem01-valid", "cem03-coverage-covered"} {
		if _, e := decodeStable(readConvertFixture(t, name+".converted.json")); e != nil {
			t.Fatalf("%s: stable reader refused converter output: %v", name, e)
		}
	}
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}

	m, read := readPacket03(t)
	repos := buildPacket03Repositories(t, m, read)
	r, _ := runStableCanonical(context.Background(), repos["sha1"], readConvertFixture(t, "cem03-coverage-covered.converted.json"),
		"016af7ee8e162bc10d0f98dcd52b2319ac584c85", "cef3e87ac46d04009f761104ea01d263ac3b32b2", portableTempDir(t), nil)
	if r.Outcome != "ACCEPT" || r.Axes["changeIntegrity"] != "VERIFIED" || r.Sidecar != "ABSENT" || len(r.Hunks) != 1 || r.Hunks[0].Coverage == nil {
		t.Fatalf("0.3 converted = %s/%s/%s sidecar %s hunks %d", r.Outcome, r.Stage, stableCode(r), r.Sidecar, len(r.Hunks))
	}

	dir := portableTempDir(t)
	conformance := []string{"GIT_AUTHOR_NAME=CEM Conformance", "GIT_AUTHOR_EMAIL=cem@example.invalid", "GIT_COMMITTER_NAME=CEM Conformance",
		"GIT_COMMITTER_EMAIL=cem@example.invalid", "GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000"}
	packet03Git(t, dir, nil, nil, "init", "-q", "--object-format=sha1", "-b", "main")
	index := append([]string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}, conformance...)
	for _, p := range []string{"docs/rule.txt", "src/app.py"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "conformance", "cem-0.1", "base", filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		oid := packet03Git(t, dir, data, nil, "hash-object", "-w", "--no-filters", "--stdin")
		packet03Git(t, dir, nil, index, "update-index", "--add", "--cacheinfo", "100644,"+oid+","+p)
	}
	base := packet03Git(t, dir, nil, index, "commit-tree", packet03Git(t, dir, nil, index, "write-tree"), "-m", "CEM 0.1 base")
	patch, err := os.ReadFile(filepath.Join("..", "..", "conformance", "cem-0.1", "change.patch"))
	if err != nil {
		t.Fatal(err)
	}
	packet03Git(t, dir, patch, index, "apply", "--cached", "-")
	target := packet03Git(t, dir, nil, index, "commit-tree", packet03Git(t, dir, nil, index, "write-tree"), "-p", base, "-m", "CEM 0.1 target")
	if base != "4ca153370afd9bd8c6034ad73acc3925150ab681" || target != "0ea42c62cf2ed06e051ab583c49abbfedd3f5a86" {
		t.Fatalf("0.1 repository rebuilt as %s..%s", base, target)
	}
	r, _ = runStableCanonical(context.Background(), dir, readConvertFixture(t, "cem01-valid.converted.json"), base, target, portableTempDir(t), nil)
	if r.Outcome != "REJECT" || r.Stage != "verification" || len(r.IssueCodes) != 1 || r.IssueCodes[0] != "patch-digest-mismatch" {
		t.Fatalf("0.1 converted = %s/%s/%s %v", r.Outcome, r.Stage, stableCode(r), r.IssueCodes)
	}

	empty := portableTempDir(t)
	packet03Git(t, empty, nil, nil, "init", "-q", "--object-format=sha1", "-b", "main")
	r, _ = runStableCanonical(context.Background(), empty, readConvertFixture(t, "reachable-a79ab231.converted.json"),
		"d8ceb030f812f4d77908fe492ef2ef1b2baf2178", "a79ab231e86f5365d8a84732e90f4b2c0755ac4a", portableTempDir(t), nil)
	if r.Outcome != "UNSUPPORTED" || r.Stage != "repository" || stableCode(r) != "git-read-failed" {
		t.Fatalf("unavailable objects = %s/%s/%s", r.Outcome, r.Stage, stableCode(r))
	}
}

func portableTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
