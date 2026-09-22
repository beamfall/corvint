package jstestprovider

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDigestAppBuildDir_Unknown(t *testing.T) {
	id, err := DigestAppBuildDir("")
	if err != nil {
		t.Fatal(err)
	}
	if !id.Unknown || id.Reason == "" {
		t.Fatalf("want explicit unknown with a reason, got %+v", id)
	}
}

func TestDigestAppBuildDir_MissingDirIsUnknown(t *testing.T) {
	id, err := DigestAppBuildDir(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatal(err)
	}
	if !id.Unknown {
		t.Fatalf("want unknown for a missing directory, got %+v", id)
	}
}

// TestDigestAppBuildDir_ChangeDetected is the stale-app-build detection unit
// test: the same directory digests differently once a served file's content
// changes, which is exactly the signal RunE2E uses to set Receipt.StaleAppBuild.
func TestDigestAppBuildDir_ChangeDetected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<html>v1</html>")
	before, err := DigestAppBuildDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if before.Unknown {
		t.Fatalf("want a real digest, got unknown: %+v", before)
	}
	writeFile(t, filepath.Join(dir, "index.html"), "<html>v2</html>")
	after, err := DigestAppBuildDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if before.Digest == after.Digest {
		t.Fatalf("want digest to change when served content changes, got the same %q both times", before.Digest)
	}
}

// TestDigestAppBuildDir_SymlinkedDirectoryIsUnknown: a symlink to a directory
// is not a regular file the digest can bind, so the identity abstains as
// unknown instead of returning an error that discards the E2E receipt.
func TestDigestAppBuildDir_SymlinkedDirectoryIsUnknown(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<html>v1</html>")
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, "assets")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	id, err := DigestAppBuildDir(dir)
	if err != nil {
		t.Fatalf("want an explicit unknown identity, got error %v", err)
	}
	if !id.Unknown || id.Reason == "" || id.Digest != "" {
		t.Fatalf("want unknown with a reason and no digest, got %+v", id)
	}
}

func TestDigestCombined_ChangesWithEitherInput(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "package.json")
	lock := filepath.Join(dir, "package-lock.json")
	writeFile(t, pkg, `{"name":"a"}`)
	writeFile(t, lock, `{"lock":1}`)
	base, err := digestCombined(pkg, lock)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, lock, `{"lock":2}`)
	changed, err := digestCombined(pkg, lock)
	if err != nil {
		t.Fatal(err)
	}
	if base == changed {
		t.Fatalf("want digest to change when the lockfile changes")
	}
}
