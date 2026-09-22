package archivebuild

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"testing"
)

func fixtureFiles() []File {
	return []File{
		{Name: "root/z", Mode: 0o644, Data: []byte("legal")},
		{Name: "root/a", Mode: 0o755, Data: []byte("binary")},
	}
}

func TestTarGzipCanonicalRawMetadata(t *testing.T) {
	raw, err := TarGzip(fixtureFiles())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw[:10], []byte{0x1f, 0x8b, 0x08, 0x00, 0, 0, 0, 0, 0x02, 0xff}) {
		t.Fatalf("gzip header %x", raw[:10])
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(gz)
	header, err := reader.Next()
	if err != nil {
		t.Fatal(err)
	}
	if header.Name != "root/a" || header.Mode != 0o755 || header.Uid != 0 || header.Gid != 0 || header.Format != tar.FormatUSTAR {
		t.Fatalf("header %+v", header)
	}
	if _, err := io.ReadAll(reader); err != nil {
		t.Fatal(err)
	}
}

func TestZIPCanonicalRawMetadata(t *testing.T) {
	raw, err := ZIP(fixtureFiles())
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 || reader.File[0].Name != "root/a" {
		t.Fatalf("files %+v", reader.File)
	}
	file := reader.File[0]
	if file.Flags != 0 || file.Method != zip.Store || len(file.Extra) != 0 || file.ModifiedDate != 0x21 || file.ExternalAttrs != uint32(0o100755)<<16 {
		t.Fatalf("header %+v", file.FileHeader)
	}
}

func TestAssemblerRejectsUnsafeInputs(t *testing.T) {
	for _, files := range [][]File{
		{{Name: "../x", Mode: 0o644}},
		{{Name: "a", Mode: 0o600}},
		{{Name: "Root/a", Mode: 0o644}, {Name: "root/A", Mode: 0o644}},
		{{Name: "a", Mode: 0o644}, {Name: "a", Mode: 0o644}},
	} {
		if _, err := TarGzip(files); err == nil {
			t.Fatalf("unsafe fixture accepted: %+v", files)
		}
		if _, err := ZIP(files); err == nil {
			t.Fatalf("unsafe fixture accepted by zip: %+v", files)
		}
	}
}

func TestRepeatedAssemblyIsByteIdentical(t *testing.T) {
	for _, assemble := range []func([]File) ([]byte, error){TarGzip, ZIP} {
		first, err := assemble(fixtureFiles())
		if err != nil {
			t.Fatal(err)
		}
		second, err := assemble(fixtureFiles())
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("assembly differs: %v", err)
		}
	}
}
