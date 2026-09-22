package gitauth

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func integrityGit(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	binary := os.Getenv("CORVINT_OBJECT_INTEGRITY_GIT")
	if binary == "" {
		binary = "git"
	}
	out, err := gitrun.Run(context.Background(), gitrun.NewDefaultBudget(), gitrun.Options{Binary: binary, Dir: root, Env: os.Environ(), StdoutLimit: 8 << 20}, args...)
	if err != nil {
		t.Fatalf("fixture Git %v: %v", args, err)
	}
	return out
}
func integrityHash(width int, raw []byte) []byte {
	var h hash.Hash
	if width == 64 {
		h = sha256.New()
	} else {
		h = sha1.New()
	}
	h.Write(raw)
	return h.Sum(nil)
}
func corruptLoose(t *testing.T, root, oid, kind string, raw []byte) {
	t.Helper()
	var out bytes.Buffer
	zw := zlib.NewWriter(&out)
	fmt.Fprintf(zw, "%s %d\x00", kind, len(raw))
	zw.Write(raw)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".git", "objects", oid[:2], oid[2:])
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0600); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

// A valid pack and index with one deliberately mislabeled blob proves that the
// consumer checks payload identity even when container CRC/checksums are valid.
func corruptPackedBlob(t *testing.T, root, oid string, raw []byte) {
	corruptPackedObject(t, root, oid, raw, 3)
}

func corruptPackedObject(t *testing.T, root, oid string, raw []byte, kind byte) {
	t.Helper()
	entry := []byte{}
	size := len(raw)
	first := kind<<4 | byte(size&15)
	size >>= 4
	if size > 0 {
		first |= 128
	}
	entry = append(entry, first)
	for size > 0 {
		v := byte(size & 127)
		size >>= 7
		if size > 0 {
			v |= 128
		}
		entry = append(entry, v)
	}
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	zw.Write(raw)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	entry = append(entry, compressed.Bytes()...)
	pack := []byte("PACK\x00\x00\x00\x02\x00\x00\x00\x01")
	pack = append(pack, entry...)
	checksum := integrityHash(len(oid), pack)
	pack = append(pack, checksum...)
	id, err := hex.DecodeString(oid)
	if err != nil {
		t.Fatal(err)
	}
	index := []byte{255, 't', 'O', 'c', 0, 0, 0, 2}
	for i := 0; i < 256; i++ {
		n := uint32(0)
		if i >= int(id[0]) {
			n = 1
		}
		index = binary.BigEndian.AppendUint32(index, n)
	}
	index = append(index, id...)
	index = binary.BigEndian.AppendUint32(index, crc32.ChecksumIEEE(entry))
	index = binary.BigEndian.AppendUint32(index, 12)
	index = append(index, checksum...)
	index = append(index, integrityHash(len(oid), index)...)
	dir := filepath.Join(root, ".git", "objects", "pack")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	stem := filepath.Join(dir, "pack-"+hex.EncodeToString(checksum))
	if err := os.WriteFile(stem+".pack", pack, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stem+".idx", index, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".git", "objects", oid[:2], oid[2:])); err != nil {
		t.Fatal(err)
	}
}
func integrityOID(t *testing.T, root, revision string) string {
	return strings.TrimSpace(string(integrityGit(t, root, "rev-parse", revision)))
}

func TestBlobBytesRejectsMislabeledLooseAndPackedObjects(t *testing.T) {
	for _, packed := range []bool{false, true} {
		for _, earlier := range []bool{false, true} {
			t.Run(fmt.Sprintf("packed=%t/read-earlier=%t", packed, earlier), func(t *testing.T) {
				root, _, target := makeRepo(t)
				oid := integrityOID(t, root, target+":f.go")
				repo := open(t, root)
				if earlier {
					if _, err := repo.BlobBytes(context.Background(), oid); err != nil {
						t.Fatal(err)
					}
				}
				forged := []byte(strings.Replace(bodyV2, "h := 9", "h := 0", 1))
				if packed {
					corruptPackedBlob(t, root, oid, forged)
				} else {
					corruptLoose(t, root, oid, "blob", forged)
				}
				raw, err := repo.BlobBytes(context.Background(), oid)
				t.Logf("case=blob packed=%t earlier=%t refused=%t returnedBytes=%d error=%v", packed, earlier, err != nil, len(raw), err)
				if err == nil {
					t.Fatal("mislabeled blob accepted")
				}
			})
		}
	}
}
func TestGitIntegrityCommitAndTreeLinks(t *testing.T) {
	for _, kind := range []string{"commit", "root-tree", "nested-tree"} {
		t.Run(kind, func(t *testing.T) {
			root, base, target := makeRepo(t)
			repo := open(t, root)
			oid := target
			typ := "commit"
			if kind == "root-tree" {
				oid = integrityOID(t, root, target+"^{tree}")
				typ = "tree"
			}
			if kind == "nested-tree" {
				oid = integrityOID(t, root, target+":docs")
				typ = "tree"
			}
			raw := integrityGit(t, root, "cat-file", typ, oid)
			if typ == "commit" {
				raw = append(raw, []byte("changed message\n")...)
			} else {
				old := integrityOID(t, root, target+":f.go")
				replacement := integrityOID(t, root, base+":f.go")
				if kind == "nested-tree" {
					old = integrityOID(t, root, target+":docs/rule.txt")
				}
				from, _ := hex.DecodeString(old)
				to, _ := hex.DecodeString(replacement)
				raw = bytes.Replace(raw, from, to, 1)
			}
			corruptLoose(t, root, oid, typ, raw)
			var err error
			if kind == "commit" {
				_, err = repo.CommitTree(context.Background(), target)
			} else {
				_, _, err = repo.LookupTreeEntry(context.Background(), target, "docs/rule.txt")
			}
			t.Logf("case=%s refused=%t error=%v", kind, err != nil, err)
			if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
				t.Fatalf("corrupt object link: %v", err)
			}
		})
	}
}

// The verified walk must report exactly what ls-tree reports for every honest
// path shape, so callers see no change on an uncorrupted repository.
func TestLookupTreeEntryVerifiedWalkMatchesLsTree(t *testing.T) {
	root, _, target := makeRepo(t)
	repo := open(t, root)
	for _, path := range []string{"docs/rule.txt", "docs", "f.go", "docs/missing.txt", "missing/rule.txt", "no dir/rule.txt", "f.go/inner"} {
		entry, exists, err := repo.LookupTreeEntry(context.Background(), target, path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		want := strings.TrimSpace(string(integrityGit(t, root, "ls-tree", target, "--", ":(literal)"+path)))
		got := ""
		if exists {
			got = entry.Mode + " " + entry.Type + " " + entry.OID + "\t" + path
		}
		if got != want {
			t.Fatalf("%s: got %q want %q", path, got, want)
		}
	}
}

// A diff input blob replaced under its original OID must refuse the patch
// rather than derive a silently different one (an empty patch, here).
func TestCanonicalDiffRefusesMislabeledInputBlob(t *testing.T) {
	root, base, target := makeRepo(t)
	honest, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if err != nil || len(honest) == 0 {
		t.Fatalf("honest diff %d bytes: %v", len(honest), err)
	}
	oid := integrityOID(t, root, target+":f.go")
	corruptLoose(t, root, oid, "blob", integrityGit(t, root, "cat-file", "blob", integrityOID(t, root, base+":f.go")))
	got, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || got != nil {
		t.Fatalf("mislabeled diff input: %q %v", got, err)
	}
}

func TestBlobBytesPackedValidAndWrongType(t *testing.T) {
	root, _, target := makeRepo(t)
	oid := integrityOID(t, root, target+":f.go")
	integrityGit(t, root, "repack", "-ad")
	integrityGit(t, root, "prune-packed")
	repo := open(t, root)
	raw, err := repo.BlobBytes(context.Background(), oid)
	if err != nil || string(raw) != bodyV2 {
		t.Fatalf("valid packed blob: %v", err)
	}
	for _, bad := range []string{target, integrityOID(t, root, target+"^{tree}")} {
		if _, err := repo.BlobBytes(context.Background(), bad); err == nil {
			t.Fatal("wrong object type accepted")
		}
	}
}

func TestBlobIdentitySHA256AndCancellation(t *testing.T) {
	root := t.TempDir()
	integrityGit(t, root, "init", "-q", "--object-format=sha256")
	writeFile(t, root, "data", "sha256 fixture\n")
	oid := strings.TrimSpace(string(integrityGit(t, root, "hash-object", "-w", "data")))
	if len(oid) != 64 {
		t.Fatal("fixture did not use SHA-256")
	}
	repo := open(t, root)
	if raw, err := repo.BlobBytes(context.Background(), oid); err != nil || string(raw) != "sha256 fixture\n" {
		t.Fatalf("SHA-256 read: %v", err)
	}
	corruptLoose(t, root, oid, "blob", []byte("different data\n"))
	if _, err := repo.BlobBytes(context.Background(), oid); err == nil {
		t.Fatal("SHA-256 mismatch accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := requireObjectIdentity(ctx, "blob", oid, nil); cemcode.CodeOf(err) != cemcode.GitCancelled {
		t.Fatalf("cancelled hash: %v", err)
	}
}

// CEM-CB-GIT-007: an old- or new-side blob body replaced under its original
// OID fails canonical derivation, including after an earlier derivation.
func TestCanonicalDiffRejectsMislabeledBlobInputs(t *testing.T) {
	for _, side := range []string{"old", "new"} {
		for _, earlier := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/read-earlier=%t", side, earlier), func(t *testing.T) {
				root, base, target := makeRepo(t)
				repo := open(t, root)
				if earlier {
					if _, err := repo.CanonicalDiff(context.Background(), base, target); err != nil {
						t.Fatal(err)
					}
				}
				revision, original := base, bodyV1
				if side == "new" {
					revision, original = target, bodyV2
				}
				oid := integrityOID(t, root, revision+":f.go")
				forged := []byte(strings.Replace(original, "a := 1", "a := 0", 1))
				corruptLoose(t, root, oid, "blob", forged)
				raw, err := repo.CanonicalDiff(context.Background(), base, target)
				t.Logf("case=diff side=%s earlier=%t refused=%t patchBytes=%d error=%v", side, earlier, err != nil, len(raw), err)
				if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || raw != nil {
					t.Fatalf("diff consumed mislabeled blob: %v", err)
				}
			})
		}
	}
}

// CEM-CB-024: a blob body forged to equal the other side's content leaves the
// tree entries differing while Git's diff collapses to bytes that name no blob
// at all. The coverage cross-check must refuse those bytes; a check driven
// only by the blobs the patch names cannot see this case.
func TestCanonicalDiffRefusesBlobFreeOmission(t *testing.T) {
	root, base, target := makeRepo(t)
	corruptLoose(t, root, integrityOID(t, root, target+":f.go"), "blob", []byte(bodyV1))
	raw, err := open(t, root).CanonicalDiff(context.Background(), base, target)
	t.Logf("case=blob-free-omission refused=%t patchBytes=%d error=%v", err != nil, len(raw), err)
	if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || raw != nil {
		t.Fatalf("blob-free diff accepted: bytes=%d err=%v", len(raw), err)
	}
}

// CEM-CB-GIT-008: a legacy regular-file mode Git still reads (100664) is the
// canonical 100644 entry Git lists, so lookup returns it and a mode-only
// legacy rewrite is no change, as Git's own diff reports.
func TestLegacyRegularFileModeReadsAsGitCanonical(t *testing.T) {
	root, base, _ := makeRepo(t)
	blob := integrityOID(t, root, base+":f.go")
	raw, err := hex.DecodeString(blob)
	if err != nil {
		t.Fatal(err)
	}
	commit := func(mode string) string {
		command := exec.Command("git", "hash-object", "-w", "-t", "tree", "--literally", "--stdin")
		command.Dir = root
		command.Stdin = bytes.NewReader(append([]byte(mode+" f.go\x00"), raw...))
		tree, err := command.Output()
		if err != nil {
			t.Fatal(err)
		}
		return gitCmd(t, root, "commit-tree", strings.TrimSpace(string(tree)), "-m", mode)
	}
	ctx := context.Background()
	canonical, legacy := commit("100644"), commit("100664")
	entry, exists, err := open(t, root).LookupTreeEntry(ctx, legacy, "f.go")
	if err != nil || !exists || entry != (TreeEntry{Mode: "100644", Type: "blob", OID: blob}) {
		t.Fatalf("legacy lookup = %+v exists=%t err=%v", entry, exists, err)
	}
	if raw, err := open(t, root).CanonicalDiff(ctx, canonical, legacy); err != nil || len(raw) != 0 {
		t.Fatalf("legacy mode-only diff = %d bytes, err=%v", len(raw), err)
	}
}
