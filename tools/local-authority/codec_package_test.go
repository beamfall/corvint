package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"testing"
)

// TestPLEV0013CodecPackageAdmitsTestdata pins the package-scoped codec check
// (PLE-V0-013, V1-2036): a target whose internal/wp3codec directory holds
// codec.go, regular *_test.go files and a testdata directory, as this
// repository's real package does, is admitted, and a sibling outside the
// root package directory is unconstrained. Every other package entry still
// refuses.
func TestPLEV0013CodecPackageAdmitsTestdata(t *testing.T) {
	objects := objectSet{}
	put := func(kind string, body []byte) []byte {
		raw := append([]byte(fmt.Sprintf("%s %d\x00", kind, len(body))), body...)
		h := sha1.Sum(raw)
		objects[hex.EncodeToString(h[:])] = gitObject{kind, body}
		return h[:]
	}
	type entry struct {
		mode, name string
		oid        []byte
	}
	tree := func(entries ...entry) []byte {
		var body []byte
		for _, e := range entries {
			body = append(append(body, e.mode+" "+e.name+"\x00"...), e.oid...)
		}
		return put("tree", body)
	}
	blob := put("blob", []byte("package wp3codec\n"))
	vector := tree(entry{"100644", "vector.json", put("blob", []byte("{}\n"))})
	commit := func(codec []byte) string {
		internal := tree(entry{"40000", "wp3codec", codec})
		root := tree(entry{"100644", "README.md", blob}, entry{"40000", "internal", internal}, entry{"40000", "tools", vector})
		return hex.EncodeToString(put("commit", []byte("tree "+hex.EncodeToString(root)+"\n\nfixture\n")))
	}
	real := commit(tree(entry{"100644", "codec.go", blob}, entry{"100644", "codec_test.go", blob}, entry{"100644", "fuzz_test.go", blob}, entry{"40000", "testdata", vector}))
	if err := objects.onlyCodecPackage(real); err != nil {
		t.Fatalf("real codec package with testdata refused: %v", err)
	}
	for name, codec := range map[string][]byte{
		"extra production file":   tree(entry{"100644", "codec.go", blob}, entry{"100644", "extra.go", blob}),
		"other directory":         tree(entry{"100644", "codec.go", blob}, entry{"40000", "internal", vector}),
		"testdata as a file":      tree(entry{"100644", "codec.go", blob}, entry{"100644", "testdata", blob}),
		"testdata as a symlink":   tree(entry{"100644", "codec.go", blob}, entry{"120000", "testdata", blob}),
		"executable test file":    tree(entry{"100644", "codec.go", blob}, entry{"100755", "codec_test.go", blob}),
		"assembly beside codec":   tree(entry{"100644", "codec.go", blob}, entry{"100644", "codec_amd64.s", blob}),
		"testdata as a submodule": tree(entry{"100644", "codec.go", blob}, entry{"160000", "testdata", blob}),
	} {
		if err := objects.onlyCodecPackage(commit(codec)); err == nil {
			t.Errorf("%s admitted", name)
		}
	}
}
