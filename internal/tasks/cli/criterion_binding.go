package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/criterionbinding"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func criterionIdentity() (wire.CriterionIdentity, error) {
	path, e := os.Executable()
	if e != nil {
		return wire.CriterionIdentity{}, e
	}
	f, e := os.Open(path)
	if e != nil {
		return wire.CriterionIdentity{}, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() > 256<<20 {
		return wire.CriterionIdentity{}, fmt.Errorf("bounded regular verifier executable required")
	}
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, (256<<20)+1))
	if e != nil || n > 256<<20 {
		return wire.CriterionIdentity{}, fmt.Errorf("verifier executable bound")
	}
	return wire.CriterionIdentity{ExecutableSHA256: wire.Digest(hex.EncodeToString(h.Sum(nil))), Version: Version, Build: Build}, nil
}
func criterionBindingCommand(env Env, args []string) *wire.Result {
	command := []string{"criterion-binding"}
	if len(args) == 0 {
		return usage(command, "expected capture or verify")
	}
	command = append(command, args[0])
	if args[0] != "capture" && args[0] != "verify" {
		return usage(command, "unknown criterion-binding verb")
	}
	ticketID, attemptID := "", ""
	if args[0] == "verify" && len(args) != 1 {
		return usage(command, "verify consumes only canonical capture stdin")
	}
	if args[0] == "capture" {
		seen := map[string]bool{}
		for i := 1; i < len(args); i += 2 {
			if i+1 >= len(args) || seen[args[i]] || (args[i] != "--ticket" && args[i] != "--attempt") {
				return usage(command, "capture requires unique --ticket and --attempt")
			}
			seen[args[i]] = true
			if args[i] == "--ticket" {
				ticketID = args[i+1]
			} else {
				attemptID = args[i+1]
			}
		}
		if _, e := wire.ParseTicketID("ticket", ticketID); e != nil || attemptID == "" {
			return usage(command, "canonical ticket and attempt required")
		}
	}
	identity, e := criterionIdentity()
	if e != nil {
		return failure(command, nil, e)
	}
	var raw []byte
	var c wire.CriterionCapture
	var verified wire.CriterionVerification
	if args[0] == "verify" {
		raw, e = io.ReadAll(io.LimitReader(env.Stdin, wire.MaxCriterionCaptureBytes+1))
		if e == nil {
			verified, e = criterionbinding.Verify(raw, identity)
		}
	} else {
		c, verified, e = criterionCapture(env, ticketID, attemptID, identity)
	}
	if e != nil {
		return failure(command, nil, e)
	}
	end, e := criterionIdentity()
	if e != nil || end != identity {
		return failure(command, nil, wire.Errorf(wire.CodeSnapshotMoved, "executable", "verifier executable changed"))
	}
	result := &wire.Result{Command: command, Outcome: wire.OutcomeOK, Untrusted: true, Items: []wire.Value{verified.Value()}}
	if args[0] == "capture" {
		result.Snapshot = verified.Binding.Snapshot.Envelope()
		result.Items = []wire.Value{wire.CriterionCaptureResult(c, verified)}
	}
	return result
}
func criterionCapture(env Env, ticketID, attemptID string, identity wire.CriterionIdentity) (wire.CriterionCapture, wire.CriterionVerification, error) {
	var last error
	for tries := 0; tries < 3; tries++ {
		results := []*wire.Result{ticketShow(env, []string{ticketID}, true), queueStatus(env, []string{"--retries"}), readAttempt(env, []string{"show", attemptID}, false)}
		encoded := make([]string, 3)
		ok := true
		for i, result := range results {
			if result.Outcome != wire.OutcomeOK {
				last = wire.Errorf(wire.CodeSnapshotMoved, "capture", "native read refused")
				ok = false
				break
			}
			b, e := result.Encode()
			if e != nil {
				return wire.CriterionCapture{}, wire.CriterionVerification{}, e
			}
			encoded[i] = string(b)
		}
		if !ok {
			continue
		}
		s := results[1].Snapshot
		if s == nil || s.IntentTreeSha256 == nil {
			last = wire.Errorf(wire.CodeSnapshotMoved, "capture", "missing audited snapshot")
			continue
		}
		repo, e := intent.Resolve(env.Cwd)
		if e != nil {
			return wire.CriterionCapture{}, wire.CriterionVerification{}, e
		}
		store, e := intent.LoadExpecting(repo.IntentRoot(), *s.IntentTreeSha256)
		if e != nil {
			last = e
			continue
		}
		c := wire.CriterionCapture{Producer: identity, Ticket: encoded[0], Queue: encoded[1], Attempt: encoded[2], Policy: string(store.Policy.Raw)}
		claimed, e := criterionClaimedTicket(env, c, attemptID)
		if e != nil {
			last = e
			continue
		}
		c.ClaimedTicket = string(claimed)
		verified, e := criterionbinding.Verify(wire.EncodeFile(c.Value()), identity)
		if e != nil {
			last = e
			continue
		}
		if verified.Binding.TicketID.Raw != ticketID || verified.Binding.AttemptID != attemptID {
			return c, verified, wire.Errorf(wire.CodeMalformed, "capture", "requested identity mismatch")
		}
		return c, verified, nil
	}
	return wire.CriterionCapture{}, wire.CriterionVerification{}, wire.Errorf(wire.CodeSnapshotMoved, "capture", "coherent native capture unavailable: %v", last)
}

// claimedTicketSource retains one immutable afterimage while the native audit
// streams its already bounded history. Bytes are usable only after Audit passes.
type claimedTicketSource struct {
	journal.Source
	path   string
	digest wire.Digest
	raw    []byte
}

func (s *claimedTicketSource) Read(path string, max int) ([]byte, error) {
	raw, e := s.Source.Read(path, max)
	if e != nil || !strings.HasPrefix(path, "receipts/") {
		return raw, e
	}
	receipt, e := snapshot.DecodeReceipt(raw)
	if e != nil {
		return nil, e
	}
	for _, p := range receipt.Post {
		if p.Path != s.path || p.Sha256 == nil || *p.Sha256 != s.digest {
			continue
		}
		var b []byte
		if p.Record != nil {
			b = wire.EncodeFile(*p.Record)
		} else if p.BlobSha256 != nil {
			bound, e := snapshot.PostBound(p.Path)
			if e != nil {
				return nil, e
			}
			b, e = s.Source.Read("evidence/"+string(*p.BlobSha256), bound)
			if e != nil {
				return nil, e
			}
		}
		if wire.Sum(b) != s.digest {
			return nil, fmt.Errorf("claimed ticket afterimage digest mismatch")
		}
		s.raw = b
	}
	return raw, nil
}
func criterionClaimedTicket(env Env, c wire.CriterionCapture, attemptID string) ([]byte, error) {
	ar, e := wire.DecodeResult([]byte(c.Attempt))
	if e != nil || len(ar.Items) != 1 {
		return nil, fmt.Errorf("attempt capture malformed")
	}
	a, e := snapshot.DecodeAttempt(wire.EncodeFile(ar.Items[0]))
	if e != nil {
		return nil, e
	}
	tr, e := wire.DecodeResult([]byte(c.Ticket))
	if e != nil || len(tr.Items) != 1 || tr.Items[0].Obj == nil {
		return nil, fmt.Errorf("ticket capture malformed")
	}
	record, ok := tr.Items[0].Obj.Get("record")
	if !ok {
		return nil, fmt.Errorf("ticket record missing")
	}
	var claimed []byte
	_, e = withStore(env, func(rc *readCtx) error {
		path := "intent/tickets/" + a.TicketID.Local + ".json"
		attemptPath := "attempts/" + attemptID + ".json"
		source := &claimedTicketSource{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.IntentRoot()}, path: path, digest: a.TicketRecordSha256}
		audit, e := (journal.Reader{Source: source, QueueID: rc.snap.Head.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree}).Audit(path, attemptPath)
		if e != nil {
			return e
		}
		if audit.Identity.HeadSha256 != rc.snap.HeadSha256 || audit.Identity.IntentTreeSha256 != rc.snap.IntentTree || ar.Snapshot == nil || ar.Snapshot.HeadSeq == nil || *ar.Snapshot.HeadSeq != audit.LastSeq || ar.Snapshot.HeadReceiptSha256 == nil || *ar.Snapshot.HeadReceiptSha256 != audit.LastReceiptSha256 {
			return fmt.Errorf("claim history snapshot moved")
		}
		if !bytes.Equal(audit.Records[path].Raw, wire.EncodeFile(record)) || !bytes.Equal(audit.Records[attemptPath].Raw, wire.EncodeFile(ar.Items[0])) {
			return fmt.Errorf("claim history current records differ")
		}
		if len(source.raw) == 0 {
			return fmt.Errorf("claimed ticket afterimage unavailable")
		}
		claimed = source.raw
		return nil
	})
	return claimed, e
}
