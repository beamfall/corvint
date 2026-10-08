package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// minWriterCheckpointSeq is the smallest audited head a writer checkpoint is
// retained or used for (CAL-V0-115, proposed). Below it the complete audit is
// already cheap. Tests lower it to exercise the route on small stores.
var minWriterCheckpointSeq uint64 = 128

// errWriterRoute marks why the writer-checkpoint route declined. It is never
// returned to a caller: the complete route runs instead.
func errWriterRoute(where, why string) error {
	return wire.Errorf(wire.CodeSnapshotMoved, where, "writer checkpoint route: %s", why)
}

// readWriterCheckpoint returns the derived writer checkpoint, or nil when it
// is absent, unreadable, oversized or malformed. Nothing here is an error:
// the caller then runs the complete audit.
func readWriterCheckpoint(repo *intent.Repository) *journal.WriterCheckpoint {
	raw, err := intent.ReadFile(journal.WriterCheckpointPath(repo.StateDir), journal.MaxWriterCheckpointBytes)
	if err != nil {
		return nil
	}
	wc, err := journal.DecodeWriterCheckpoint(raw)
	if err != nil {
		return nil
	}
	return wc
}

// boundWriterCheckpoint returns the retained writer checkpoint and the token
// read with it, or nil unless the checkpoint was published under that token
// (CAL-V0-117, proposed). A refusing refresh replaces the token before it
// removes the checkpoint, so a stop between the two still leaves the
// checkpoint unused; the removal is cleanup.
func boundWriterCheckpoint(repo *intent.Repository) (*journal.WriterCheckpoint, writerToken) {
	wc := readWriterCheckpoint(repo)
	if wc == nil {
		return nil, writerToken{}
	}
	token := writerInvalidation(repo)
	if !token.ok || wc.Invalidation != token.digest() {
		return nil, writerToken{}
	}
	return wc, token
}

// retainWriterCheckpoint replaces the derived writer checkpoint with wc,
// bound to since: the token read before the audit wc was derived from. It
// retains nothing when that token was unobserved or has changed since, so a
// refusal recorded meanwhile is never undone. Like retainCheckpoint it is
// best effort and runs under the writer lock, so the fixed temporary name
// cannot collide; a lost write costs the next writer one complete audit. A
// checkpoint below minWriterCheckpointSeq is not kept.
func retainWriterCheckpoint(repo *intent.Repository, wc *journal.WriterCheckpoint, since writerToken) {
	if wc == nil || wc.Seq.Uint64() < minWriterCheckpointSeq || !since.ok || writerInvalidation(repo) != since {
		return
	}
	wc.Invalidation = since.digest()
	replaceDerived(journal.WriterCheckpointPath(repo.StateDir), wc.Encode())
}

// replaceDerived replaces one derived file through a fixed temporary name,
// which the writer lock keeps from colliding.
func replaceDerived(path string, raw []byte) error {
	tmp := path + ".tmp"
	os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// writerInvalidationPath names the token a refresh replaces, under the writer
// lock, before it removes the writer checkpoint (CAL-V0-117, proposed). Like
// the checkpoint it is a sibling of the state directory and derived state.
func writerInvalidationPath(repo *intent.Repository) string {
	return journal.WriterCheckpointPath(repo.StateDir) + ".invalidated"
}

// writerToken is one read of the invalidation token: empty when absent, and
// ok false when it could be neither read nor shown absent.
type writerToken struct {
	value string
	ok    bool
}

// digest is what a writer checkpoint published under this token carries.
func (t writerToken) digest() [sha256.Size]byte { return sha256.Sum256([]byte(t.value)) }

// writerInvalidation reads the invalidation token.
func writerInvalidation(repo *intent.Repository) writerToken {
	path := writerInvalidationPath(repo)
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		return writerToken{ok: true}
	}
	raw, err := intent.ReadFile(path, 64)
	if err != nil || len(raw) == 0 {
		return writerToken{}
	}
	return writerToken{value: string(raw), ok: true}
}

// invalidateWriterCheckpoint runs under the writer lock. It replaces the
// invalidation token first, which at once unbinds the retained checkpoint
// and stops a refresh whose audit began before this one from reinstalling
// one, then removes the checkpoint. A token that cannot be written still
// removes the checkpoint; an older refresh may then reinstall one, and the
// next scheduled refresh removes it again.
func invalidateWriterCheckpoint(ctx context.Context, repo *intent.Repository) {
	replaceDerived(writerInvalidationPath(repo), []byte(rand.Text()+"\n"))
	mutationStage(ctx, "refresh.invalidated")
	os.Remove(journal.WriterCheckpointPath(repo.StateDir))
}

// writerObservation is what the writer-checkpoint route observed under the
// writer lock: the checkpoint, the writer audit, the bytes it read, the
// summarized inventory and the §5.2 state the model takes.
type writerObservation struct {
	wc                          *journal.WriterCheckpoint
	token                       writerToken
	proof                       *journal.Result
	phys                        map[string]journal.PhysicalFile
	inv                         *transaction.Inventory
	head, barrier, reservations []byte
	headReceipt                 []byte
	branch                      string
	// replay is set when the caller's request is retained: the route then
	// answers it from the original entry and observes nothing else
	// (CAL-V0-187, proposed).
	replay *writerReplay
}

// writerReplay is a retained request as the writer audit found it.
type writerReplay struct {
	entry  mutation.IndexEntry
	ticket string
}

// tail is the number of receipts the writer audit walked after wc.
func (w *writerObservation) tail() uint64 {
	return w.proof.Head.LastSeq.Uint64() - w.wc.Seq.Uint64()
}

// observeWriter runs the writer audit from the retained checkpoint under the
// caller's writer lock (CAL-V0-116, proposed). decline is set whenever the
// route cannot serve this write, with nothing observed that the complete
// route would not observe again; terminal is a failed native close, which no
// route may retry past.
func observeWriter(repo *intent.Repository, headState *snapshot.Head, requestID string, forMutation bool) (w *writerObservation, decline, terminal error) {
	if headState == nil || headState.LastSeq.Uint64() < minWriterCheckpointSeq {
		return nil, errWriterRoute("head.json", "below the checkpoint threshold"), nil
	}
	wc, token := boundWriterCheckpoint(repo)
	if wc == nil {
		return nil, errWriterRoute("checkpoint", "absent, unusable or invalidated"), nil
	}
	proof, obs, err := journalReader(repo, headState).AuditForWriter(wc, requestID, forMutation)
	if obs.Cleanup != nil {
		return nil, nil, errors.Join(err, obs.Cleanup)
	}
	if err != nil {
		return nil, err, nil
	}
	if proof == nil || proof.Mode != journal.ModeWriter || proof.Pending || proof.StagingPresent || proof.IntentError != nil || obs.Files == nil || proof.Head == nil {
		return nil, errWriterRoute("audit", "not a settled writer observation"), nil
	}
	w = &writerObservation{wc: wc, token: token, proof: proof, phys: obs.Files}
	if w.head, w.barrier, w.reservations, err = journalBytes(repo); err != nil {
		return nil, err, nil
	}
	if wire.Sum(w.head) != proof.Identity.HeadSha256 {
		return nil, errWriterRoute("head.json", "head changed after the audit"), nil
	}
	// A retained request replays from its original entry; one the route
	// cannot bind to the checkpoint declines (CAL-V0-187, proposed).
	found, entry, ticketID, cleanup, err := journalReader(repo, headState).WriterReplay(proof, requestID)
	if cleanup != nil {
		return nil, nil, err
	}
	if err != nil {
		return nil, errWriterRoute("requests", err.Error()), nil
	}
	if found {
		w.replay = &writerReplay{entry: entry, ticket: ticketID}
		return w, nil, nil
	}
	// Request shards are not listed on this route, so a request file no
	// receipt posted is looked for by name: the complete route classifies it.
	rp, err := snapshot.RequestPath(requestID)
	if err != nil {
		return nil, err, nil
	}
	if _, err := os.Lstat(filepath.Join(repo.StateDir, rp)); !errors.Is(err, fs.ErrNotExist) {
		return nil, errWriterRoute(rp, "request path present or unobservable"), nil
	}
	if w.inv, err = writerInventory(repo, proof, obs.Files, wc); err != nil {
		return nil, err, nil
	}
	if w.headReceipt, err = headReceipt(repo, w.head); err != nil {
		return nil, err, nil
	}
	if w.branch, err = primaryBranch(repo); err != nil {
		return nil, err, nil
	}
	return w, nil, nil
}

// writerInventory is the transaction inventory of a writer observation:
// receipts 1..wc.Seq and the request afterimages they posted are summarized
// by the checkpoint, and every other path the complete inventory lists comes
// from the audit's listing and the bytes it read. Evidence and pinned blobs
// take their digest from their content-addressed name. A path the route
// cannot account for without a read it did not make declines the route.
func writerInventory(repo *intent.Repository, proof *journal.Result, phys map[string]journal.PhysicalFile, wc *journal.WriterCheckpoint) (*transaction.Inventory, error) {
	listing := proof.WriterListing()
	if listing == nil {
		return nil, errWriterRoute("listing", "absent")
	}
	state := map[string]bool{}
	for _, name := range stateFiles {
		state[name] = true
	}
	scanned := map[string]bool{}
	for _, dir := range scanDirectories {
		scanned[dir] = true
	}
	files := []archive.FileEntry{}
	dirs := []string{}
	for p, l := range listing {
		if l.Dir {
			if shard, ok := strings.CutPrefix(p, "requests/"); scanned[p] || (ok && !strings.Contains(shard, "/")) {
				dirs = append(dirs, p)
			}
			continue
		}
		switch {
		case state[p]:
			entry, ok, err := fileEntryWithReader(filepath.Join(repo.StateDir, p), p, intent.ReadFile, phys)
			if err != nil {
				return nil, err
			}
			if ok {
				files = append(files, entry)
			}
		case strings.HasPrefix(p, "attempts/"):
			entry, err := observedEntry(p, phys)
			if err != nil {
				return nil, err
			}
			files = append(files, entry)
		case strings.HasPrefix(p, "evidence/") || strings.HasPrefix(p, "pinned/"):
			name := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(p, "evidence/"), "pinned/"), ".json")
			if strings.HasPrefix(p, "pinned/") != strings.HasSuffix(p, ".json") || strings.Contains(name, "/") {
				return nil, errWriterRoute(p, "blob name is not content-addressed")
			}
			digest, err := wire.ParseDigest(p, name)
			if err != nil {
				return nil, err
			}
			bound, ok := intent.BoundFor(p)
			if !ok {
				bound = wire.MaxEvidenceBlobBytes
			}
			if l.Bytes < 0 || l.Bytes > int64(bound) {
				return nil, wire.Errorf(wire.CodeLimitExceeded, p, "listed blob bytes exceed bound")
			}
			files = append(files, archive.FileEntry{Path: p, Sha256: digest, Bytes: wire.SizeOf(uint64(l.Bytes))})
		}
	}
	for seq := wc.Seq.Uint64() + 1; seq <= proof.Head.LastSeq.Uint64(); seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return nil, err
		}
		entry, err := observedEntry("receipts/"+name, phys)
		if err != nil {
			return nil, err
		}
		files = append(files, entry)
	}
	for p := range phys {
		if strings.HasPrefix(p, "requests/") {
			entry, err := observedEntry(p, phys)
			if err != nil {
				return nil, err
			}
			files = append(files, entry)
		}
	}
	intentFiles, err := scanIntentWithReader(repo, intent.ReadFile, phys)
	if err != nil {
		return nil, err
	}
	files = append(files, intentFiles...)
	sort.Strings(dirs)
	archive.SortFiles(files)
	return transaction.NewSummarizedInventory(files, dirs, transaction.Elided{
		Receipts:           wc.Seq.Uint64(),
		FirstReceiptSha256: proof.Head.InitSha256,
		LastReceiptSha256:  wc.ReceiptSha256,
		ReceiptBytes:       wc.ReceiptBytes,
		Requests:           wc.Requests(),
		HasRequest:         wc.HasRequest,
		Cost:               wc.Cost,
	})
}

// observedEntry is the inventory entry of a file the writer audit read.
func observedEntry(p string, phys map[string]journal.PhysicalFile) (archive.FileEntry, error) {
	if _, ok := phys[p]; !ok {
		return archive.FileEntry{}, errWriterRoute(p, "listed file was not read")
	}
	entry, _, err := fileEntryWithReader("", p, nil, phys)
	return entry, err
}

// advanceWriterCheckpoint re-bases the checkpoint on the tail the writer
// audit just walked, under the same lock, once that tail reaches
// WriterAdvanceTail, and reports whether the next scheduled complete audit
// is due: the head this write produced is WriterRefreshInterval receipts or
// more past the complete audit the checkpoint descends from.
func advanceWriterCheckpoint(repo *intent.Repository, w *writerObservation) (refresh bool) {
	if w.tail() >= journal.WriterAdvanceTail {
		if next, err := w.proof.WriterCheckpoint(w.phys); err == nil {
			retainWriterCheckpoint(repo, next, w.token)
		}
	}
	return w.proof.Head.LastSeq.Uint64()+1-w.wc.FullSeq >= journal.WriterRefreshInterval
}

// refreshWriterCheckpoint is the scheduled complete audit (CAL-V0-117,
// proposed). It audits from receipt 1 outside the writer lock, then takes the
// lock only to replace the checkpoint with one bound to the receipt head it
// audited; a writer that appended meanwhile is walked as tail by the next
// writer, and a checkpoint whose named receipt no longer matches is refused
// when that writer rebinds it. A refusal other than a moved snapshot removes
// the checkpoint, so the next writer runs the complete audit and refuses
// too. An intent-only divergence keeps the checkpoint for the intent repair.
// A refresh publishes nothing if another refresh invalidated the checkpoint
// after its audit began: an older audit cannot undo a newer refusal.
func refreshWriterCheckpoint(ctx context.Context, repo *intent.Repository) {
	token := writerInvalidation(repo)
	head, err := readHead(repo)
	if err != nil {
		return
	}
	proof, obs, err := journalReader(repo, head).AuditForWriteObserved()
	if obs.Cleanup != nil || wire.CodeOf(err) == wire.CodeSnapshotMoved {
		return
	}
	var next *journal.WriterCheckpoint
	if err == nil && proof != nil && proof.IntentError != nil {
		return
	}
	if err == nil {
		next, err = proof.WriterCheckpoint(obs.Files)
	}
	mutationStage(ctx, "refresh.audited")
	lock, lockErr := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if lockErr != nil {
		return
	}
	defer lock.Close()
	if err != nil {
		if proof == nil || !proof.Pending {
			invalidateWriterCheckpoint(ctx, repo)
		}
		return
	}
	if now := writerInvalidation(repo); !token.ok || now != token {
		mutationStage(ctx, "refresh.superseded")
		return
	}
	if current, _ := boundWriterCheckpoint(repo); current != nil && current.FullSeq > next.FullSeq {
		return
	}
	retainWriterCheckpoint(repo, next, token)
	if raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), wire.MaxJournalHeadBytes); err == nil && wire.Sum(raw) == proof.Identity.HeadSha256 {
		retainCheckpoint(repo, proof)
	}
}

// writerStage reports one writer-route phase to the mutation stage hook.
func writerStage(ctx context.Context, name string, since time.Time) time.Time {
	now := time.Now()
	mutationStage(ctx, "fast."+name+"="+now.Sub(since).String())
	return now
}
