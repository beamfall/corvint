package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/localauthority"
)

type capsuleObject struct {
	OID  string `json:"oid"`
	Data []byte `json:"data"`
}
type capsule struct {
	Profile string          `json:"profile"`
	Objects []capsuleObject `json:"objects"`
}
type gitObject struct {
	kind string
	body []byte
}
type objectSet map[string]gitObject

// Ingestion verifies raw immutable Git object bytes without invoking Git, reading
// its configuration, resolving refs or executing a repository-owned file.
func validateCapsule(c capsule) (objectSet, error) {
	if c.Profile != "corvint-source-capsule/0" || len(c.Objects) == 0 || len(c.Objects) > 1024 {
		return nil, errors.New("capsule profile/count")
	}
	objects := objectSet{}
	total := 0
	for _, o := range c.Objects {
		total += len(o.Data)
		if total > maxSource {
			return nil, errors.New("capsule byte bound")
		}
		if _, found := objects[o.OID]; found {
			return nil, errors.New("duplicate object")
		}
		var hash string
		switch len(o.OID) {
		case 40:
			s := sha1.Sum(o.Data)
			hash = hex.EncodeToString(s[:])
		case 64:
			s := sha256.Sum256(o.Data)
			hash = hex.EncodeToString(s[:])
		default:
			return nil, errors.New("object hash size")
		}
		if hash != o.OID {
			return nil, errors.New("object identity mismatch")
		}
		header, body, ok := bytes.Cut(o.Data, []byte{0})
		if !ok {
			return nil, errors.New("object header")
		}
		fields := strings.Split(string(header), " ")
		if len(fields) != 2 {
			return nil, errors.New("object header fields")
		}
		size, e := strconv.Atoi(fields[1])
		if e != nil || strconv.Itoa(size) != fields[1] || size != len(body) {
			return nil, errors.New("object size")
		}
		if fields[0] != "commit" && fields[0] != "tree" && fields[0] != "blob" {
			return nil, errors.New("object type")
		}
		objects[o.OID] = gitObject{fields[0], body}
	}
	return objects, nil
}
func (objects objectSet) tree(commit string) (string, error) {
	o, ok := objects[commit]
	if !ok || o.kind != "commit" {
		return "", errors.New("commit missing")
	}
	line, _, ok := bytes.Cut(o.body, []byte{'\n'})
	if !ok || !bytes.HasPrefix(line, []byte("tree ")) {
		return "", errors.New("commit tree header")
	}
	tree := string(line[5:])
	if len(tree) != len(commit) {
		return "", errors.New("tree hash width")
	}
	return tree, nil
}
func (objects objectSet) path(commit, path string) ([]byte, error) {
	tree, e := objects.tree(commit)
	if e != nil {
		return nil, e
	}
	parts := strings.Split(path, "/")
	for index, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, errors.New("path component")
		}
		o, ok := objects[tree]
		if !ok || o.kind != "tree" {
			return nil, errors.New("tree missing")
		}
		oid, mode, e := treeEntry(o.body, part, len(commit)/2)
		if e != nil {
			return nil, e
		}
		if index < len(parts)-1 {
			if mode != "40000" {
				return nil, errors.New("not directory")
			}
			tree = oid
			continue
		}
		if mode != "100644" {
			return nil, errors.New("nonregular source")
		}
		blob, ok := objects[oid]
		if !ok || blob.kind != "blob" {
			return nil, errors.New("blob missing")
		}
		return blob.body, nil
	}
	return nil, errors.New("empty source path")
}
func treeEntry(raw []byte, wanted string, width int) (string, string, error) {
	seen := map[string]bool{}
	result, mode := "", ""
	for len(raw) > 0 {
		head, tail, ok := bytes.Cut(raw, []byte{0})
		if !ok || len(tail) < width {
			return "", "", errors.New("tree framing")
		}
		entryMode, name, ok := strings.Cut(string(head), " ")
		if !ok || name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
			return "", "", errors.New("tree name")
		}
		if seen[name] {
			return "", "", errors.New("duplicate tree name")
		}
		seen[name] = true
		if name == wanted {
			result = hex.EncodeToString(tail[:width])
			mode = entryMode
		}
		raw = tail[width:]
	}
	if result == "" {
		return "", "", fmt.Errorf("source path absent: %s", wanted)
	}
	return result, mode, nil
}

// The initial recipe compiles one exact production file. An undeclared sibling
// cannot silently disappear from the subject whose execution is attested.
func (objects objectSet) onlyCodecPackage(commit string) error {
	tree, e := objects.tree(commit)
	if e != nil {
		return e
	}
	for _, part := range []string{"internal", "wp3codec"} {
		o, ok := objects[tree]
		if !ok || o.kind != "tree" {
			return errors.New("package tree missing")
		}
		next, mode, e := treeEntry(o.body, part, len(commit)/2)
		if e != nil {
			return e
		}
		if mode != "40000" {
			return errors.New("package path mode")
		}
		tree = next
	}
	o, ok := objects[tree]
	if !ok || o.kind != "tree" {
		return errors.New("package directory missing")
	}
	raw := o.body
	for len(raw) > 0 {
		head, tail, ok := bytes.Cut(raw, []byte{0})
		if !ok || len(tail) < len(commit)/2 {
			return errors.New("package tree framing")
		}
		mode, name, ok := strings.Cut(string(head), " ")
		if !ok || mode != "100644" {
			return errors.New("unsupported package entry")
		}
		if name != "codec.go" && !strings.HasSuffix(name, "_test.go") {
			return errors.New("undeclared package source")
		}
		raw = tail[len(commit)/2:]
	}
	return nil
}

func decodeCapsule(raw []byte, result *capsule) error {
	return localauthority.DecodeDocument(raw, result, 24<<20)
}
