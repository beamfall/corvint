package releasecandidate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPUBV0025VersionedInstallCoexistsAndNeverReplaces(t *testing.T) {
	t.Run("PUB-V0-025 versioned install coexists and never replaces", testPUBV0025VersionedInstallCoexistsAndNeverReplaces)
}

func testPUBV0025VersionedInstallCoexistsAndNeverReplaces(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("qualified installer supports darwin and linux")
	}
	candidate := t.TempDir()
	archivePath := "core/corvint_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	previous := verifyForInstall
	verifyForInstall = func(context.Context, string) (*VerifiedCandidate, error) {
		return &VerifiedCandidate{Manifest: Manifest{Version: "0.5.0a1", CorvintVersion: "Corvint 0.5.0a1 (build 9)"}, files: map[string][]byte{archivePath: coreArchiveFixture(t, runtime.GOOS+"_"+runtime.GOARCH)}}, nil
	}
	t.Cleanup(func() { verifyForInstall = previous })
	store := filepath.Join(t.TempDir(), "store")
	installed, err := InstallCore(t.Context(), candidate, store)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(installed, filepath.Join("0.5.0a1", runtime.GOOS+"-"+runtime.GOARCH)) {
		t.Fatalf("unexpected installed path %s", installed)
	}
	if _, err := InstallCore(t.Context(), candidate, store); err == nil {
		t.Fatal("second install silently replaced the versioned path")
	}
	if raw, err := os.ReadFile(filepath.Join(installed, "corvint")); err != nil || len(raw) == 0 {
		t.Fatalf("first installed path was not preserved: %v", err)
	}
}

func TestPUBV0026CandidateVerifierRejectsChecksumDrift(t *testing.T) {
	files := map[string][]byte{"README.md": []byte("fixture\n")}
	files["SHA256SUMS"] = renderChecksums(files)
	if err := validateCandidateChecksums(files); err != nil {
		t.Fatal(err)
	}
	files["README.md"] = []byte("changed\n")
	if err := validateCandidateChecksums(files); err == nil {
		t.Fatal("candidate checksum drift accepted")
	}
}

func coreArchiveFixture(t *testing.T, platform string) []byte {
	t.Helper()
	var compressed bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter.Header.ModTime = time.Unix(0, 0)
	tarWriter := tar.NewWriter(gzipWriter)
	script := []byte("#!/bin/sh\nprintf '%s\\n' 'Corvint 0.5.0a1 (build 9)'\n")
	name := "corvint_" + platform + "/corvint"
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(script)), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(script); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}
