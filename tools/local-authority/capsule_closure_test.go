package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/Beamfall/corvint/internal/localauthority"
	"strings"
	"testing"
)

func closureObject(kind string, body []byte, wide bool) capsuleObject {
	raw := append([]byte(fmt.Sprintf("%s %d\x00", kind, len(body))), body...)
	var oid string
	if wide {
		h := sha256.Sum256(raw)
		oid = hex.EncodeToString(h[:])
	} else {
		h := sha1.Sum(raw)
		oid = hex.EncodeToString(h[:])
	}
	return capsuleObject{OID: oid, Data: raw}
}
func closureTree(name, mode string, child capsuleObject, wide bool) capsuleObject {
	oid, _ := hex.DecodeString(child.OID)
	return closureObject("tree", append([]byte(mode+" "+name+"\x00"), oid...), wide)
}
func closureCommit(tree capsuleObject, message string, wide bool) capsuleObject {
	return closureObject("commit", []byte("tree "+tree.OID+"\n\n"+message+"\n"), wide)
}
func TestCompleteCapsuleRequiresNestedAndChangedObjects(t *testing.T) {
	t.Run("PLE-V0-003 capsule requires nested trees and changed blobs", func(t *testing.T) {
		for _, wide := range []bool{false, true} {
			t.Run(fmt.Sprint(wide), func(t *testing.T) {
				old := closureObject("blob", []byte("old"), wide)
				next := closureObject("blob", []byte("new"), wide)
				oldTree := closureTree("file", "100644", old, wide)
				newTree := closureTree("file", "100644", next, wide)
				oldRoot := closureTree("docs", "40000", oldTree, wide)
				newRoot := closureTree("docs", "40000", newTree, wide)
				base := closureCommit(oldRoot, "base", wide)
				target := closureCommit(newRoot, "target", wide)
				full := capsule{"corvint-source-capsule/0", []capsuleObject{old, next, oldTree, newTree, oldRoot, newRoot, base, target}}
				got, _, err := completeCapsule(full, base.OID, target.OID, newRoot.OID)
				if err != nil || len(got.Objects) != 9 {
					t.Fatalf("complete %d %v", len(got.Objects), err)
				}
				for _, missing := range []string{old.OID, next.OID, oldTree.OID, newTree.OID, base.OID, target.OID} {
					t.Run(missing, func(t *testing.T) {
						c := capsule{Profile: full.Profile}
						for _, o := range full.Objects {
							if o.OID != missing {
								c.Objects = append(c.Objects, o)
							}
						}
						if _, _, err := completeCapsule(c, base.OID, target.OID, newRoot.OID); err == nil {
							t.Fatal("omission accepted")
						}
					})
				}
				mixed := full
				mixed.Objects = append(append([]capsuleObject{}, full.Objects...), closureObject("blob", []byte("mixed"), !wide))
				if _, _, err := completeCapsule(mixed, base.OID, target.OID, newRoot.OID); err == nil {
					t.Fatal("mixed format")
				}
			})
		}

	})
}
func TestCompleteCapsuleAllowsUnchangedMissingBlobAndDeletedSymlink(t *testing.T) {
	blob := closureObject("blob", []byte("stable"), false)
	tree := closureTree("file", "100644", blob, false)
	base := closureCommit(tree, "base", false)
	target := closureCommit(tree, "target", false)
	c := capsule{"corvint-source-capsule/0", []capsuleObject{tree, base, target}}
	if _, _, err := completeCapsule(c, base.OID, target.OID, tree.OID); err != nil {
		t.Fatal(err)
	}
	link := closureObject("blob", []byte("destination"), false)
	linkTree := closureTree("link", "120000", link, false)
	base = closureCommit(linkTree, "base", false)
	c.Objects = append(c.Objects, base, linkTree, link, blob)
	if _, _, err := completeCapsule(c, base.OID, target.OID, tree.OID); err != nil {
		t.Fatal(err)
	}
}
func TestCompleteCapsuleRejectsMalformedTreeAndFixedObjectOverflow(t *testing.T) {
	blob := closureObject("blob", []byte("x"), false)
	good := closureTree("file", "100644", blob, false)
	_, body, _ := bytes.Cut(good.Data, []byte{0})
	for _, raw := range [][]byte{append(append([]byte{}, body...), body...), []byte("100644 broken\x00short")} {
		tree := closureObject("tree", raw, false)
		commit := closureCommit(tree, "same", false)
		c := capsule{"corvint-source-capsule/0", []capsuleObject{blob, tree, commit}}
		if _, _, err := completeCapsule(c, commit.OID, commit.OID, tree.OID); err == nil {
			t.Fatal("malformed tree")
		}
	}
	commit := closureCommit(good, "same", false)
	c := capsule{"corvint-source-capsule/0", []capsuleObject{good, commit}}
	for len(c.Objects) < 1024 {
		c.Objects = append(c.Objects, closureObject("blob", []byte(fmt.Sprint(len(c.Objects))), false))
	}
	if _, _, err := completeCapsule(c, commit.OID, commit.OID, good.OID); err == nil {
		t.Fatal("fixed empty tree escaped object budget")
	}
}

func TestCompleteCapsulePreservesLeafAndPathDepthBounds(t *testing.T) {
	blob := closureObject("blob", []byte("shared"), false)
	oid, _ := hex.DecodeString(blob.OID)
	var raw []byte
	for i := 0; i < 8192; i++ {
		raw = append(raw, []byte(fmt.Sprintf("100644 f%04d\x00", i))...)
		raw = append(raw, oid...)
	}
	leafTree := closureObject("tree", raw, false)
	root := closureTree("directory", "40000", leafTree, false)
	commit := closureCommit(root, "same", false)
	c := capsule{"corvint-source-capsule/0", []capsuleObject{leafTree, root, commit}}
	_, objects, err := completeCapsule(c, commit.OID, commit.OID, root.OID)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := objects.completeTree(commit.OID)
	if err != nil || len(entries) != 8192 {
		t.Fatalf("leaf boundary %d %v", len(entries), err)
	}
	tree := closureTree("f", "100644", blob, false)
	c = capsule{Profile: "corvint-source-capsule/0", Objects: []capsuleObject{tree}}
	for i := 0; i < 255; i++ {
		tree = closureTree("d", "40000", tree, false)
		c.Objects = append(c.Objects, tree)
	}
	commit = closureCommit(tree, "deep", false)
	c.Objects = append(c.Objects, commit)
	if _, _, err := completeCapsule(c, commit.OID, commit.OID, tree.OID); err != nil {
		t.Fatal("valid deep leaf", err)
	}
}
func TestCompleteCapsuleValidatesEmptyDAGOnce(t *testing.T) {
	tree := closureObject("tree", nil, false)
	c := capsule{Profile: "corvint-source-capsule/0", Objects: []capsuleObject{tree}}
	for i := 0; i < 200; i++ {
		oid, _ := hex.DecodeString(tree.OID)
		body := append([]byte("40000 a\x00"), oid...)
		body = append(body, []byte("40000 b\x00")...)
		body = append(body, oid...)
		tree = closureObject("tree", body, false)
		c.Objects = append(c.Objects, tree)
	}
	commit := closureCommit(tree, "empty DAG", false)
	c.Objects = append(c.Objects, commit)
	if _, _, err := completeCapsule(c, commit.OID, commit.OID, tree.OID); err != nil {
		t.Fatal(err)
	}
}
func TestCapsuleDecoderUsesItsOwnByteBound(t *testing.T) {
	c := capsule{"corvint-source-capsule/0", []capsuleObject{closureObject("blob", bytes.Repeat([]byte("x"), 200<<10), false)}}
	raw, err := localauthority.Canonical(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) <= localauthority.MaxReceiptBytes {
		t.Fatal("fixture too small")
	}
	var decoded capsule
	if err := decodeCapsule(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, err := validateCapsule(decoded); err != nil {
		t.Fatal(err)
	}
	if localauthority.Decode(raw, &decoded) == nil {
		t.Fatal("receipt bound widened")
	}
	if decodeCapsule(bytes.Repeat([]byte(" "), (24<<20)+1), &decoded) == nil {
		t.Fatal("capsule wire bound")
	}
}

func TestCompleteCapsuleGitlinkExactRowByteBound(t *testing.T) {
	missingCommit := closureCommit(closureObject("tree", nil, false), "external", false)
	oid, _ := hex.DecodeString(missingCommit.OID)
	for _, overflow := range []bool{false, true} {
		var raw []byte
		for i := 0; i < 8192; i++ {
			name := fmt.Sprintf("f%04d", i) + strings.Repeat("x", 249)
			if overflow && i == 8191 {
				name += "x"
			}
			raw = append(raw, []byte("160000 "+name+"\x00")...)
			raw = append(raw, oid...)
		}
		leaf := closureObject("tree", raw, false)
		root := closureTree(strings.Repeat("d", 201), "40000", leaf, false)
		commit := closureCommit(root, "gitlinks", false)
		c := capsule{"corvint-source-capsule/0", []capsuleObject{leaf, root, commit}}
		_, _, err := completeCapsule(c, commit.OID, commit.OID, root.OID)
		// Each SHA-1 commit row is 512 bytes: 6 mode + 6 type + 40 OID +
		// 456 path + 4 delimiters. One extra byte must exceed 4 MiB.
		if overflow && (err == nil || !strings.Contains(err.Error(), "enumeration bound")) {
			t.Fatalf("overflow accepted: %v", err)
		}
		if !overflow && err != nil {
			t.Fatalf("exact boundary rejected: %v", err)
		}
	}
}
