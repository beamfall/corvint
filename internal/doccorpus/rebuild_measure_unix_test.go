//go:build linux || darwin

package doccorpus

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestDCXV1534RebuildReuseMeasured measures docs corpus build --artifact on a large inventory,
// each mode in its own process so its peak resident set is its own: "full" rebuilds an artifact
// of another derivation (the cost every rebuild paid before DCX-V1-532), "reuse" one of another
// revision of the running derivation. Both must print the bytes a full build prints. A Linux
// child's peak resident set starts at its parent's, so a third process prepares the corpus and
// this one stays small. Set CORVINT_REBUILD_MEASURE_FILES (the PR measurement uses 9500) to run
// it (DCX-V1-534).
func TestDCXV1534RebuildReuseMeasured(t *testing.T) {
	if mode := os.Getenv("CORVINT_REBUILD_MEASURE_CHILD"); mode == "prepare" {
		prepareRebuildMeasure(t)
		return
	} else if mode != "" {
		measureRebuild(t, mode)
		return
	}
	if files, _ := strconv.Atoi(os.Getenv("CORVINT_REBUILD_MEASURE_FILES")); files <= 0 {
		t.Skip("set CORVINT_REBUILD_MEASURE_FILES to measure")
	}
	dir := t.TempDir()
	for _, mode := range []string{"prepare", "full", "reuse"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDCXV1534RebuildReuseMeasured$", "-test.count=1", "-test.v")
		cmd.Env = append(os.Environ(), "CORVINT_REBUILD_MEASURE_CHILD="+mode, "CORVINT_REBUILD_MEASURE_DIR="+dir)
		began := time.Now()
		out, err := cmd.CombinedOutput()
		elapsed := time.Since(began)
		if err != nil {
			t.Fatalf("%s: %v\n%s", mode, err, out)
		}
		if mode == "prepare" {
			t.Logf("%s", bytes.TrimSpace(out))
			continue
		}
		peak := cmd.ProcessState.SysUsage().(*syscall.Rusage).Maxrss
		if runtime.GOOS == "linux" {
			peak *= 1024
		}
		t.Logf("%s rebuild (%s/%s): wall %s, peak RSS %.0f MiB", mode, runtime.GOOS, runtime.GOARCH, elapsed.Round(time.Millisecond), float64(peak)/(1<<20))
	}
}

// prepareRebuildMeasure builds the measured corpus and leaves its repository, the expected
// digest and both artifacts in the measurement directory.
func prepareRebuildMeasure(t *testing.T) {
	dir := os.Getenv("CORVINT_REBUILD_MEASURE_DIR")
	files, _ := strconv.Atoi(os.Getenv("CORVINT_REBUILD_MEASURE_FILES"))
	root, m := largeAdoptionCorpus(t, files, 8)
	a, err := BuildAdmitted(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	current := encodeArtifact(t, a)
	for name, data := range map[string][]byte{"want.sha256": []byte(Digest(current)), "full.json": rehashed(t, a, staleDerivation), "reuse.json": rehashed(t, a, otherRevision)} {
		writeFixture(t, dir, name, data)
	}
	if err := os.CopyFS(filepath.Join(dir, "repo"), os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	t.Logf("corpus: %d files, %d subjects, %d claims, artifact %d bytes", files, len(a.Subjects), len(a.Claims), len(current))
}

// measureRebuild is one measured rebuild: the CLI's read, embedded-manifest decode and rebuild.
func measureRebuild(t *testing.T, mode string) {
	dir := os.Getenv("CORVINT_REBUILD_MEASURE_DIR")
	data, err := os.ReadFile(filepath.Join(dir, mode+".json"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(dir, "want.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ManifestOfArtifact(data)
	if err != nil {
		t.Fatal(err)
	}
	a, reused, err := RebuildArtifact(context.Background(), filepath.Join(dir, "repo"), m, data)
	if err != nil {
		t.Fatal(err)
	}
	if reused != (mode == "reuse") {
		t.Fatalf("%s: reused=%v", mode, reused)
	}
	out, err := Encode(a)
	if err != nil || !bytes.Equal([]byte(Digest(out)), want) {
		t.Fatalf("%s: rebuilt bytes differ from a full build (%v)", mode, err)
	}
}
