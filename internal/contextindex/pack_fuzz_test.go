package contextindex

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

// packFuzzBase is one well-formed pack with a cochange section, built once
// per fuzzing process from the task-context fixture.
type packFuzzBase struct {
	content  []byte
	header   packHeader
	identity repositoryIdentity
}

var (
	packFuzzOnce    sync.Once
	packFuzzFixture *packFuzzBase
)

// FuzzPackSnapshotRefusesOrServesBoundedWalks splices arbitrary bytes into one
// section of a well-formed pack and reseals the section's block digests, its
// digest table sha256 and the header, so the mutation reaches the section
// decoders rather than dying at a checksum. Under IDX-SNAP-V0-003 and
// IDX-SNAP-V0-015 every load must refuse or return an index, never panic, and
// an accepted index must let every deferred body read and every term-table
// walk the ranking performs without a bound check of its own stay in range. A
// section resealed unchanged must still load.
func FuzzPackSnapshotRefusesOrServesBoundedWalks(f *testing.F) {
	f.Add(uint8(0), uint8(0), uint32(0), uint16(0), []byte{})
	f.Add(uint8(1), uint8(0), uint32(0), uint16(4), []byte{0xff, 0xff, 0xff, 0x7f})
	f.Add(uint8(2), uint8(1), uint32(8), uint16(8), []byte{0, 0, 0, 0, 0, 0, 0, 1})
	f.Add(uint8(15), uint8(0), uint32(4), uint16(4), []byte{9, 0, 0, 0})
	f.Add(uint8(17), uint8(0), uint32(64), uint16(0), []byte("zz"))
	f.Fuzz(func(t *testing.T, section, load uint8, offset uint32, cut uint16, patch []byte) {
		base := packFuzzBaseFor(t)
		pack := resealedPack(t, base, int(section)%len(base.header.Sections), int(offset), int(cut), patch)
		file := &packFile{at: bytes.NewReader(pack), size: int64(len(pack)), mapping: pack}
		loads := [3]snapshotLoad{loadFull, loadEvent, loadCompact}
		index, err := decodePackSnapshot(file, base.identity, analyzerEngine(), loads[int(load)%len(loads)])
		if err != nil && len(patch) == 0 && cut == 0 {
			t.Fatalf("section %d resealed unchanged was refused: %v", int(section)%len(base.header.Sections), err)
		}
		if err != nil {
			return
		}
		forgetPackHistory(index)
		for _, source := range index.Sources {
			source.Text()
		}
		if index.Vocabulary != nil {
			walkTermTable(index.Vocabulary)
		}
	})
}

func packFuzzBaseFor(t *testing.T) *packFuzzBase {
	packFuzzOnce.Do(func() {
		index := taskContextFixture(t)
		path := filepath.Join(t.TempDir(), "fuzz"+packExtension)
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		history := []historyEntry{{commit: index.CommitRevision, tree: index.Revision, subject: "fixture", paths: []string{"cache/cache.go", "server/server.go"}}}
		_, err = encodePackSnapshot(file, index, analyzerEngine(), history, index.CommitRevision)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		opened, err := openPackFile(path)
		if err != nil {
			t.Fatal(err)
		}
		header, err := opened.header()
		opened.close()
		opened.release()
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		packFuzzFixture = &packFuzzBase{content: content, header: header, identity: fixtureIdentity(index)}
	})
	if packFuzzFixture == nil {
		t.Skip("the pack fixture could not be built")
	}
	return packFuzzFixture
}

// resealedPack replaces content[offset:offset+cut] of the chosen section with
// patch, appends the new section and its block digest table after the
// original bytes, and rewrites the header slot to name them.
func resealedPack(t *testing.T, base *packFuzzBase, position, offset, cut int, patch []byte) []byte {
	header := base.header
	header.Sections = slices.Clone(base.header.Sections)
	entry := &header.Sections[position]
	original := base.content[entry.Offset : entry.Offset+entry.Length]
	low := min(offset, len(original))
	high := min(low+cut, len(original))
	section := slices.Concat(original[:low], patch, original[high:])
	digester := newBlockDigester(io.Discard)
	digester.Write(section)
	digests := digester.finish()
	pack := slices.Clone(base.content)
	pack = append(pack, make([]byte, (packAlignment-len(pack)%packAlignment)%packAlignment)...)
	entry.Offset, entry.Length = uint64(len(pack)), uint64(len(section))
	pack = append(pack, section...)
	entry.DigestsOffset, entry.DigestsSHA256 = uint64(len(pack)), sha256.Sum256(digests)
	pack = append(pack, digests...)
	slot, err := encodePackHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	copy(pack, slot)
	return pack
}
