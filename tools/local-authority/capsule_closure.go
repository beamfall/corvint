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

// stableCapsuleRoles are the six artifact roles a stable capsule closure
// accounts (C15). The capsule never reads their bytes: they stay opaque and
// are checked only by the stable verifier over the published view.
var stableCapsuleRoles = []string{
	"tasks-capture", "tasks-verification", "tasks-claimed-ticket",
	"tasks-snapshot-head-receipt", "runner-plan", "runner-receipt",
}

// stableCapsuleClosure is the exact raw cem/1.0 map's closure: its digest,
// and one artifact per role in stableCapsuleRoles order (CEM-V1-295).
type stableCapsuleClosure struct {
	MapSHA256 string
	Roles     []wire.StableArtifact
}

// stableClosure parses the exact enrolled map and accounts the six artifact
// roles. A cem/0.x map keeps the decoder's unsupported-spec refusal and fix
// line (CEM-V1-210); a map whose base is not the enrolled base is stale.
func stableClosure(in enrolledInput) (*wire.StableMap, stableCapsuleClosure, error) {
	raw := in.raw["cem.json"]
	document, err := wire.ParseStable(raw)
	if err != nil {
		return nil, stableCapsuleClosure{}, err
	}
	if document.Change.BaseRevision != in.enrollment.Binding.Base {
		return nil, stableCapsuleClosure{}, errors.New("capsule CEM base")
	}
	closure := stableCapsuleClosure{MapSHA256: digest(raw)}
	for _, role := range stableCapsuleRoles {
		found := false
		for _, artifact := range document.Artifacts {
			if artifact.Kind == role && !found {
				closure.Roles = append(closure.Roles, artifact)
				found = true
			}
		}
		if !found {
			return nil, stableCapsuleClosure{}, errors.New("capsule stable role missing")
		}
	}
	return document, closure, nil
}

// requireEvidencePaths derives the capsule's object demand from the exact
// enrolled cem/1.0 map (CEM-V1-295..297): every evidence blob at the base
// with its bound OID, the OCM intent and claim paths, the check driver and
// subject, and the committed sidecar at the target, whose blob must be the
// enrolled cem.json bytes exactly. The capsule does not verify the map; the
// stable verifier over the published view does, with the artifact bytes.
func (objects objectSet) requireEvidencePaths(in enrolledInput) error {
	document, _, err := stableClosure(in)
	if err != nil {
		return err
	}
	b := in.enrollment.Binding
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
	for _, e := range document.Change.Evidence {
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
		for _, demand := range []struct {
			rows map[string]capsuleEntry
			path string
		}{{before, check.DriverPath}, {after, check.DriverPath}, {after, check.Subject}} {
			if err := require(demand.rows, demand.path, false); err != nil {
				return err
			}
		}
	}
	if err := require(after, wire.ExcludedCEMPath, false); err != nil {
		return err
	}
	if sidecar := objects[after[wire.ExcludedCEMPath].oid]; !bytes.Equal(sidecar.body, in.raw["cem.json"]) {
		return errors.New("capsule CEM sidecar binding")
	}
	return nil
}
