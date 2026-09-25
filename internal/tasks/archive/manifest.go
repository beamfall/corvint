// Package archive implements `archive export` and `archive verify`
// (TM-V0-022, §3.5 taskman-archive/0). Export is a TM-V0-008 read: it takes
// no lock and creates nothing under the state dir or the repository; the
// only files it writes are its staging file, which it verifies and removes.
// `archive restore` is TCP-02's.
package archive

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Profile is the archive manifest profile.
const Profile = "taskman-archive/0"

// ManifestName is the first tar entry.
const ManifestName = "manifest.json"

// MaxManifestBytes bounds the manifest read on verify and on decode: the §1
// archive manifest cap (wire.MaxArchiveManifestBytes, arithmetic there).
const MaxManifestBytes = wire.MaxArchiveManifestBytes

// ManifestNodeCap is the decoded-node cap of one manifest: the top-level
// object (1), its thirteen members (13, the `files` array among them) and
// four nodes per file entry (the entry object and its three strings).
const ManifestNodeCap = 4*wire.MaxArchiveFiles + 14

// manifestParseOptions is the only widening any taskman profile uses
// (§3.5): the top-level `files` array up to wire.MaxArchiveFiles and the
// node cap above. Depth, every other array and every other document keep
// the ordinary §1 bounds.
var manifestParseOptions = wire.ParseOptions{WideArrayKey: "files", WideArrayMax: wire.MaxArchiveFiles, MaxNodes: ManifestNodeCap}

// ParseManifestDocument is the named opt-in entry point that parses
// manifest.json bytes under the archive bounds. The byte cap is checked
// before any parsing; the element and node caps fail LIMIT_EXCEEDED while
// parsing, before the document is materialized. Nothing else in the
// repository parses under these bounds.
func ParseManifestDocument(data []byte) (wire.Value, error) {
	if len(data) > MaxManifestBytes {
		return wire.Value{}, wire.Errorf(wire.CodeLimitExceeded, ManifestName, "manifest larger than %d bytes", MaxManifestBytes)
	}
	return wire.ParseWith(data, manifestParseOptions)
}

// FileEntry is one `files` element.
type FileEntry struct {
	Path   string
	Sha256 wire.Digest
	Bytes  wire.Size
}

// Manifest is a taskman-archive/0 manifest.
type Manifest struct {
	QueueID           wire.QueueID
	ExportedAtSeq     wire.Size
	HeadSha256        wire.Digest
	VersionSha256     wire.Digest
	BarrierSha256     *wire.Digest
	PrimaryWorktree   string
	IntentTreeSha256  wire.Digest
	Files             []FileEntry
	ReceiptCount      wire.Size
	LastReceiptSha256 wire.Digest
	HeadGeneration    wire.Size
	Complete          bool
}

func entryValue(f FileEntry) wire.Value {
	o := wire.NewObject()
	o.Set("path", wire.String(f.Path))
	o.Set("sha256", wire.String(string(f.Sha256)))
	o.Set("bytes", wire.String(string(f.Bytes)))
	return wire.ObjectValue(o)
}

// SortFiles orders entries as §2 requires of a non-semantic array:
// canonical element bytes ascending.
func SortFiles(files []FileEntry) {
	sort.SliceStable(files, func(i, j int) bool {
		return string(wire.Encode(entryValue(files[i]))) < string(wire.Encode(entryValue(files[j])))
	})
}

// Value renders the manifest with `files` in canonical order.
func (m *Manifest) Value() wire.Value {
	files := make([]FileEntry, len(m.Files))
	copy(files, m.Files)
	SortFiles(files)
	fv := make([]wire.Value, len(files))
	for i, f := range files {
		fv[i] = entryValue(f)
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(Profile))
	o.Set("queueId", wire.String(m.QueueID.Raw))
	o.Set("exportedAtSeq", wire.String(string(m.ExportedAtSeq)))
	o.Set("headSha256", wire.String(string(m.HeadSha256)))
	o.Set("versionSha256", wire.String(string(m.VersionSha256)))
	if m.BarrierSha256 == nil {
		o.Set("barrierSha256", wire.Null())
	} else {
		o.Set("barrierSha256", wire.String(string(*m.BarrierSha256)))
	}
	o.Set("primaryWorktree", wire.String(m.PrimaryWorktree))
	o.Set("intentTreeSha256", wire.String(string(m.IntentTreeSha256)))
	o.Set("files", wire.Array(fv...))
	o.Set("receiptCount", wire.String(string(m.ReceiptCount)))
	o.Set("lastReceiptSha256", wire.String(string(m.LastReceiptSha256)))
	o.Set("headGeneration", wire.String(string(m.HeadGeneration)))
	o.Set("complete", wire.Bool(m.Complete))
	return wire.ObjectValue(o)
}

// Encode returns the manifest.json bytes.
func (m *Manifest) Encode() []byte {
	return wire.EncodeFile(m.Value())
}

// DecodeManifest parses and validates manifest.json.
func DecodeManifest(data []byte) (*Manifest, error) {
	v, err := ParseManifestDocument(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "queueId", "exportedAtSeq", "headSha256", "versionSha256", "barrierSha256", "primaryWorktree",
		"intentTreeSha256", "files", "receiptCount", "lastReceiptSha256", "headGeneration", "complete")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), Profile); err != nil {
		return nil, err
	}
	m := &Manifest{}
	m.QueueID = r.Field("queueId").QueueID()
	m.ExportedAtSeq = r.Field("exportedAtSeq").Size()
	m.HeadSha256 = r.Field("headSha256").Digest()
	m.VersionSha256 = r.Field("versionSha256").Digest()
	m.BarrierSha256 = r.Field("barrierSha256").DigestOrNull()
	m.PrimaryWorktree = r.Field("primaryWorktree").PathText()
	m.IntentTreeSha256 = r.Field("intentTreeSha256").Digest()
	seen := map[string]bool{}
	// Array(-1): the parser already bounded `files` at wire.MaxArchiveFiles;
	// the sorted-unique check and the per-entry validation below are the
	// same as for any other non-semantic array.
	for _, f := range r.Field("files").Array(-1, false) {
		f.Closed("path", "sha256", "bytes")
		fe := FileEntry{}
		fe.Path = f.Field("path").Identifier()
		if f.Err() == nil {
			if _, err := wire.ParsePath(f.Field("path").Where(), fe.Path); err != nil {
				f.Field("path").Fail(wire.CodeOf(err), "%v", err)
			}
			if fe.Path == ManifestName || seen[fe.Path] {
				f.Field("path").Fail(wire.CodeDuplicateID, "duplicate or reserved path %q", fe.Path)
			}
			seen[fe.Path] = true
		}
		fe.Sha256 = f.Field("sha256").Digest()
		fe.Bytes = f.Field("bytes").Size()
		m.Files = append(m.Files, fe)
	}
	m.ReceiptCount = r.Field("receiptCount").Size()
	m.LastReceiptSha256 = r.Field("lastReceiptSha256").Digest()
	m.HeadGeneration = r.Field("headGeneration").Size()
	m.Complete = r.Field("complete").Bool()
	if err := r.Err(); err != nil {
		return nil, err
	}
	if len(m.Files) > wire.MaxArchiveFiles {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/files", "more than %d files entries", wire.MaxArchiveFiles)
	}
	if !m.Complete {
		return nil, wire.Errorf(wire.CodeMalformed, "/complete", "an archive with complete:false never verifies")
	}
	if m.ExportedAtSeq != m.ReceiptCount {
		return nil, wire.Errorf(wire.CodeJournalForked, "/receiptCount", "receiptCount %s differs from exportedAtSeq %s", m.ReceiptCount, m.ExportedAtSeq)
	}
	return m, nil
}
