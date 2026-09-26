package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func Release(ctx context.Context, repo *intent.Repository, actor mutation.Binding, raw []byte, now wire.Timestamp) (*Report, error) {
	report := &Report{}
	req, err := release.DecodeEnvelope(raw)
	if err != nil {
		return report, err
	}
	if repo == nil {
		return report, wire.Errorf(wire.CodeMalformed, "", "no repository")
	}
	if _, err = os.Stat(repo.StateDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return report, wire.Errorf(wire.CodeUninitialized, repo.StateDir, "no store")
		}
		return report, err
	}
	if _, err = authority.Qualify(repo.CommonDir); err != nil {
		return report, err
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return report, err
	}
	defer lock.Close()
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return report, err
	}
	defer session.Close()
	headState, err := writerGuards(repo, transaction.Release)
	if err != nil {
		return report, err
	}
	if report.Redone, err = redoPending(repo, session); err != nil {
		return report, err
	}
	reader := journalReader(repo, headState)
	idx := journal.RequestIndex{Reader: reader}
	entry, found, err := idx.Lookup(req.RequestID)
	if err != nil {
		return report, err
	}
	if found {
		if entry.MutationSha256 != wire.Sum(raw) {
			return report, wire.Errorf(wire.CodeRequestIDConflict, "requestId", "different original request")
		}
		report.Outcome = entry.Outcome
		report.Outcome.Replayed = true
		report.Kind = "Replay"
		report.Release = req.ReleaseID
		return report, nil
	}
	inv, err := inventory(repo)
	if err != nil {
		return report, err
	}
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, f := range inv.Files() {
		if strings.HasPrefix(f.Path, "intent/tickets/") || strings.HasPrefix(f.Path, "intent/releases/") {
			paths = append(paths, f.Path)
		}
	}
	canon, err := reader.Audit(paths...)
	if err != nil {
		return report, err
	}
	q, err := intent.DecodeQueue(canon.Records["intent/queue.json"].Raw)
	if err != nil {
		return report, err
	}
	if !q.Fixture || q.ExecutionCutover != nil {
		return report, wire.Errorf(wire.CodeUnsupported, "queueId", "release mutation is fixture-only")
	}
	p, err := intent.DecodePolicy(canon.Records["intent/policy.json"].Raw)
	if err != nil {
		return report, err
	}
	var current *release.Record
	releases := map[string]*release.Record{}
	ticketPresent := map[string]bool{}
	tickets := map[string]*ticket.Record{}
	ticketDigests := map[string]wire.Digest{}
	for _, path := range paths[2:] {
		rr := canon.Records[path].Raw
		if strings.HasPrefix(path, "intent/releases/") {
			x, e := release.Decode(rr)
			if e != nil {
				return report, e
			}
			releases[x.ReleaseID] = x
			if x.ReleaseID == req.ReleaseID {
				current = x
			}
		} else {
			t, e := ticket.Decode(rr)
			if e != nil {
				return report, e
			}
			tickets[t.TicketID.Raw] = t
			ticketPresent[t.TicketID.Raw] = true
			ticketDigests[t.TicketID.Raw] = wire.Sum(rr)
		}
	}
	var headCommit, headTree string
	var source wire.Digest
	if req.Operation == release.OpCandidate || req.Operation == release.OpPromote {
		headCommit, headTree, source, err = ObserveSource(repo.PrimaryWorktree, req.Operation == release.OpCandidate)
		if err != nil {
			return report, err
		}
	}
	preds := map[string]wire.Digest{}
	for id, x := range releases {
		if x.Promotion != nil {
			preds[id] = release.PromotionDigest(x.Promotion)
		}
	}
	obs := release.Observation{HeadCommit: headCommit, SourceSha256: source, PolicySha256: wire.Sum(p.Raw), Tickets: tickets, TicketDigests: ticketDigests, PredecessorPromotions: preds}
	gates := make([]release.Gate, len(p.Gates))
	for i, g := range p.Gates {
		gates[i] = release.Gate{GateID: g.GateID, Kind: g.Kind, Required: g.Required}
	}
	var candidate *release.Candidate
	if req.Operation == release.OpCandidate {
		if current == nil {
			return report, wire.Errorf(wire.CodeMalformed, "releaseId", "release absent")
		}
		bindings := make([]release.TicketBinding, 0, len(current.TicketIDs))
		for _, id := range current.TicketIDs {
			t := tickets[id.Raw]
			if t == nil {
				return report, wire.Errorf(wire.CodeMalformed, "ticketIds", "ticket absent")
			}
			bindings = append(bindings, release.TicketBinding{TicketID: id, RecordSha256: ticketDigests[id.Raw], AcceptanceRevision: t.AcceptanceRevision})
		}
		pbs := make([]release.PredecessorBinding, 0, len(current.PredecessorIDs))
		for _, id := range current.PredecessorIDs {
			d, ok := preds[id]
			if !ok {
				return report, wire.Errorf(wire.CodeMalformed, "predecessorReleaseIds", "predecessor not promoted")
			}
			pbs = append(pbs, release.PredecessorBinding{ReleaseID: id, PromotionSha256: d})
		}
		candidate = &release.Candidate{RepositoryIdentity: q.RepositoryAuthorityID, HeadCommit: headCommit, HeadTree: headTree, SourceSha256: source, DefinitionSha256: release.DefinitionDigest(current), PolicySha256: wire.Sum(p.Raw), Tickets: bindings, Predecessors: pbs}
	}
	headRaw, barrier, reservations, err := journalBytes(repo)
	if err != nil {
		return report, err
	}
	branch, err := primaryBranch(repo)
	if err != nil {
		return report, err
	}
	ticketRaws := [][]byte{}
	releaseRaws := [][]byte{}
	for _, path := range paths[2:] {
		if strings.HasPrefix(path, "intent/tickets/") {
			ticketRaws = append(ticketRaws, canon.Records[path].Raw)
		} else {
			releaseRaws = append(releaseRaws, canon.Records[path].Raw)
		}
	}
	tr := transaction.Request{Operation: transaction.Release, QueueID: q.QueueID.Raw, RequestID: req.RequestID, TargetID: req.ReleaseID, Actor: actor, Envelope: raw}
	modeled := transaction.Model(tr, transaction.Input{Inventory: inv, Head: headRaw, Queue: canon.Records["intent/queue.json"].Raw, Policy: canon.Records["intent/policy.json"].Raw, Barrier: barrier, Reservations: reservations, CanonicalTickets: ticketRaws, CanonicalReleases: releaseRaws, ReleaseCandidate: candidate, ReleaseGates: gates, ReleaseObservation: obs, Premise: transaction.LocalOperator, Branch: branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now})
	report.Outcome = modeled.Outcome
	report.Coverage = modeled.Coverage
	report.Detail = modeled.Detail
	report.Kind = modeled.Kind
	report.Release = req.ReleaseID
	if modeled.Kind != "Transaction" || modeled.Plan == nil {
		return report, nil
	}
	if err = requireBranch(repo, q.IntentBranch); err != nil {
		return report, err
	}
	if err = bindObservation(repo, canon.Identity, transaction.Release); err != nil {
		return report, err
	}
	report.Receipt, err = apply(repo, session, modeled.Plan)
	return report, err
}

func gitOutput(root string, args ...string) ([]byte, error) {
	c := exec.Command("git", args...)
	c.Dir = root
	out, err := c.Output()
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
	}
	return out, nil
}

func CaptureSource(root string) (string, string, wire.Digest, error) {
	return ObserveSource(root, true)
}

func ObserveSource(root string, requireClean bool) (string, string, wire.Digest, error) {
	status, err := gitOutput(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return "", "", "", err
	}
	extra := [][]byte{}
	for _, entry := range bytes.Split(status, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		path := string(entry)
		if len(path) > 3 {
			path = path[3:]
		}
		if !strings.HasPrefix(path, ".taskman/") && requireClean {
			return "", "", "", wire.Errorf(wire.CodeDirtyWorktree, path, "source change outside .taskman")
		}
		if !strings.HasPrefix(path, ".taskman/") && strings.HasPrefix(string(entry), "?? ") {
			extra = append(extra, []byte(path))
		}
	}
	head, err := gitOutput(root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", "", "", err
	}
	tree, err := gitOutput(root, "rev-parse", "--verify", "HEAD^{tree}")
	if err != nil {
		return "", "", "", err
	}
	files, err := gitOutput(root, "ls-files", "-z")
	if err != nil {
		return "", "", "", err
	}
	names := bytes.Split(files, []byte{0})
	names = append(names, extra...)
	sort.Slice(names, func(i, j int) bool { return bytes.Compare(names[i], names[j]) < 0 })
	h := sha256.New()
	for _, n := range names {
		if len(n) == 0 || bytes.HasPrefix(n, []byte(".taskman/")) {
			continue
		}
		path := filepath.Join(root, string(n))
		info, e := os.Lstat(path)
		var raw []byte
		mode := "absent"
		if e == nil {
			mode = info.Mode().String()
			if info.Mode()&os.ModeSymlink != 0 {
				var target string
				target, e = os.Readlink(path)
				raw = []byte(target)
			} else if info.Mode().IsRegular() {
				raw, e = os.ReadFile(path)
			} else {
				return "", "", "", wire.Errorf(wire.CodeMalformed, string(n), "unsupported source file type")
			}
		}
		if e != nil {
			if !requireClean && errors.Is(e, fs.ErrNotExist) {
				raw = []byte("<absent>")
			} else {
				return "", "", "", e
			}
		}
		sum := sha256.Sum256(raw)
		h.Write(n)
		h.Write([]byte{0})
		h.Write([]byte(mode))
		h.Write([]byte{0})
		h.Write([]byte(hex.EncodeToString(sum[:])))
		h.Write([]byte{'\n'})
	}
	return string(bytes.TrimSpace(head)), string(bytes.TrimSpace(tree)), wire.Digest(hex.EncodeToString(h.Sum(nil))), nil
}
