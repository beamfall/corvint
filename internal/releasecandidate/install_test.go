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
	platform := runtime.GOOS + "_" + runtime.GOARCH
	archivePath := "core/corvint_" + platform + ".tar.gz"
	previous := verifyForInstall
	verifyForInstall = func(context.Context, string) (*VerifiedCandidate, error) {
		files := map[string][]byte{archivePath: coreArchiveFixture(t, platform)}
		for _, app := range CoreSetApps {
			files[appArchivePath(app, platform)] = appScriptArchive(t, app, platform, "#!/bin/sh\nprintf '%s\\n' '"+app+" 0.5.0a1 (build 9)'\n")
		}
		return &VerifiedCandidate{Manifest: Manifest{Version: "0.5.0a1", CorvintVersion: "Corvint 0.5.0a1 (build 9)"}, files: files}, nil
	}
	t.Cleanup(func() { verifyForInstall = previous })
	store := filepath.Join(canonicalTemp(t), "store")
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
	return coreScriptArchive(t, platform, "#!/bin/sh\nprintf '%s\\n' 'Corvint 0.5.0a1 (build 9)'\n")
}

func coreScriptArchive(t *testing.T, platform, program string) []byte {
	return scriptArchive(t, "corvint", platform, program, false)
}

// appScriptArchive is a core-set app archive whose executable is a shell script, with the inner
// SHA256SUMS row extractAppBinary requires.
func appScriptArchive(t *testing.T, app, platform, program string) []byte {
	return scriptArchive(t, app, platform, program, true)
}

func scriptArchive(t *testing.T, binary, platform, program string, withChecksum bool) []byte {
	t.Helper()
	if withChecksum {
		return scriptArchiveWithMembers(t, binary, platform, program, nil)
	}
	return scriptArchiveWithMembers(t, binary, platform, program, map[string][]byte{"": nil})
}

// scriptArchiveWithMembers writes root/BINARY, root/SHA256SUMS and root/NAME for each extra
// member; the sentinel "" key omits the checksum row (the core fixture's shape).
func scriptArchiveWithMembers(t *testing.T, binary, platform, program string, extra map[string][]byte) []byte {
	t.Helper()
	_, withoutChecksum := extra[""]
	withChecksum := !withoutChecksum
	var compressed bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter.Header.ModTime = time.Unix(0, 0)
	tarWriter := tar.NewWriter(gzipWriter)
	script := []byte(program)
	name := binary + "_" + platform + "/" + binary
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(script)), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(script); err != nil {
		t.Fatal(err)
	}
	if withChecksum {
		sums := []byte(digest(script) + "  " + binary + "\n")
		if err := tarWriter.WriteHeader(&tar.Header{Name: binary + "_" + platform + "/SHA256SUMS", Mode: 0o644, Size: int64(len(sums)), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(sums); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range extra {
		if name == "" {
			continue
		}
		if err := tarWriter.WriteHeader(&tar.Header{Name: binary + "_" + platform + "/" + name, Mode: 0o644, Size: int64(len(content)), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}
