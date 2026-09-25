// Package store is the callable durable writer (decision 0003): it applies a
// validated transaction.Plan to a real repository authority following the
// §5.2 sequence, and it creates the state dir at genesis.
//
// It decides nothing. Authorization is the caller's: this package never
// manufactures a mutation.Binding, never authenticates an actor, and never
// reads the premise as if it made a Coverage axis observed. Its whole job is
// ordering effects so that state never precedes its receipt.
package store

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// genesisDirectories are the state-dir children `corvint-tasks init` creates. They are
// the §3.4 directories the genesis transaction needs; the lane directories
// (`attempts`, `effects`, `worktrees`) arrive with the runtime slice.
var genesisDirectories = []string{"receipts", "evidence", "pinned", "requests", "staging"}

// Report is what one applied transaction did, in terms the envelope can
// state without inventing a fact.
type Report struct {
	// Outcome is the model's outcome, unchanged.
	Outcome mutation.Outcome
	// Coverage is the model's coverage, unchanged: an axis stays
	// NOT_OBSERVED until it is genuinely measured.
	Coverage transaction.Coverage
	// Receipt is the published receipt entry name, empty when nothing was
	// committed.
	Receipt string
	// Directories are the state-dir children this call created.
	Directories []string
	// Kind is the model's own result kind: Transaction, Replay, NoChange or
	// Refused. It is reported rather than collapsed into the outcome so a
	// replayed retry stays distinguishable from a fresh commit.
	Kind string
	// Detail is the model's explanation of a refusal; never queue prose.
	Detail string
	// Redone reports that this call completed a receipt an earlier call had
	// committed but not finished publishing (§5.2 crash point C2).
	Redone bool
	// Ticket is the ticket a committed mutation wrote, read back from the
	// receipt rather than from the request: a CREATE that allocated a serial
	// learns its own id here and nowhere else.
	Ticket  string
	Release string
	// OldPolicySha256 and NewPolicySha256 are the policy digests a committed
	// PolicyUpdate replaced and wrote; the receipt's pre/post entries carry
	// the same values.
	OldPolicySha256, NewPolicySha256 wire.Digest
}

// target maps one plan artifact onto the publication destination its path
// names. The plan's paths are archive-namespace paths (§3.4); the session's
// roles are the physical destinations. An unrecognized path is refused
// rather than guessed.
func target(role, path string) (authority.Target, error) {
	switch role {
	case "RECEIPT":
		return authority.Target{Role: authority.RoleReceipt, Name: strings.TrimPrefix(path, "receipts/")}, nil
	case "HEAD":
		return authority.Target{Role: authority.RoleHead, Name: path}, nil
	case "EVIDENCE":
		return authority.Target{Role: authority.RoleEvidence, Name: strings.TrimPrefix(path, "evidence/")}, nil
	case "POST":
		return postTarget(path)
	}
	return authority.Target{}, wire.Errorf(wire.CodeMalformed, path, "unknown artifact role %q", role)
}

func postTarget(path string) (authority.Target, error) {
	switch {
	case path == "VERSION":
		return authority.Target{Role: authority.RoleVersion, Name: path}, nil
	case path == "reservations.json":
		return authority.Target{Role: authority.RoleReservations, Name: path}, nil
	case path == "barrier.json":
		return authority.Target{Role: authority.RoleBarrier, Name: path}, nil
	case path == "intent/queue.json":
		return authority.Target{Role: authority.RoleQueue, Name: "queue.json"}, nil
	case path == "intent/policy.json":
		return authority.Target{Role: authority.RolePolicy, Name: "policy.json"}, nil
	case path == "intent/import-map.json":
		return authority.Target{Role: authority.RoleImportMap, Name: "import-map.json"}, nil
	case strings.HasPrefix(path, "intent/tickets/"):
		return authority.Target{Role: authority.RoleTicket, Name: strings.TrimPrefix(path, "intent/tickets/")}, nil
	case strings.HasPrefix(path, "intent/releases/"):
		return authority.Target{Role: authority.RoleRelease, Name: strings.TrimPrefix(path, "intent/releases/")}, nil
	case strings.HasPrefix(path, "pinned/"):
		return authority.Target{Role: authority.RolePin, Name: strings.TrimPrefix(path, "pinned/")}, nil
	case strings.HasPrefix(path, "evidence/"):
		return authority.Target{Role: authority.RoleEvidence, Name: strings.TrimPrefix(path, "evidence/")}, nil
	case strings.HasPrefix(path, "requests/"):
		name := path[strings.LastIndex(path, "/")+1:]
		return authority.Target{Role: authority.RoleRequest, Name: name}, nil
	}
	return authority.Target{}, wire.Errorf(wire.CodeMalformed, path, "post path has no publication destination")
}

// destination maps an archive-namespace path to the absolute file it names:
// intent paths live in the primary worktree's Git-tracked store, everything
// else under the private state dir (§3.4).
func destination(repo *intent.Repository, path string) string {
	if rest, ok := strings.CutPrefix(path, "intent/"); ok {
		return filepath.Join(repo.PrimaryWorktree, intent.Dir, rest)
	}
	return filepath.Join(repo.StateDir, path)
}

// settled applies the §5.2 redo rule to one post destination before it is
// published. A destination already holding the post bytes is settled and must
// not be written again; a destination holding the receipt's own pre bytes, or
// no file at all, must be written. A destination holding any third value is
// never overwritten: the receipt stays committed and the projection waits for
// `reconcile intent`, so a concurrent or manual edit is never destroyed.
//
// The pre digest comes from the committed receipt, which is the same evidence
// a crash-recovery redo would read, so a first write and a redo of it decide
// identically.
// publication is what the redo rule decided for one destination.
type publication int

const (
	skipPublication publication = iota // already holds the post bytes
	createPublication
	replacePublication // holds the pre bytes; replaced under CAS
)

func settled(repo *intent.Repository, path string, want wire.Digest, pre *wire.Digest) (publication, error) {
	full := destination(repo, path)
	bound, ok := intent.BoundFor(path)
	if !ok {
		bound = wire.MaxEvidenceBlobBytes
	}
	raw, err := intent.ReadFile(full, bound)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return createPublication, nil
		}
		return skipPublication, err
	}
	found := wire.Sum(raw)
	if found == want {
		return skipPublication, nil
	}
	if pre != nil && found == *pre {
		return replacePublication, nil
	}
	// §5.2 redo rule: an intent projection diverged and waits for
	// `reconcile intent`; a state-dir file only `corvint-tasks` writes has forked.
	if strings.HasPrefix(path, "intent/") {
		return skipPublication, wire.Errorf(wire.CodeIntentDiverged, path,
			"projection holds neither the pre nor the post state; not overwriting (reconcile intent)")
	}
	return skipPublication, wire.Errorf(wire.CodeJournalForked, path,
		"state file holds neither the pre nor the post state")
}

// mutable reports whether a destination is replaced (write .tmp, fsync,
// rename) rather than link-created. The §5.2 mutable state files and every
// intent projection are replaced: a projection legitimately already holds the
// pre state, so an exclusive create would fail on the ordinary path.
//
// Everything content-addressed or append-only is link-created instead —
// receipts, evidence, pinned documents and request-index entries — because
// those must never be overwritten at all. The redo rule decides whether a
// replace happens; this only decides how.
func mutable(t authority.Target) bool {
	switch t.Role {
	case authority.RoleHead, authority.RoleBarrier, authority.RoleReservations,
		authority.RoleQueue, authority.RolePolicy, authority.RoleImportMap,
		authority.RoleTicket, authority.RoleRelease, authority.RoleVersion:
		return true
	}
	return false
}

// apply performs the §5.2 effect order for one plan: evidence and blob-backed
// bytes first, then the receipt link-in (the commit point), then the post
// files, then the head. Each artifact is staged and fully synced before it is
// published, so a destination is never partially written.
//
// Ordering is the whole contract here: the receipt is linked in before any
// post file and before the head, so a crash can leave a committed receipt
// with unwritten projections (which redo completes) but never a projection
// or a head with no receipt.
func apply(repo *intent.Repository, session *authority.Session, plan *transaction.Plan) (receipt string, err error) {
	return applyBeforeCommit(repo, session, plan, nil)
}

func applyBeforeCommit(repo *intent.Repository, session *authority.Session, plan *transaction.Plan, beforeCommit func() error) (receipt string, err error) {
	return applyWithFaults(repo, session, plan, beforeCommit, nil)
}

func applyWithFaults(repo *intent.Repository, session *authority.Session, plan *transaction.Plan, beforeCommit, beforeBarrierDelete func() error) (receipt string, err error) {
	arts := plan.Artifacts()
	record, err := snapshot.DecodeReceipt(plan.Receipt())
	if err != nil {
		return "", err
	}
	pre := preDigests(record)
	order := map[string]int{"EVIDENCE": 0, "RECEIPT": 1, "POST": 2, "HEAD": 3}
	staged := make([][]transaction.Artifact, 4)
	for _, a := range arts {
		phase, ok := order[a.Role]
		if !ok {
			return "", wire.Errorf(wire.CodeMalformed, a.Target, "unknown artifact role %q", a.Role)
		}
		// KEEP_JOURNAL retains discarded bytes as a blob-backed evidence POST.
		// Its bytes must exist before the receipt refers to them (§5.2/§5.5).
		if a.Role == "POST" && strings.HasPrefix(a.Target, "evidence/") {
			phase = 0
		}
		staged[phase] = append(staged[phase], a)
	}
	slot := 0
	made := map[string]bool{}
	// Every slot this transaction still occupies is cleared before returning,
	// including on a failure part-way through: a slot left occupied with no
	// live descriptor is an unassigned slot that every later reader refuses.
	// A replaced destination is reached by renaming the staging file onto it,
	// which consumes the slot, so only link-published and unpublished slots
	// are left to remove.
	occupied := map[authority.Slot]bool{}
	defer func() {
		for s, live := range occupied {
			if live {
				err = errors.Join(err, session.RemoveStage(s))
			}
		}
	}()
	for phaseIndex, phase := range staged {
		for _, a := range phase {
			if a.Role == "RECEIPT" && beforeCommit != nil {
				if err := beforeCommit(); err != nil {
					return receipt, err
				}
			}
			t, err := target(a.Role, a.Target)
			if err != nil {
				return receipt, err
			}
			// What the destination must currently hold differs by role: a
			// post or blob is governed by the receipt's own pre entry, while
			// the head and the other mutable state files are simply replaced
			// over whatever this transaction observed there.
			how, expected, err := decide(repo, a, t, pre[a.Target])
			if err != nil {
				return receipt, err
			}
			if how == skipPublication {
				continue
			}
			if key, dir := parentOf(t); key != "" && !made[key] {
				if err = ensureDir(repo, session, key, dir); err != nil {
					return receipt, err
				}
				made[key] = true
			}
			if slot > 10 {
				return receipt, wire.Errorf(wire.CodeLimitExceeded, a.Target, "more than 11 staged artifacts in one transaction")
			}
			name := authority.Slot(slotName(slot))
			stage, err := session.Prepare(name, t.Role, a.Data)
			if err != nil {
				return receipt, err
			}
			occupied[name] = true
			slot++
			if err = publish(session, stage, t, how, expected); err != nil {
				return receipt, err
			}
			if how == replacePublication {
				occupied[name] = false
			}
			if a.Role == "RECEIPT" {
				receipt = t.Name
			}
		}
		if phaseIndex == 2 {
			if err := removeBarrierPost(session, record, pre, beforeBarrierDelete); err != nil {
				return receipt, err
			}
		}
	}
	return receipt, nil
}

func removeBarrierPost(session *authority.Session, record *snapshot.Receipt, pre map[string]*wire.Digest, beforeDelete func() error) error {
	for _, post := range record.Post {
		if post.Sha256 != nil {
			continue
		}
		// DecodeReceipt admits only the paired UNPAUSE barrier deletion.
		if beforeDelete != nil {
			if err := beforeDelete(); err != nil {
				return err
			}
		}
		return session.RemoveBarrier(*pre[post.Path])
	}
	return nil
}

// parentOf names the directory a target needs, as the session key and the
// path to test for existence. Only two destinations live in a directory that
// may not exist yet: a request index shard, and the intent store's tickets
// directory in a queue that has never held a ticket.
func parentOf(t authority.Target) (key, dir string) {
	switch t.Role {
	case authority.RoleRequest:
		shard := "requests/" + t.Name[:2]
		return shard, shard
	case authority.RoleTicket:
		return "tickets", "intent/tickets"
	case authority.RoleRelease:
		return "releases", "intent/releases"
	}
	return "", ""
}

// ensureDir creates a publication directory that is not already there. It is
// created by whichever transaction first publishes into it, so a later
// transaction and a redo both find it present.
func ensureDir(repo *intent.Repository, session *authority.Session, key, dir string) error {
	full := filepath.Join(repo.PrimaryWorktree, intent.Dir, strings.TrimPrefix(dir, "intent/"))
	if !strings.HasPrefix(dir, "intent/") {
		full = filepath.Join(repo.StateDir, dir)
	}
	if _, err := os.Stat(full); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return session.Mkdir(key)
}

// decide reports how one artifact must be published and, for a replacement,
// the digest its destination must currently hold. A receipt is never
// replaced: its destination must be absent, which is what makes linking it in
// the commit point.
func decide(repo *intent.Repository, a transaction.Artifact, t authority.Target, pre *wire.Digest) (publication, *wire.Digest, error) {
	if a.Role == "POST" || a.Role == "EVIDENCE" {
		how, err := settled(repo, a.Target, a.Sha256, pre)
		return how, pre, err
	}
	if !mutable(t) {
		return createPublication, nil, nil
	}
	current, err := currentDigest(repo, a.Target)
	if err != nil {
		return skipPublication, nil, err
	}
	if current == nil {
		return createPublication, nil, nil
	}
	if *current == a.Sha256 {
		return skipPublication, nil, nil
	}
	return replacePublication, current, nil
}

// currentDigest is the digest of what a destination holds now, or nil when
// there is no file there.
func currentDigest(repo *intent.Repository, path string) (*wire.Digest, error) {
	bound, ok := intent.BoundFor(path)
	if !ok {
		bound = wire.MaxEvidenceBlobBytes
	}
	raw, err := intent.ReadFile(destination(repo, path), bound)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	d := wire.Sum(raw)
	return &d, nil
}

// publish writes one staged artifact to its destination. A destination that
// must be created is link-created, which fails rather than clobbering an
// unexpected file; one that holds the pre bytes is renamed over under a CAS
// on exactly those bytes, so a file that changed between the read and the
// write is refused instead of overwritten.
func publish(session *authority.Session, stage *authority.Stage, t authority.Target, how publication, expected *wire.Digest) error {
	if how == replacePublication {
		return session.Replace(stage, t, expected)
	}
	// A create goes to an absent destination, so link(2) is right for every
	// role: it publishes atomically and fails rather than clobbering a file
	// that appeared since the destination was read.
	return session.Link(stage, t)
}

// preDigests reads the state each post destination is expected to hold before
// this transaction, as the receipt itself records it. A null pre digest means
// the receipt expects no file there.
func preDigests(receipt *snapshot.Receipt) map[string]*wire.Digest {
	out := make(map[string]*wire.Digest, len(receipt.Pre))
	for _, entry := range receipt.Pre {
		out[entry.Path] = entry.Sha256
	}
	return out
}

func slotName(i int) string {
	return string([]byte{'a', byte('0' + i/10), byte('0' + i%10)})
}

// Init creates the state dir and commits the genesis INIT transaction
// (§5.2, SPEC line 1523: VERSION, queue, policy, empty reservations, the
// pinned init record and the request index entry, all with null pre).
//
// The caller supplies the actor binding; this package neither authenticates
// nor manufactures one. The queue and policy are read from the primary
// worktree's Git-tracked intent store, which the operator authored.
func Init(ctx context.Context, repo *intent.Repository, actor mutation.Binding, requestID string, now wire.Timestamp) (*Report, error) {
	report := &Report{}
	if repo == nil {
		return report, wire.Errorf(wire.CodeMalformed, "", "no repository authority was resolved")
	}
	// An existing state dir is an initialized store. Report that as the
	// model's own refusal before qualifying, locking or creating anything:
	// a second init must not present itself as a filesystem failure.
	if _, err := os.Stat(repo.StateDir); err == nil {
		report.Outcome = mutation.Outcome{RequestID: requestID, Outcome: mutation.OutcomeBlocked}
		report.Coverage = transaction.Coverage{}
		report.Kind = "Refused"
		return report, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return guardFailure(report, requestID, err)
	}
	// Genesis binds only the queue and policy. A ticket or release record
	// already in the intent store would have no journal afterimage, and every
	// later read would refuse the store as INTENT_DIVERGED (V1-0323). Refuse
	// before anything is created (CTS-V0-001).
	record, err := existingRecord(repo.PrimaryWorktree)
	if err != nil {
		return guardFailure(report, requestID, err)
	}
	if record != "" {
		report.Kind = "Refused"
		report.Detail = "the intent store already holds " + record + "; init over existing records would freeze the store, and adopting them is not implemented"
		report.Outcome = mutation.Outcome{RequestID: requestID, Outcome: mutation.OutcomeBlocked, Codes: []string{wire.CodeIntentDiverged}}
		return report, nil
	}
	if _, err := authority.Qualify(repo.CommonDir); err != nil {
		return guardFailure(report, requestID, err)
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return guardFailure(report, requestID, err)
	}
	defer lock.Close()

	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return guardFailure(report, requestID, err)
	}
	defer session.Close()

	// The lock serializes concurrent initializers; refusal precedes state creation.
	if _, err := os.Lstat(repo.StateDir); err == nil {
		report.Kind = "Refused"
		report.Outcome = mutation.Outcome{RequestID: requestID, Outcome: mutation.OutcomeBlocked}
		return report, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return guardFailure(report, requestID, err)
	}
	queue, policy, queueID, branch, err := readIntent(repo)
	if err != nil {
		return guardFailure(report, requestID, err)
	}
	inventory, err := transaction.NewInventory(nil, genesisDirectories)
	if err != nil {
		return guardFailure(report, requestID, err)
	}
	result := transaction.Model(
		transaction.Request{
			Operation:       transaction.Init,
			QueueID:         queueID,
			RequestID:       requestID,
			Actor:           actor,
			PrimaryWorktree: repo.PrimaryWorktree,
			Queue:           queue,
			Policy:          policy,
		},
		transaction.Input{
			Inventory:  inventory,
			Premise:    transaction.LocalOperator,
			Branch:     branch,
			Replay:     transaction.ReplayObservation{State: "ABSENT"},
			RecordedAt: now,
		},
	)
	report.Outcome = result.Outcome
	report.Coverage = result.Coverage
	report.Detail = result.Detail
	report.Kind = result.Kind
	if result.Kind != "Transaction" || result.Plan == nil {
		return report, nil
	}
	currentQueue, currentPolicy, _, currentBranch, err := readIntent(repo)
	if err != nil {
		return guardFailure(report, requestID, err)
	}
	if wire.Sum(queue) != wire.Sum(currentQueue) || wire.Sum(policy) != wire.Sum(currentPolicy) || branch != currentBranch {
		return report, wire.Errorf(wire.CodeSnapshotMoved, "intent", "initialization inputs changed")
	}
	if err = session.Mkdir("state"); err != nil {
		return guardFailure(report, requestID, err)
	}
	report.Directories = append(report.Directories, "taskman")
	for _, d := range genesisDirectories {
		if err = session.Mkdir(d); err != nil {
			return guardFailure(report, requestID, err)
		}
		report.Directories = append(report.Directories, d)
	}

	report.Receipt, err = apply(repo, session, result.Plan)
	if err != nil {
		return guardFailure(report, requestID, err)
	}
	return report, nil
}

// existingRecord names one ticket or release record already in the intent
// store of the primary worktree, or returns "" when there is none.
func existingRecord(primaryWorktree string) (string, error) {
	for _, dir := range []string{intent.TicketsDir, intent.ReleasesDir} {
		name, err := firstEntry(filepath.Join(primaryWorktree, intent.Dir, dir))
		if err != nil {
			return "", err
		}
		if name != "" {
			return dir + "/" + name, nil
		}
	}
	return "", nil
}

// firstEntry returns one name from a directory, or "" when the directory is
// empty or absent. It reads a single entry, so a large directory costs no
// more than an empty one.
func firstEntry(dir string) (string, error) {
	d, err := os.Open(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", wire.Errorf(wire.CodeUnsupportedFilesystem, dir, "cannot open: %v", err)
	}
	defer d.Close()
	names, err := d.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return "", nil
	}
	if err != nil {
		return "", wire.Errorf(wire.CodeUnsupportedFilesystem, dir, "cannot list: %v", err)
	}
	return names[0], nil
}

// readIntent reads the Git-tracked queue and policy the operator authored,
// as raw canonical bytes: the transaction binds the exact bytes, not a
// re-encoding of a decoded record. Their absence is the operator's own
// missing input, reported as such rather than defaulted into existence.
func readIntent(repo *intent.Repository) (queue, policy []byte, queueID, branch string, err error) {
	root := filepath.Join(repo.PrimaryWorktree, intent.Dir)
	queue, err = intent.ReadFile(filepath.Join(root, "queue.json"), wire.MaxQueueFileBytes)
	if err != nil {
		return nil, nil, "", "", err
	}
	policy, err = intent.ReadFile(filepath.Join(root, "policy.json"), wire.MaxPolicyFileBytes)
	if err != nil {
		return nil, nil, "", "", err
	}
	q, err := intent.DecodeQueue(queue)
	if err != nil {
		return nil, nil, "", "", err
	}
	if _, err = intent.DecodePolicy(policy); err != nil {
		return nil, nil, "", "", err
	}
	branch, err = primaryBranch(repo)
	return queue, policy, q.QueueID.Raw, branch, err
}
