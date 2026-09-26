package archive

import (
	"archive/tar"
	"math"
	"strconv"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	tarBlockBytes   = uint64(512)
	tarTrailerBytes = uint64(1024)
)

// ManifestEncoding is the exact taskman-archive/0 encoding cost of a supplied
// manifest description. Files and PayloadBytes exclude manifest.json and tar
// framing; ManifestBytes includes its trailing LF; TarBytes includes all PAX
// headers, padding and the end marker.
type ManifestEncoding struct {
	Files         int
	PayloadBytes  uint64
	ManifestBytes uint64
	TarBytes      uint64
}

// MeasureManifestEncoding validates and measures a hypothetical supplied
// taskman-archive/0 manifest without materializing its complete manifest or
// any file body. It is encoding arithmetic only: it proves no inventory,
// content digest, admission, journal, reserve or runtime fact (TM-V0-022).
func MeasureManifestEncoding(m *Manifest) (ManifestEncoding, error) {
	if m == nil {
		return ManifestEncoding{}, wire.Errorf(wire.CodeMalformed, ManifestName, "nil manifest")
	}
	if err := validateManifestFileCount(len(m.Files)); err != nil {
		return ManifestEncoding{}, err
	}

	emptyEncoding, err := validateManifestMetadata(m)
	if err != nil {
		return ManifestEncoding{}, err
	}
	payloadBytes, err := validateManifestFiles(m.Files)
	if err != nil {
		return ManifestEncoding{}, err
	}
	manifestBytes, err := measureManifestBytes(emptyEncoding, m.Files)
	if err != nil {
		return ManifestEncoding{}, err
	}
	tarBytes, err := measureTarBytes(manifestBytes, m.Files)
	if err != nil {
		return ManifestEncoding{}, err
	}
	return ManifestEncoding{
		Files:         len(m.Files),
		PayloadBytes:  payloadBytes,
		ManifestBytes: manifestBytes,
		TarBytes:      tarBytes,
	}, nil
}

func validateManifestFileCount(n int) error {
	if n > wire.MaxArchiveFiles {
		return wire.Errorf(wire.CodeLimitExceeded, "/files", "more than %d files entries", wire.MaxArchiveFiles)
	}
	return nil
}

func validateManifestMetadata(m *Manifest) ([]byte, error) {
	if _, err := wire.ParseQueueID("/queueId", m.QueueID.Raw); err != nil {
		return nil, err
	}
	if _, err := wire.ParseSize("/exportedAtSeq", string(m.ExportedAtSeq)); err != nil {
		return nil, err
	}
	if _, err := wire.ParseDigest("/headSha256", string(m.HeadSha256)); err != nil {
		return nil, err
	}
	if _, err := wire.ParseDigest("/versionSha256", string(m.VersionSha256)); err != nil {
		return nil, err
	}
	if m.BarrierSha256 != nil {
		if _, err := wire.ParseDigest("/barrierSha256", string(*m.BarrierSha256)); err != nil {
			return nil, err
		}
	}
	if _, err := wire.ParsePathText("/primaryWorktree", m.PrimaryWorktree); err != nil {
		return nil, err
	}
	if _, err := wire.ParseDigest("/intentTreeSha256", string(m.IntentTreeSha256)); err != nil {
		return nil, err
	}
	if _, err := wire.ParseSize("/receiptCount", string(m.ReceiptCount)); err != nil {
		return nil, err
	}
	if _, err := wire.ParseDigest("/lastReceiptSha256", string(m.LastReceiptSha256)); err != nil {
		return nil, err
	}
	if _, err := wire.ParseSize("/headGeneration", string(m.HeadGeneration)); err != nil {
		return nil, err
	}
	if !m.Complete {
		return nil, wire.Errorf(wire.CodeMalformed, "/complete", "an archive with complete:false never verifies")
	}
	if m.ExportedAtSeq != m.ReceiptCount {
		return nil, wire.Errorf(wire.CodeJournalForked, "/receiptCount", "receiptCount %s differs from exportedAtSeq %s", m.ReceiptCount, m.ExportedAtSeq)
	}

	empty := *m
	empty.Files = nil
	raw := empty.Encode()
	if len(raw) > MaxManifestBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, ManifestName, "manifest larger than %d bytes", MaxManifestBytes)
	}
	if _, err := DecodeManifest(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func validateManifestFiles(files []FileEntry) (uint64, error) {
	seen := make(map[string]struct{}, len(files))
	var payload uint64
	for i, f := range files {
		where := "/files/" + strconv.Itoa(i)
		pathWhere := where + "/path"
		if _, err := wire.ParseIdentifier(pathWhere, f.Path); err != nil {
			return 0, err
		}
		if _, err := wire.ParsePath(pathWhere, f.Path); err != nil {
			return 0, err
		}
		if f.Path == ManifestName {
			return 0, wire.Errorf(wire.CodeDuplicateID, pathWhere, "duplicate or reserved path %q", f.Path)
		}
		if _, ok := seen[f.Path]; ok {
			return 0, wire.Errorf(wire.CodeDuplicateID, pathWhere, "duplicate or reserved path %q", f.Path)
		}
		seen[f.Path] = struct{}{}
		if _, err := wire.ParseDigest(where+"/sha256", string(f.Sha256)); err != nil {
			return 0, err
		}
		size, err := wire.ParseSize(where+"/bytes", string(f.Bytes))
		if err != nil {
			return 0, err
		}
		n := size.Uint64()
		if n > math.MaxInt64 {
			return 0, wire.Errorf(wire.CodeLimitExceeded, where+"/bytes", "entry size exceeds int64 tar header range")
		}
		bound := uint64(boundFor(f.Path))
		if n > bound {
			return 0, wire.Errorf(wire.CodeLimitExceeded, f.Path, "entry larger than its §1 bound of %d bytes", bound)
		}
		payload, err = checkedAdd(payload, n, where+"/bytes")
		if err != nil {
			return 0, err
		}
	}
	return payload, nil
}

func measureManifestBytes(empty []byte, files []FileEntry) (uint64, error) {
	n := uint64(len(empty))
	for i, f := range files {
		if i > 0 {
			var err error
			n, err = addWithin(n, 1, uint64(MaxManifestBytes), ManifestName)
			if err != nil {
				return 0, err
			}
		}
		entryBytes := uint64(len(wire.Encode(entryValue(f))))
		var err error
		n, err = addWithin(n, entryBytes, uint64(MaxManifestBytes), ManifestName)
		if err != nil {
			return 0, err
		}
	}
	return n, nil
}

func measureTarBytes(manifestBytes uint64, files []FileEntry) (uint64, error) {
	manifestHeaderBytes, err := measureHeaderBytes(ManifestName, manifestBytes)
	if err != nil {
		return 0, err
	}
	manifestBodyBytes, err := roundTarBody(manifestBytes)
	if err != nil {
		return 0, err
	}
	total, err := addArchiveBytes(0, manifestHeaderBytes)
	if err != nil {
		return 0, err
	}
	total, err = addArchiveBytes(total, manifestBodyBytes)
	if err != nil {
		return 0, err
	}
	for _, f := range files {
		size := f.Bytes.Uint64()
		headerBytes, err := measureHeaderBytes(f.Path, size)
		if err != nil {
			return 0, err
		}
		bodyBytes, err := roundTarBody(size)
		if err != nil {
			return 0, err
		}
		total, err = addArchiveBytes(total, headerBytes)
		if err != nil {
			return 0, err
		}
		total, err = addArchiveBytes(total, bodyBytes)
		if err != nil {
			return 0, err
		}
	}
	return addArchiveBytes(total, tarTrailerBytes)
}

type countingWriter struct{ n uint64 }

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n += uint64(len(p))
	return len(p), nil
}

func measureHeaderBytes(name string, size uint64) (uint64, error) {
	if size > math.MaxInt64 {
		return 0, wire.Errorf(wire.CodeLimitExceeded, name, "entry size exceeds int64 tar header range")
	}
	w := &countingWriter{}
	tw := tar.NewWriter(w)
	if err := tw.WriteHeader(archiveHeader(name, int64(size))); err != nil {
		return 0, wire.Errorf(wire.CodeMalformed, name, "tar header: %v", err)
	}
	// Do not close: the declared body has deliberately not been written.
	return w.n, nil
}

func roundTarBody(n uint64) (uint64, error) {
	remainder := n % tarBlockBytes
	if remainder == 0 {
		return n, nil
	}
	return checkedAdd(n, tarBlockBytes-remainder, "stream")
}

func checkedAdd(a, b uint64, where string) (uint64, error) {
	if b > math.MaxUint64-a {
		return 0, wire.Errorf(wire.CodeLimitExceeded, where, "size arithmetic overflow")
	}
	return a + b, nil
}

func addWithin(a, b, limit uint64, where string) (uint64, error) {
	n, err := checkedAdd(a, b, where)
	if err != nil {
		return 0, err
	}
	if n > limit {
		return 0, wire.Errorf(wire.CodeLimitExceeded, where, "encoded size exceeds %d bytes", limit)
	}
	return n, nil
}

func addArchiveBytes(a, b uint64) (uint64, error) {
	return addWithin(a, b, wire.MaxArchiveBytes, "stream")
}
