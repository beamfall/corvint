package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestCapsuleHashAndPathBoundary(t *testing.T) {
	object := func(kind string, body []byte) capsuleObject {
		raw := append([]byte(fmt.Sprintf("%s %d\x00", kind, len(body))), body...)
		h := sha1.Sum(raw)
		return capsuleObject{hex.EncodeToString(h[:]), raw}
	}
	blob := object("blob", []byte("package wp3codec"))
	rawOID, _ := hex.DecodeString(blob.OID)
	tree := object("tree", append([]byte("100644 codec.go\x00"), rawOID...))
	commit := object("commit", []byte("tree "+tree.OID+"\n\nfixture\n"))
	c := capsule{"corvint-source-capsule/0", []capsuleObject{blob, tree, commit}}
	set, e := validateCapsule(c)
	if e != nil {
		t.Fatal(e)
	}
	b, e := set.path(commit.OID, "codec.go")
	if e != nil || string(b) != "package wp3codec" {
		t.Fatalf("%s %v", b, e)
	}
	if _, e = set.path(commit.OID, "../codec.go"); e == nil {
		t.Fatal("traversal")
	}
	c.Objects[0].Data[0] = 'x'
	if _, e = validateCapsule(c); e == nil {
		t.Fatal("object replacement")
	}
}
