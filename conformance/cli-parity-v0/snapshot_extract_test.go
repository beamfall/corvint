package main

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// extractFixture is one owned temporary tree with an extraction root and a
// sibling the archive must never reach.
type extractFixture struct {
	root, outside string
}

func newExtractFixture(t *testing.T) extractFixture {
	t.Helper()
	base := t.TempDir()
	probe := filepath.Join(base, "probe")
	if err := os.Symlink("target", probe); err != nil {
		t.Skipf("symbolic links are unavailable here: %v", err)
	}
	fixture := extractFixture{root: filepath.Join(base, "root"), outside: filepath.Join(base, "outside")}
	for _, dir := range []string{fixture.root, fixture.outside} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func (f extractFixture) assertOutsideUntouched(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(f.outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("extraction wrote outside its root: %v", entries)
	}
}

func tarOf(t *testing.T, headers []tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	for _, header := range headers {
		header := header
		body := []byte("proof")
		if header.Typeflag == tar.TypeReg {
			header.Size = int64(len(body))
		}
		if err := writer.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := writer.Write(body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Positive control: contained files, directories, links and a write through an
// internal directory link all extract.
func TestV13179ExtractSafeTarKeepsContainedTree(t *testing.T) {
	f := newExtractFixture(t)
	archive := tarOf(t, []tar.Header{
		{Name: "x/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "d/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "d/a", Typeflag: tar.TypeSymlink, Linkname: "../x"},
		{Name: "d/a/inner", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "file-link", Typeflag: tar.TypeSymlink, Linkname: "d/a/inner"},
		{Name: "dangling", Typeflag: tar.TypeSymlink, Linkname: "later/missing"},
		{Name: "top", Typeflag: tar.TypeReg, Mode: 0o644},
	})
	if err := extractSafeTar(archive, f.root); err != nil {
		t.Fatalf("contained archive refused: %v", err)
	}
	for _, path := range []string{"x/inner", "file-link", "top"} {
		got, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(path)))
		if err != nil || string(got) != "proof" {
			t.Fatalf("%s = %q, %v", path, got, err)
		}
	}
	f.assertOutsideUntouched(t)
}

// The confirmed V1-3179 escape: two individually admitted links (d/a -> ../x and
// b -> d/a/../../outside) used to carry b/payload outside the extraction root.
func TestV13179ExtractSafeTarRefusesLinkChainEscape(t *testing.T) {
	f := newExtractFixture(t)
	headers := []tar.Header{
		{Name: "x", Typeflag: tar.TypeDir, Mode: 0o700},
		{Name: "d", Typeflag: tar.TypeDir, Mode: 0o700},
		{Name: "d/a", Typeflag: tar.TypeSymlink, Linkname: "../x"},
		{Name: "b", Typeflag: tar.TypeSymlink, Linkname: "d/a/../../outside"},
		{Name: "b/payload", Typeflag: tar.TypeReg, Mode: 0o600},
	}
	for _, header := range headers {
		if header.Typeflag == tar.TypeSymlink {
			if err := validateSymlinkTarget(header.Name, header.Linkname); err != nil {
				t.Fatalf("the lexical validator must still admit %s, or this test no longer reaches the escape: %v", header.Name, err)
			}
		}
	}
	if err := extractSafeTar(tarOf(t, headers), f.root); err == nil {
		t.Fatal("link-chain escape was extracted")
	}
	f.assertOutsideUntouched(t)
	if _, err := os.Lstat(filepath.Join(f.root, "b")); !os.IsNotExist(err) {
		t.Fatalf("escaping link b was created: %v", err)
	}
}

// A link created before the link that makes it escape is caught after the last
// member and removed; a write through it is refused when it is reached.
func TestV13179ExtractSafeTarRefusesLateEscape(t *testing.T) {
	for name, headers := range map[string][]tar.Header{
		"link-only": {
			{Name: "b", Typeflag: tar.TypeSymlink, Linkname: "d/a/../../outside"},
			{Name: "x", Typeflag: tar.TypeDir, Mode: 0o700},
			{Name: "d", Typeflag: tar.TypeDir, Mode: 0o700},
			{Name: "d/a", Typeflag: tar.TypeSymlink, Linkname: "../x"},
		},
		"write-through": {
			{Name: "b", Typeflag: tar.TypeSymlink, Linkname: "d/a/../../outside"},
			{Name: "x", Typeflag: tar.TypeDir, Mode: 0o700},
			{Name: "d", Typeflag: tar.TypeDir, Mode: 0o700},
			{Name: "d/a", Typeflag: tar.TypeSymlink, Linkname: "../x"},
			{Name: "b/payload", Typeflag: tar.TypeReg, Mode: 0o600},
		},
		"directory-through": {
			{Name: "b", Typeflag: tar.TypeSymlink, Linkname: "d/a/../../outside"},
			{Name: "x", Typeflag: tar.TypeDir, Mode: 0o700},
			{Name: "d", Typeflag: tar.TypeDir, Mode: 0o700},
			{Name: "d/a", Typeflag: tar.TypeSymlink, Linkname: "../x"},
			{Name: "b/sub/", Typeflag: tar.TypeDir, Mode: 0o700},
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newExtractFixture(t)
			if err := extractSafeTar(tarOf(t, headers), f.root); err == nil {
				t.Fatal("late link-chain escape was extracted")
			}
			f.assertOutsideUntouched(t)
			if name == "link-only" {
				if _, err := os.Lstat(filepath.Join(f.root, "b")); !os.IsNotExist(err) {
					t.Fatalf("escaping link b was kept: %v", err)
				}
			}
		})
	}
}

func TestV13179ExtractSafeTarRefusesUnsafeMembers(t *testing.T) {
	for name, headers := range map[string][]tar.Header{
		"dot-dot member":  {{Name: "../payload", Typeflag: tar.TypeReg, Mode: 0o600}},
		"absolute member": {{Name: "/payload", Typeflag: tar.TypeReg, Mode: 0o600}},
		"escaping link":   {{Name: "l", Typeflag: tar.TypeSymlink, Linkname: "../outside"}},
		"absolute link":   {{Name: "l", Typeflag: tar.TypeSymlink, Linkname: "/tmp"}},
		"link loop": {
			{Name: "p", Typeflag: tar.TypeSymlink, Linkname: "q"},
			{Name: "q", Typeflag: tar.TypeSymlink, Linkname: "p"},
			{Name: "p/payload", Typeflag: tar.TypeReg, Mode: 0o600},
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newExtractFixture(t)
			if err := extractSafeTar(tarOf(t, headers), f.root); err == nil {
				t.Fatal("unsafe member was extracted")
			}
			f.assertOutsideUntouched(t)
		})
	}
}
