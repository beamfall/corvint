package affected

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// GeneratedPathsPath is the repository's declaration of the generated or
// vendored paths the Playwright source walk excludes (AFP-V0-444).
const GeneratedPathsPath = ".corvint/generated-paths.json"

// GeneratedPathsProfile is the declaration's closed profile name.
const GeneratedPathsProfile = "corvint-generated-paths/0"

// SourceBoundRemedy names the fixed per-file bound and the one way past it, so
// an over-bound error or unknown says how to proceed (AFP-V0-444).
var SourceBoundRemedy = fmt.Sprintf("the per-file source bound is a fixed %d bytes (MaxSourceBytes) and cannot be raised; list a generated or vendored path in %s to exclude it from the Playwright source walk", MaxSourceBytes, GeneratedPathsPath)

// MaxHashedSourceBytes bounds the streamed identity of one over-bound file
// that a source binding digests without parsing it (AFP-V0-444).
const MaxHashedSourceBytes = 1 << 30

const (
	maxGeneratedBytes   = 64 << 10
	maxGeneratedEntries = 256
)

type generatedFile struct {
	Profile string   `json:"profile"`
	Paths   []string `json:"paths"`
}

// ReadGeneratedDeclaration reads the declaration under root and returns the
// SHA-256 of its bytes and its entries, or an empty digest and nil entries
// when root declares nothing (AFP-V0-444). The entry rules are those of the
// documentation declaration (AFP-V0-160 (1)): a repository-relative file, or
// a directory when it ends in `/`, strictly ascending, at most 256, and none
// is or lies under `.corvint/`. An unreadable, oversized or invalid
// declaration is an error, never an empty one.
func ReadGeneratedDeclaration(root string) (string, []string, error) {
	source := DiskSource(root)
	body, err := source.ReadBounded(GeneratedPathsPath, maxGeneratedBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", GeneratedPathsPath, err)
	}
	var declared generatedFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return "", nil, fmt.Errorf("%s: %w", GeneratedPathsPath, err)
	}
	if declared.Profile != GeneratedPathsProfile || declared.Paths == nil || len(declared.Paths) > maxGeneratedEntries {
		return "", nil, fmt.Errorf("%s: profile must be %s with at most %d paths", GeneratedPathsPath, GeneratedPathsProfile, maxGeneratedEntries)
	}
	previous := ""
	for _, entry := range declared.Paths {
		trimmed := strings.TrimSuffix(entry, "/")
		if !ValidReadScopeEntry(entry) || entry <= previous || trimmed == ".corvint" || strings.HasPrefix(trimmed, ".corvint/") {
			return "", nil, fmt.Errorf("%s: path %q", GeneratedPathsPath, entry)
		}
		previous = entry
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), declared.Paths, nil
}

// HashOverBoundSource streams the SHA-256 of one regular file too large for
// ReadSource, up to MaxHashedSourceBytes, so a source binding still changes
// when the file does (AFP-V0-444). It never parses the bytes.
func HashOverBoundSource(root, relative string) (string, error) {
	if !ValidRelativePath(relative) {
		return "", fmt.Errorf("%w: path %q", ErrInvalidUnit, relative)
	}
	full := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(full)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %q is not a regular file", ErrInvalidUnit, relative)
	}
	file, err := os.Open(full)
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !os.SameFile(info, opened) {
		return "", fmt.Errorf("%w: %q changed while it was opened", ErrInvalidUnit, relative)
	}
	var digest hash.Hash = sha256.New()
	copied, err := io.Copy(digest, io.LimitReader(file, MaxHashedSourceBytes+1))
	if err != nil {
		return "", err
	}
	if copied > MaxHashedSourceBytes {
		return "", fmt.Errorf("%w: %q is over the %d-byte identity bound; %s", ErrWalkLimit, relative, MaxHashedSourceBytes, SourceBoundRemedy)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
