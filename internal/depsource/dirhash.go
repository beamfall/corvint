package depsource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// escapeModulePath is the golang.org/x/mod/module escaping rule: an uppercase
// letter becomes '!' followed by its lowercase form, so a case-insensitive file
// system cannot collide two distinct module paths. x/mod is not in this
// repository's dependency graph, so the rule is implemented here.
func escapeModulePath(value string) (string, error) {
	if value == "" {
		return "", &Error{Message: "module path is empty"}
	}
	if err := checkModulePath(value); err != nil {
		return "", err
	}
	escaped := strings.Builder{}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' {
			escaped.WriteByte('!')
			escaped.WriteRune(character - 'A' + 'a')
			continue
		}
		if character == '!' {
			return "", &Error{Message: "module path contains an unescapable '!'"}
		}
		escaped.WriteRune(character)
	}
	return escaped.String(), nil
}

// checkModulePath rejects the escaping shapes module.CheckPath rejects, because
// the escaped value is joined onto the module cache root: filepath.Join cleans
// "..", so without this a crafted replace directive aims the cache read at an
// arbitrary directory.
func checkModulePath(value string) error {
	for _, element := range strings.Split(value, "/") {
		if element == "" {
			return &Error{Message: "module path has an empty element"}
		}
		if strings.HasPrefix(element, ".") {
			return &Error{Message: "module path element begins with a dot"}
		}
		if strings.Contains(element, `\`) {
			return &Error{Message: "module path element contains a backslash"}
		}
	}
	return nil
}

// hash1 is the "h1:" directory hash: base64 of the SHA-256 over one
// "<file sha256 hex><two spaces><name>\n" line per file, names sorted.
// Confirmed against $(go env GOROOT)/src/cmd/vendor/golang.org/x/mod/sumdb/dirhash/hash.go.
func hash1(ctx context.Context, names []string, open func(string) (io.ReadCloser, error)) (string, error) {
	summary := sha256.New()
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	for _, name := range sorted {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if strings.Contains(name, "\n") {
			return "", &Error{Message: "module file name contains a newline"}
		}
		reader, err := open(name)
		if err != nil {
			return "", err
		}
		file := sha256.New()
		_, copyErr := io.Copy(file, contextReader{ctx: ctx, Reader: reader})
		reader.Close()
		if copyErr != nil {
			return "", copyErr
		}
		fmt.Fprintf(summary, "%x  %s\n", file.Sum(nil), name)
	}
	return "h1:" + base64.StdEncoding.EncodeToString(summary.Sum(nil)), nil
}

// contextReader checks cancellation before every read, so a large or slow
// file cannot hold the hash past the caller's context.
type contextReader struct {
	ctx context.Context
	io.Reader
}

func (reader contextReader) Read(content []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.Reader.Read(content)
}

// hashDirectory hashes every regular file under directory, naming each file
// "<prefix>/<relative path>" exactly as the module zip does. When wantRel names
// one of those files, its bounded excerpt and whole-file metadata are captured
// off the same handle that feeds the hash and returned alongside, so a caller
// wanting that file's content never has to reopen the path by name after
// verification (that reopen is a TOCTOU: nothing holds the directory between
// the hash read and a later one). Each open is contained by an os.Root held
// across the inventory, never follows a final symlink or blocks on a FIFO, and
// must be the same regular file the inventory recorded.
func hashDirectory(ctx context.Context, directory, prefix, wantRel string) (string, []moduleFile, *fileCapture, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return "", nil, nil, err
	}
	defer root.Close()
	files, err := directoryFiles(directory, prefix)
	if err != nil {
		return "", nil, nil, err
	}
	afterCacheInventory()
	names := make([]string, len(files))
	inventory := make(map[string]fs.FileInfo, len(files))
	for position, file := range files {
		names[position] = file.Name
		inventory[file.Name] = file.info
	}
	var wantName string
	if wantRel != "" {
		wantName = path.Join(prefix, wantRel)
	}
	var captured *fileCaptureWriter
	digest, err := hash1(ctx, names, func(name string) (io.ReadCloser, error) {
		relative := strings.TrimPrefix(name, prefix+"/")
		file, openErr := openInventoriedFile(root, filepath.FromSlash(relative), inventory[name])
		if openErr != nil || name != wantName {
			return file, openErr
		}
		captured = newFileCaptureWriter()
		return &teeReadCloser{Reader: io.TeeReader(file, captured), Closer: file}, nil
	})
	if err != nil {
		return "", nil, nil, err
	}
	if captured == nil {
		return digest, files, nil, nil
	}
	return digest, files, captured.result(), nil
}

// afterCacheInventory is a test seam between the directory inventory and the
// first open; production leaves it a no-op.
var afterCacheInventory = func() {}

// openInventoriedFile opens relative inside root without following a final
// symlink or blocking on a FIFO, then requires the descriptor to be the regular
// file the inventory recorded, so a swap after inventory fails closed.
func openInventoriedFile(root *os.Root, relative string, inventoried fs.FileInfo) (*os.File, error) {
	file, err := root.OpenFile(relative, cacheFileOpenFlags, 0)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !opened.Mode().IsRegular() {
		file.Close()
		return nil, &Error{Message: "module cache entry is not a regular file: " + relative}
	}
	if !os.SameFile(inventoried, opened) {
		file.Close()
		return nil, &Error{Message: "module cache entry changed after inventory: " + relative}
	}
	return file, nil
}

type fileCapture struct {
	content []byte
	sha256  string
	size    int64
}

type fileCaptureWriter struct {
	content bytes.Buffer
	digest  hash.Hash
	size    int64
}

func newFileCaptureWriter() *fileCaptureWriter {
	return &fileCaptureWriter{digest: sha256.New()}
}

func (writer *fileCaptureWriter) Write(content []byte) (int, error) {
	written := len(content)
	writer.size += int64(written)
	_, _ = writer.digest.Write(content)
	remaining := maxExcerptBytes - writer.content.Len()
	if remaining <= 0 {
		return written, nil
	}
	if len(content) > remaining {
		content = content[:remaining]
	}
	_, _ = writer.content.Write(content)
	return written, nil
}

func (writer *fileCaptureWriter) result() *fileCapture {
	content := writer.content.Bytes()
	if content == nil {
		content = []byte{}
	}
	return &fileCapture{content: content, sha256: fmt.Sprintf("%x", writer.digest.Sum(nil)), size: writer.size}
}

// teeReadCloser observes bytes from the same handle hash1 reads while still
// closing that handle normally.
type teeReadCloser struct {
	io.Reader
	io.Closer
}

type moduleFile struct {
	Name string // The hashed name: "<prefix>/<relative path>".
	Rel  string // The path relative to the extracted module directory.
	Size int64
	info fs.FileInfo // The inventory's Lstat record, matched against each open.
}

func directoryFiles(directory, prefix string) ([]moduleFile, error) {
	cleaned := filepath.Clean(directory)
	files := []moduleFile{}
	walkErr := filepath.WalkDir(cleaned, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if !info.Mode().IsRegular() {
			return &Error{Message: "module cache entry is not a regular file: " + current}
		}
		relative := filepath.ToSlash(strings.TrimPrefix(current, cleaned+string(filepath.Separator)))
		files = append(files, moduleFile{Name: path.Join(prefix, relative), Rel: relative, Size: info.Size(), info: info})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Rel < files[right].Rel })
	return files, nil
}
