package store

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/evidencestore"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Archive restore (TRL-V0-870, TRL-V0-871,
// docs/specs/corvint-tasks-store-relocation-v0.md).

// RestoreReport is what one `archive restore` verified and wrote.
type RestoreReport struct {
	Verified *archive.VerifyResult
	// StateFiles and IntentWritten count the state files published and the
	// intent files created; intent files already present are left alone.
	StateFiles, IntentWritten int
	// RecordedPrimary is the archived head's primary worktree when it is
	// not this repository's; the store is then bound by `relocate`.
	RecordedPrimary string
}

// restoreDirs are the flat state-dir directories an archive may populate,
// beside the genesis directories every restored store gets.
var restoreDirs = map[string]bool{"receipts": true, "attempts": true, "effects": true, "pinned": true, "evidence": true, evidencestore.Dir: true}

var restoreFiles = map[string]bool{"VERSION": true, "head.json": true, "barrier.json": true, "programs.json": true, "pools.json": true, "reservations.json": true}

// restorePath reports whether an archive member path names a place in the
// state-dir or intent layout export writes; nothing else is ever created.
func restorePath(p string) bool {
	if !filepath.IsLocal(p) || strings.Contains(p, `\`) || filepath.ToSlash(filepath.Clean(p)) != p {
		return false
	}
	parts := strings.Split(p, "/")
	switch {
	case len(parts) == 1:
		return restoreFiles[p]
	case parts[0] == "intent":
		return len(parts) == 2 || (len(parts) == 3 && (parts[1] == "tickets" || parts[1] == "releases"))
	case parts[0] == "requests":
		return len(parts) == 3 && len(parts[1]) == 2
	}
	return len(parts) == 2 && restoreDirs[parts[0]]
}

// RestoreArchive restores the archive at path into an absent state dir.
// Every refusal is decided before the writer lock is taken and writes
// nothing (TRL-V0-870); the store is published by one rename (TRL-V0-871).
func RestoreArchive(ctx context.Context, repo *intent.Repository, path string) (*RestoreReport, error) {
	if repo == nil {
		return nil, wire.Errorf(wire.CodeMalformed, "", "no repository authority was resolved")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "cannot open the archive: %v", err)
	}
	defer f.Close()
	verified, err := archive.Verify(f)
	if err != nil {
		return nil, err
	}
	m := verified.Manifest
	if verified.Head.Segments != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, "head.json", "the archived head binds journal segments; restoring a segmented journal is not supported")
	}
	want := map[string]wire.Digest{}
	for _, fe := range m.Files {
		if !restorePath(fe.Path) {
			return nil, wire.Errorf(wire.CodeMalformed, fe.Path, "archive member is outside the store layout")
		}
		want[fe.Path] = fe.Sha256
	}
	if err := restoreAbsent(repo); err != nil {
		return nil, err
	}
	missing, err := restoreIntent(repo, want)
	if err != nil {
		return nil, err
	}
	if _, err := authority.Qualify(repo.CommonDir); err != nil {
		return nil, err
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	// The lock serializes init and other restores: decide again under it.
	if err := restoreAbsent(repo); err != nil {
		return nil, err
	}
	if again, err := restoreIntent(repo, want); err != nil {
		return nil, err
	} else if len(again) != len(missing) {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, "intent", "the intent root changed during the restore")
	}
	report := &RestoreReport{Verified: verified}
	if m.PrimaryWorktree != repo.PrimaryWorktree {
		report.RecordedPrimary = m.PrimaryWorktree
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "cannot reread the archive: %v", err)
	}
	stage, err := os.MkdirTemp(filepath.Dir(repo.StateDir), "."+filepath.Base(repo.StateDir)+"-restore-")
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, repo.StateDir, "cannot create the restore staging directory: %v", err)
	}
	published := false
	defer func() {
		if !published {
			os.RemoveAll(stage)
		}
	}()
	intentRaw, err := restoreStage(f, stage, want, report)
	if err != nil {
		return nil, err
	}
	created, err := restoreWriteIntent(repo, missing, intentRaw)
	if err != nil {
		return nil, err
	}
	if err := os.Rename(stage, repo.StateDir); err != nil {
		return nil, errors.Join(wire.Errorf(wire.CodeUnsupportedFilesystem, repo.StateDir, "cannot publish the restored state dir: %v", err), removeCreated(created))
	}
	published = true
	report.IntentWritten = len(created)
	if err := authority.SyncDirectory(filepath.Dir(repo.StateDir)); err != nil {
		return report, err
	}
	return report, nil
}

func restoreAbsent(repo *intent.Repository) error {
	if _, err := os.Lstat(repo.StateDir); err == nil {
		return wire.Errorf(wire.CodeUnsupported, repo.StateDir, "the state dir exists; `archive restore` restores only into an absent state dir")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, repo.StateDir, "cannot stat: %v", err)
	}
	return nil
}

// restoreIntent proves the intent root agrees with the archive: every file
// present is an archive member with the same bytes. It returns the archived
// intent members absent from the intent root.
func restoreIntent(repo *intent.Repository, want map[string]wire.Digest) (map[string]bool, error) {
	root := filepath.Join(repo.IntentRoot(), intent.Dir)
	present := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && p == root {
				return filepath.SkipDir
			}
			return wire.Errorf(wire.CodeUnsupportedFilesystem, p, "cannot read the intent root: %v", err)
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		member := "intent/" + filepath.ToSlash(rel)
		if !d.Type().IsRegular() {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, member, "intent entry is not a regular file")
		}
		digest, ok := want[member]
		if !ok {
			return wire.Errorf(wire.CodeIntentDiverged, member, "the intent root holds a file the archive does not carry")
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, member, "cannot read: %v", err)
		}
		if wire.Sum(raw) != digest {
			what := "the intent file differs from the archive's bytes"
			if member == "intent/"+intent.QueueFile {
				what = "queue.json differs from the archive's; the archive is another queue's or another revision's"
			}
			return wire.Errorf(wire.CodeIntentDiverged, member, "%s", what)
		}
		present[member] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	missing := map[string]bool{}
	for p := range want {
		if strings.HasPrefix(p, "intent/") && !present[p] {
			missing[p] = true
		}
	}
	return missing, nil
}

// restoreStage writes every state member into the staging directory and
// returns the intent members' bytes. Each member is proved again against
// the verified manifest, so an archive changed since verification refuses.
func restoreStage(f io.Reader, stage string, want map[string]wire.Digest, report *RestoreReport) (map[string][]byte, error) {
	for _, d := range genesisDirectories {
		if err := os.Mkdir(filepath.Join(stage, d), 0o755); err != nil {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, d, "cannot create: %v", err)
		}
	}
	intentRaw := map[string][]byte{}
	dirs := map[string]bool{stage: true}
	tr := tar.NewReader(f)
	seen := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, wire.Errorf(wire.CodeMalformed, "stream", "archive changed after verification: %v", err)
		}
		if hdr.Name == archive.ManifestName {
			continue
		}
		digest, ok := want[hdr.Name]
		if !ok || hdr.Typeflag != tar.TypeReg || hdr.Size < 0 || hdr.Size > wire.MaxArchiveBytes {
			return nil, wire.Errorf(wire.CodeMalformed, hdr.Name, "archive changed after verification")
		}
		raw := make([]byte, hdr.Size)
		if _, err := io.ReadFull(tr, raw); err != nil {
			return nil, wire.Errorf(wire.CodeMalformed, hdr.Name, "archive changed after verification: %v", err)
		}
		h := sha256.Sum256(raw)
		if wire.Digest(hex.EncodeToString(h[:])) != digest {
			return nil, wire.Errorf(wire.CodeMalformed, hdr.Name, "archive changed after verification")
		}
		seen++
		if strings.HasPrefix(hdr.Name, "intent/") {
			intentRaw[hdr.Name] = raw
			continue
		}
		dest := filepath.Join(stage, filepath.FromSlash(hdr.Name))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, hdr.Name, "cannot create its directory: %v", err)
		}
		dirs[filepath.Dir(dest)] = true
		if err := writeExclusive(dest, raw); err != nil {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, hdr.Name, "cannot write: %v", err)
		}
		report.StateFiles++
	}
	if seen != len(want) {
		return nil, wire.Errorf(wire.CodeMalformed, "stream", "archive changed after verification: %d of %d members", seen, len(want))
	}
	for d := range dirs {
		if err := authority.SyncDirectory(d); err != nil {
			return nil, err
		}
	}
	for _, d := range genesisDirectories {
		if err := authority.SyncDirectory(filepath.Join(stage, d)); err != nil {
			return nil, err
		}
	}
	return intentRaw, nil
}

// restoreWriteIntent creates the missing intent files, each with O_EXCL so a
// file that appeared is never overwritten; on failure it removes the files
// it created, leaving the intent root as it was.
func restoreWriteIntent(repo *intent.Repository, missing map[string]bool, raw map[string][]byte) ([]string, error) {
	root := filepath.Join(repo.IntentRoot(), intent.Dir)
	var created []string
	for member := range missing {
		dest := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(member, "intent/")))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, errors.Join(wire.Errorf(wire.CodeUnsupportedFilesystem, member, "cannot create its directory: %v", err), removeCreated(created))
		}
		if err := writeIntentExcl(dest, raw[member]); err != nil {
			return nil, errors.Join(wire.Errorf(wire.CodeUnsupportedFilesystem, member, "cannot write: %v", err), removeCreated(created))
		}
		created = append(created, dest)
	}
	return created, nil
}

func removeCreated(paths []string) error {
	var errs []error
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// writeIntentExcl creates one tracked intent file and syncs it.
func writeIntentExcl(path string, raw []byte) error {
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := out.Write(raw); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
