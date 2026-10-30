// Package archivebuild assembles the canonical Go release containers.
package archivebuild

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"hash/crc32"
	"path"
	"sort"
	"strings"
	"time"
)

// File is one regular archive member. Name is the full root-relative path.
type File struct {
	Name string
	Mode int64
	Data []byte
}

// TarGzip returns one deterministic ustar stream in a deterministic gzip member.
func TarGzip(files []File) ([]byte, error) {
	if err := ValidateInput(files); err != nil {
		return nil, err
	}
	files = sorted(files)
	var output bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&output, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	gzipWriter.Header = gzip.Header{ModTime: time.Unix(0, 0), OS: 255}
	tarWriter := tar.NewWriter(gzipWriter)
	for _, file := range files {
		header := &tar.Header{
			Name: file.Name, Mode: file.Mode, Size: int64(len(file.Data)),
			Uid: 0, Gid: 0, ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg,
			Format: tar.FormatUSTAR,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, err
		}
		if _, err := tarWriter.Write(file.Data); err != nil {
			return nil, err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// ZIP returns deterministic stored entries with no descriptor or extra field.
func ZIP(files []File) ([]byte, error) {
	if err := ValidateInput(files); err != nil {
		return nil, err
	}
	files = sorted(files)
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range files {
		header := &zip.FileHeader{
			Name: file.Name, Method: zip.Store, Flags: 0,
			CreatorVersion: uint16(3<<8) | 20, ReaderVersion: 20,
			ModifiedDate: 0x21, ModifiedTime: 0,
			CRC32:            crc32.ChecksumIEEE(file.Data),
			CompressedSize64: uint64(len(file.Data)), UncompressedSize64: uint64(len(file.Data)),
			ExternalAttrs: uint32((0o100000|file.Mode)&0xffff) << 16,
		}
		entry, err := writer.CreateRaw(header)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(file.Data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func sorted(files []File) []File {
	copyOfFiles := append([]File(nil), files...)
	sort.Slice(copyOfFiles, func(i, j int) bool { return copyOfFiles[i].Name < copyOfFiles[j].Name })
	return copyOfFiles
}

// ValidateInput rejects names that would make the assembler's output ambiguous.
func ValidateInput(files []File) error {
	seen, folded := map[string]struct{}{}, map[string]struct{}{}
	for _, file := range files {
		if file.Name == "" || strings.HasPrefix(file.Name, "/") || strings.Contains(file.Name, "\\") || path.Clean(file.Name) != file.Name || strings.HasPrefix(file.Name, "../") {
			return fmt.Errorf("unsafe member name %q", file.Name)
		}
		if file.Mode != 0o755 && file.Mode != 0o644 {
			return fmt.Errorf("unsafe member mode %o", file.Mode)
		}
		if _, exists := seen[file.Name]; exists {
			return fmt.Errorf("duplicate member name %q", file.Name)
		}
		seen[file.Name] = struct{}{}
		fold := strings.ToLower(file.Name)
		if _, exists := folded[fold]; exists {
			return fmt.Errorf("case-fold-colliding member name %q", file.Name)
		}
		folded[fold] = struct{}{}
	}
	return nil
}
