package doccorpus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"os"
	"path"
	"runtime"
	"runtime/debug"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func Digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func Encode(value any) ([]byte, error) {
	data, err := json.Marshal(value, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	if len(data)+1 > MaxBytes {
		return nil, fail("output bound exceeded")
	}
	return append(data, '\n'), nil
}
func decode(data []byte, target any) error {
	if len(data) > MaxBytes {
		return fail("input bound exceeded")
	}
	if err := json.Unmarshal(data, target, json.RejectUnknownMembers(true)); err != nil {
		return fail("invalid closed JSON input")
	}
	return nil
}
func hashValue(value any) (string, error) {
	data, err := Encode(value)
	if err != nil {
		return "", err
	}
	return Digest(data), nil
}
func validPath(value string) bool {
	if value == "" || len(value) > 1024 || !utf8.ValidString(value) {
		return false
	}
	if path.Clean(value) != value || strings.HasPrefix(value, "/") || value == "." || value == ".." || strings.HasPrefix(value, "../") {
		return false
	}
	return !strings.ContainsAny(value, "\\\x00\r\n\t")
}
func textOK(value string) bool {
	return len(value) > 0 && len(value) <= 64<<10 && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}
func currentBuilder() Builder {
	revision := "unrecorded"
	dirty := false
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.modified" {
				dirty = s.Value == "true"
			}
			if s.Key == "vcs.revision" {
				revision = s.Value
			}
		}
	}
	if dirty {
		revision += "+dirty"
	}
	return Builder{Version: Schema, Revision: revision, Engine: "native-contextindex/" + Schema + "/" + runtime.Version()}
}
func ReadFile(root, path string) ([]byte, error) {
	if !validPath(path) {
		return nil, fail("invalid local input path")
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, fail("input root unavailable")
	}
	defer dir.Close()
	data, err := testvaliditydoc.ReadFile(dir, path)
	if err != nil {
		return nil, &Error{Code: "corpus-input-unavailable", Message: "input missing, unsafe or over bound"}
	}
	return data, nil
}
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	err := decode(data, &m)
	return m, err
}
func ParseArtifact(data []byte) (*Artifact, error) {
	var a Artifact
	if err := decode(data, &a); err != nil {
		return nil, err
	}
	canonical, err := Encode(a)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fail("noncanonical artifact")
	}
	digest := a.SHA256
	a.SHA256 = ""
	expected, err := hashValue(a)
	if err != nil || digest != expected {
		return nil, fail("artifact digest mismatch")
	}
	a.SHA256 = digest
	if a.Schema != Schema {
		return nil, fail("unsupported artifact schema")
	}
	return &a, nil
}
