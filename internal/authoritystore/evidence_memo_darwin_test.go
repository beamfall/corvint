//go:build darwin

package authoritystore

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/localauthority"
)

func TestMemoSuccessCannotReplaceFinalViewAudit(t *testing.T) {
	t.Run("PLE-V0-009 cached component success retains final view drift refusal", func(t *testing.T) {
		// This is a current-UID temp view, not a protected principal or native
		// admission. It combines actual Git memo reads with the exact final
		// audit/metadata comparison used by evidenceView.unchanged.
		root := t.TempDir()
		manifest := manifestFixture()
		manifest.Files = nil
		manifest.ReaderGID = strconv.Itoa(os.Getegid())
		files := map[string][]byte{}
		object := func(kind string, body []byte) string {
			typed := append([]byte(fmt.Sprintf("%s %d\x00", kind, len(body))), body...)
			h := sha1.Sum(typed)
			oid := hex.EncodeToString(h[:])
			var compressed bytes.Buffer
			writer := zlib.NewWriter(&compressed)
			if _, err := writer.Write(typed); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			path := ".git/objects/" + oid[:2] + "/" + oid[2:]
			raw := compressed.Bytes()
			files[path] = raw
			manifest.Files = append(manifest.Files, EvidenceFile{Path: path, OID: oid, Kind: kind, DecodedSize: strconv.Itoa(len(body)), Size: strconv.Itoa(len(raw)), SHA256: localauthority.BytesDigest(raw)})
			return oid
		}
		manifest.Tree = object("tree", nil)
		manifest.Target = object("commit", []byte("tree "+manifest.Tree+"\n\ncomponent fixture\n"))
		manifest.Base = manifest.Target
		for path, raw := range map[string][]byte{".git/HEAD": []byte(manifest.Target + "\n"), ".git/config": []byte("[core]\n\trepositoryformatversion = 0\n")} {
			files[path] = raw
			manifest.Files = append(manifest.Files, EvidenceFile{Path: path, Size: strconv.Itoa(len(raw)), SHA256: localauthority.BytesDigest(raw)})
		}
		sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
		if err := ValidateEvidenceManifest(manifest); err != nil {
			t.Fatal(err)
		}
		for path, raw := range files {
			p := filepath.Join(root, "repo", path)
			if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, raw, 0440); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(p, 0440); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.MkdirAll(filepath.Join(root, "repo/.git/refs"), 0700); err != nil {
			t.Fatal(err)
		}
		raw := encode(t, manifest)
		if err := os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0440); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Join(root, "manifest.json"), 0440); err != nil {
			t.Fatal(err)
		}
		setDirs := func(mode os.FileMode) error {
			return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return os.Chmod(path, mode)
				}
				return nil
			})
		}
		if err := setDirs(0550); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = setDirs(0700) })
		view := &evidenceView{owner: uint32(os.Getuid()), gid: uint32(os.Getegid()), manifest: manifest}
		digest := localauthority.BytesDigest(raw)
		before, err := auditTestEvidence(t, root, view, digest)
		if err != nil {
			t.Fatal(err)
		}
		immutable, err := gitauth.Open(filepath.Join(root, "repo"), gitrun.NewDefaultBudget())
		if err != nil {
			t.Fatal(err)
		}
		if err := immutable.LoadObjectFormat(context.Background()); err != nil {
			t.Fatal(err)
		}
		memo, err := gitauth.NewRequestReadMemo(immutable)
		if err != nil {
			t.Fatal(err)
		}
		defer memo.Release()
		reader, err := memo.Open(gitrun.NewDefaultBudget())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := reader.Resolve(context.Background(), manifest.Target); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(root, "repo/.git/objects", manifest.Target[:2], manifest.Target[2:])
		if err := os.Chmod(p, 0640); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("later corruption"), 0440); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, 0440); err != nil {
			t.Fatal(err)
		}
		if got, err := reader.Resolve(context.Background(), manifest.Target); err != nil || got != manifest.Target {
			t.Fatal("fixture did not exercise cached success", err)
		}
		held, err := os.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		defer held.Close()
		after, err := view.audit(context.Background(), held, false, digest)
		if err == nil && sameEvidenceNodes(before, after) {
			t.Fatal("cached success replaced final metadata refusal")
		}
	})
}
