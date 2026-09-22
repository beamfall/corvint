package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

type capsuleEntry struct{ mode, oid string }

// completeCapsule preserves the admitted size/count limits, including the fixed
// empty tree used by Git's frozen attribute isolation. No live object is read.
func completeCapsule(c capsule, base, target, tree string) (capsule, objectSet, error) {
	width := len(target)
	if (width != 40 && width != 64) || len(base) != width || len(tree) != width {
		return c, nil, errors.New("capsule format")
	}
	for _, o := range c.Objects {
		if len(o.OID) != width {
			return c, nil, errors.New("mixed capsule formats")
		}
	}
	empty := []byte("tree 0\x00")
	var emptyOID string
	if width == 40 {
		v := sha1.Sum(empty)
		emptyOID = hex.EncodeToString(v[:])
	} else {
		v := sha256.Sum256(empty)
		emptyOID = hex.EncodeToString(v[:])
	}
	found := false
	for _, o := range c.Objects {
		found = found || o.OID == emptyOID
	}
	if !found {
		c.Objects = append(c.Objects, capsuleObject{OID: emptyOID, Data: empty})
	}
	objects, err := validateCapsule(c)
	if err != nil {
		return c, nil, err
	}
	actual, err := objects.tree(target)
	if err != nil || actual != tree {
		return c, nil, errors.New("capsule target tree")
	}
	before, err := objects.completeTree(base)
	if err != nil {
		return c, nil, err
	}
	after, err := objects.completeTree(target)
	if err != nil {
		return c, nil, err
	}
	for p, entry := range before {
		if next, ok := after[p]; !ok || next != entry {
			if err := objects.requireChangedBlob(entry); err != nil {
				return c, nil, err
			}
		}
	}
	for p, entry := range after {
		if prev, ok := before[p]; !ok || prev != entry {
			if err := objects.requireChangedBlob(entry); err != nil {
				return c, nil, err
			}
		}
	}
	sort.Slice(c.Objects, func(i, j int) bool { return c.Objects[i].OID < c.Objects[j].OID })
	return c, objects, nil
}

func (objects objectSet) requireChangedBlob(entry capsuleEntry) error {
	if entry.mode == "160000" {
		return nil
	} // Git records a submodule OID without reading its history.
	o, ok := objects[entry.oid]
	if !ok || o.kind != "blob" {
		return errors.New("changed blob missing")
	}
	return nil
}

func (objects objectSet) completeTree(commit string) (map[string]capsuleEntry, error) {
	root, err := objects.tree(commit)
	if err != nil {
		return nil, err
	}
	type child struct{ name, mode, oid string }
	parsed := map[string][]child{}
	leaves := map[string]int{}
	active := map[string]bool{}
	var validate func(string) (int, error)
	validate = func(treeID string) (int, error) {
		if active[treeID] {
			return 0, errors.New("capsule tree cycle")
		}
		if n, ok := leaves[treeID]; ok {
			return n, nil
		}
		o, ok := objects[treeID]
		if !ok || o.kind != "tree" {
			return 0, errors.New("capsule tree missing")
		}
		active[treeID] = true
		defer delete(active, treeID)
		raw := o.body
		names := map[string]bool{}
		count := 0
		for len(raw) > 0 {
			head, tail, ok := bytes.Cut(raw, []byte{0})
			if !ok || len(tail) < len(commit)/2 {
				return 0, errors.New("capsule tree framing")
			}
			mode, name, ok := strings.Cut(string(head), " ")
			if !ok || names[name] || name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
				return 0, errors.New("capsule tree name")
			}
			names[name] = true
			childID := hex.EncodeToString(tail[:len(commit)/2])
			raw = tail[len(commit)/2:]
			switch mode {
			case "40000":
				n, e := validate(childID)
				if e != nil {
					return 0, e
				}
				count = min(8193, count+n)
			case "100644", "100755", "120000", "160000":
				if supplied, ok := objects[childID]; ok {
					kind := "blob"
					if mode == "160000" {
						kind = "commit"
					}
					if supplied.kind != kind {
						return 0, errors.New("capsule linked type")
					}
				}
				count = min(8193, count+1)
			default:
				return 0, fmt.Errorf("capsule tree mode: %s", mode)
			}
			parsed[treeID] = append(parsed[treeID], child{name, mode, childID})
		}
		leaves[treeID] = count
		return count, nil
	}
	count, err := validate(root)
	if err != nil {
		return nil, err
	}
	if count > 8192 {
		return nil, errors.New("capsule tree enumeration bound")
	}
	entries := map[string]capsuleEntry{}
	bytesSeen := 0
	var expand func(string, string) error
	expand = func(treeID, prefix string) error {
		// An empty DAG is already structurally validated once per supplied object.
		// Skipping its expansion avoids exponential work without charging fake leaves.
		if leaves[treeID] == 0 {
			return nil
		}
		for _, entry := range parsed[treeID] {
			p := prefix + entry.name
			if entry.mode == "40000" {
				if leaves[entry.oid] > 0 && wire.ValidatePath(p) != nil {
					return errors.New("capsule tree path")
				}
				if err := expand(entry.oid, p+"/"); err != nil {
					return err
				}
				continue
			}
			if wire.ValidatePath(p) != nil {
				return errors.New("capsule tree path")
			}
			kind := "blob"
			if entry.mode == "160000" {
				kind = "commit"
			}
			bytesSeen += len(entry.mode) + len(kind) + len(entry.oid) + len(p) + 4
			if bytesSeen > 4<<20 {
				return errors.New("capsule tree enumeration bound")
			}
			entries[p] = capsuleEntry{entry.mode, entry.oid}
		}
		return nil
	}
	if err := expand(root, ""); err != nil {
		return nil, err
	}
	return entries, nil
}

func evidenceConfiguration(format string) []byte {
	if format == "sha256" {
		return []byte("[core]\n\trepositoryformatversion = 1\n[extensions]\n\tobjectformat = sha256\n")
	}
	return []byte("[core]\n\trepositoryformatversion = 0\n")
}

func evidenceFormat(c capsule) string {
	if len(c.Objects[0].OID) == 64 {
		return "sha256"
	}
	return "sha1"
}

// requireEvidencePaths validates the finite artifact-derived demand union before
// publication. Parsing here only requests objects; the canonical verifier still
// independently validates every artifact and derives its own universe.
func (objects objectSet) requireEvidencePaths(in enrolledInput) error {
	b := in.enrollment.Binding
	cem, err := wire.ParseMap(in.raw["cem.json"])
	if err != nil || cem.BaseRevision != b.Base {
		return errors.New("capsule CEM base")
	}
	before, err := objects.completeTree(b.Base)
	if err != nil {
		return err
	}
	after, err := objects.completeTree(b.Target)
	if err != nil {
		return err
	}
	require := func(rows map[string]capsuleEntry, p string, optional bool) error {
		entry, ok := rows[p]
		if !ok {
			if optional {
				return nil
			}
			return errors.New("required capsule path missing")
		}
		if entry.mode != "100644" && entry.mode != "100755" {
			return errors.New("required capsule path mode")
		}
		return objects.requireChangedBlob(entry)
	}
	for _, e := range cem.Evidence {
		if err := require(before, e.Path, false); err != nil {
			return err
		}
		if before[e.Path].oid != e.BlobOid {
			return errors.New("capsule CEM blob binding")
		}
		if err := require(after, e.Path, true); err != nil {
			return err
		}
	}
	var ocm struct {
		Intent struct {
			Path string `json:"path"`
		} `json:"intentScope"`
		Claims []struct {
			Path string `json:"path"`
		} `json:"claims"`
	}
	if err := json.Unmarshal(in.raw["ocm.json"], &ocm); err != nil {
		return err
	}
	for _, rows := range []map[string]capsuleEntry{before, after} {
		if err := require(rows, ocm.Intent.Path, false); err != nil {
			return err
		}
	}
	for _, claim := range ocm.Claims {
		if err := require(after, claim.Path, false); err != nil {
			return err
		}
	}
	for _, check := range in.enrollment.Checks {
		if err := require(before, check.DriverPath, false); err != nil {
			return err
		}
		if err := require(after, check.DriverPath, false); err != nil {
			return err
		}
		if err := require(after, check.Subject, false); err != nil {
			return err
		}
	}
	return require(after, wire.ExcludedCEMPath, false)
}
