package journal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Identity pins the observed state for callers combining pure library results.
// It is not a cross-snapshot transaction or authority binding.
type Identity struct{ HeadSha256, InventorySha256, IntentTreeSha256 wire.Digest }

// Record is a selected canonical latest afterimage. A nil Raw means deletion;
// a non-nil zero-length Raw is present empty evidence.
type Record struct {
	Seq    wire.Size
	Sha256 *wire.Digest
	Raw    []byte
}

// HandoffPolicySelector names one current generation, never a renewed lease's
// GrantedSeq. It requests internal structural evidence, not admission authority.
type HandoffPolicySelector struct {
	AttemptID            string
	Generation           wire.Size
	OriginalPolicySha256 wire.Digest
	PoolID, MemberID     string
}

// HandoffPolicyHistory retains one original policy blob and bounded provenance.
// Compatible covers every policy post from that blob through the audited head;
// an incompatible intermediate revision remains incompatible after restoration.
type HandoffPolicyHistory struct {
	Selector                                      HandoffPolicySelector
	OriginalPolicy                                Record
	OriginalReceiptSha256                         wire.Digest
	FirstAttemptSeq                               wire.Size
	FirstAttemptSha256, FirstAttemptReceiptSha256 wire.Digest
	FirstPolicySha256, FirstConfigSha256          wire.Digest
	FirstCapabilitySha256, FirstAllocationSha256  wire.Digest
	FinalPolicySha256                             wire.Digest
	Compatible                                    bool
}

// Result claims only structural consistency and current projection agreement.
// Even known record codecs do not validate historical acceptance decisions.
type Result struct {
	Identity              Identity
	Head                  *snapshot.Head
	LastSeq               wire.Size
	LastReceiptSha256     wire.Digest
	Pending               bool
	StagingPresent        bool
	StructuralConsistency string
	ProjectionAgreement   string
	SemanticCoverage      string
	HistoricalAcceptance  string
	ActorAuthentication   string
	Liveness              string
	RuntimeQualification  string
	Records               map[string]Record
	// RequestDigests retains bounded metadata, not historical afterimage bytes.
	// A writer reads the one requested projection under its change guard.
	RequestDigests map[string]wire.Digest
	HandoffPolicy  *HandoffPolicyHistory
	// IntentError is populated only by AuditForWrite. Private consistency is
	// still mandatory; stable intent divergence permits request replay only.
	IntentError error
	// Mode is ModeFull for an audit that walked every retained receipt.
	Mode          string
	chain         *chain
	request       *snapshot.Request
	requestTicket string
	// Set only by AuditForMutation: the per-observation selection and the
	// refusals a separate Audit of it would raise after the lookup succeeded.
	selection            map[string]bool
	selectErr, intentErr error
}

// Reader always streams receipt bytes, retaining only bounded path/digest
// metadata and explicitly selected records. All calls complete a fresh audit.
type Reader struct {
	Source          Source
	QueueID         wire.QueueID
	PrimaryWorktree string
	// SelectState also retains reservations.json, pools.json and every
	// attempt record, so one audit serves a whole read command.
	SelectState bool
	// Checkpoint opts a plain Audit into resuming from a writer-retained
	// complete audit (CAL-V0-059). It is rebound to the journal before use and
	// every refusal on that path is re-derived by the complete audit, so it
	// can only make an agreeing read cheaper. Result.Mode names what ran.
	Checkpoint      *Checkpoint
	afterCapture    func() // deterministic capture/body boundary witness
	divergentIntent string // set only on a value copy by Reconciliation
	unpauseTickets  bool   // set only on a value copy by BarrierRemoval
	physical        *PhysicalObservation
	writerCache     bool
	intentOnly      bool
	handoffPolicy   *HandoffPolicySelector
	observedIntent  bool // AuditForMutation: select queue, policy and observed tickets/releases
}

// AuditForWrite carries one verified snapshot through request lookup and
// planning. It retains canonical mutable records with the existing selection
// budget, and only digests for historical requests. It writes nothing.
func (r Reader) AuditForWrite() (*Result, error) {
	r.writerCache = true
	return r.Audit()
}

// AuditForHandoff always completes one fresh full audit. It does not mutate a
// cached writer Result or put selector-dependent evidence into the read cache.
func (r Reader) AuditForHandoff(selector HandoffPolicySelector) (*Result, error) {
	q, err := snapshot.AttemptQueue(selector.AttemptID)
	if err != nil || q != r.QueueID {
		return nil, wire.Errorf(wire.CodeMalformed, "handoff selector", "attempt queue differs")
	}
	if n, err := wire.ParseSize("handoff generation", string(selector.Generation)); err != nil || n.Uint64() == 0 {
		return nil, wire.Errorf(wire.CodeMalformed, "handoff selector", "generation is missing or malformed")
	}
	if _, err := wire.ParseDigest("handoff policy", string(selector.OriginalPolicySha256)); err != nil {
		return nil, err
	}
	if (selector.PoolID == "") != (selector.MemberID == "") {
		return nil, wire.Errorf(wire.CodeMalformed, "handoff selector", "partial allocation")
	}
	r.writerCache, r.Checkpoint, r.handoffPolicy = true, nil, &selector
	return r.Audit()
}

type limits struct{ scan, selected int }

var profileLimits = limits{wire.MaxArchiveScanEntries, wire.MaxIntentTreeBytes}

type observation struct {
	headRaw []byte
	head    *snapshot.Head
	files   map[string]os.FileInfo
	// lite leaves receipts/, requests/ and evidence/ unlisted. Their directory
	// identities stay in the inventory, so a new receipt still moves it.
	lite          bool
	intentDigests map[string]wire.Digest
	receipts      []string
	identity      Identity
	staging       bool
	stage         *snapshot.StageObservation
	stageDigests  []stageDigest
	stageErr      error
}

// Audit checks all current projections and returns selected canonical bytes.
// Pending and uninitialized observations return a partial Result plus their
// closed error code. No result on any error is a usable request index.
func (r Reader) Audit(paths ...string) (*Result, error) {
	return r.audit(paths, "", profileLimits, true)
}

// BarrierRemoval preserves unrelated ticket edits while auditing a settled
// journal for UNPAUSE. Every canonical ticket must still have a regular physical
// projection. Queue, policy, private state and untracked tickets remain strict.
func (r Reader) BarrierRemoval(paths ...string) (*Result, error) {
	r.unpauseTickets = true
	return r.Audit(paths...)
}

// Reconciliation returns canonical ticket bytes after a complete audit that
// permits only this target's physical projection to diverge. Every other intent
// and private projection remains strict; errors never supply write authority.
func (r Reader) Reconciliation(targetID string, paths ...string) (*Result, error) {
	target, err := wire.ParseTicketID("targetId", targetID)
	if err != nil {
		return nil, err
	}
	if target.QueueID() != r.QueueID.Raw {
		return nil, wire.Errorf(wire.CodeOutOfScope, "targetId", "target queue differs")
	}
	r.divergentIntent = "intent/tickets/" + target.Local + ".json"
	selected := append([]string{r.divergentIntent}, paths...)
	result, err := r.Audit(selected...)
	if err != nil {
		return result, err
	}
	record, ok := result.Records[r.divergentIntent]
	if !ok || record.Sha256 == nil {
		return result, wire.Errorf(wire.CodeMalformed, "targetId", "canonical target does not exist")
	}
	return result, nil
}

func (r Reader) ReconciliationRelease(releaseID string, paths ...string) (*Result, error) {
	if _, err := wire.ParseLabel("releaseId", releaseID); err != nil {
		return nil, err
	}
	r.divergentIntent = "intent/releases/" + releaseID + ".json"
	selected := append([]string{r.divergentIntent}, paths...)
	result, err := r.Audit(selected...)
	if err != nil {
		return result, err
	}
	record, ok := result.Records[r.divergentIntent]
	if !ok || record.Sha256 == nil {
		return result, wire.Errorf(wire.CodeMalformed, "releaseId", "canonical release does not exist")
	}
	return result, nil
}

func (r Reader) audit(paths []string, request string, lim limits, checkIntent bool) (*Result, error) {
	if r.Source == nil {
		return nil, wire.Errorf(wire.CodeMalformed, "/", "missing byte source")
	}
	q, err := wire.ParseQueueID("/queueId", r.QueueID.Raw)
	if err != nil {
		return nil, err
	}
	if q != r.QueueID {
		return nil, wire.Errorf(wire.CodeMalformed, "/queueId", "inconsistent parsed identity")
	}
	if _, err := wire.ParsePathText("/primaryWorktree", r.PrimaryWorktree); err != nil {
		return nil, err
	}
	selected := make(map[string]bool)
	for _, p := range paths {
		if _, err := snapshot.PostBound(p); err != nil {
			return nil, err
		}
		if len(selected) >= lim.scan+intent.MaxIntentRootEntries+wire.MaxTicketsPerQueue+wire.MaxReleasesPerQueue {
			return nil, wire.Errorf(wire.CodeLimitExceeded, p, "selected path metadata bound")
		}
		selected[p] = true
	}
	if r.Checkpoint != nil && request == "" && checkIntent && !r.writerCache && !r.intentOnly && !r.unpauseTickets && r.divergentIntent == "" {
		for attempt := 0; attempt < 4; attempt++ {
			result, err := r.auditAttempt(selected, request, lim, checkIntent, r.Checkpoint)
			if err == nil {
				return result, nil
			}
			if wire.CodeOf(err) != wire.CodeSnapshotMoved {
				break
			}
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		result, err := r.auditAttempt(selected, request, lim, checkIntent, nil)
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
			continue
		}
		return result, err
	}
	return nil, wire.Errorf(wire.CodeSnapshotMoved, "/", "ledger moved during all four observations")
}

func (r Reader) auditAttempt(selected map[string]bool, request string, lim limits, checkIntent bool, cp *Checkpoint) (result *Result, err error) {
	if r.physical != nil {
		*r.physical = PhysicalObservation{}
	}
	var native *nativeRead
	switch n := r.Source.(type) {
	case Native:
		native = newNativeRead(n)
	case *Native:
		if n == nil {
			return nil, wire.Errorf(wire.CodeMalformed, "source", "nil native source")
		}
		native = newNativeRead(*n)
	}
	if native != nil {
		r.Source = native
		if r.physical != nil {
			native.physical = newPhysicalReads()
		}
		defer func() {
			cleanup := native.close()
			if cleanup != nil {
				result = nil
				err = wire.Errorf(wire.CodeUnsupportedFilesystem, "read lifetime", "%v; close: %v", err, cleanup)
			}
			if r.physical != nil {
				r.physical.Cleanup = cleanup
				if err == nil && result != nil && result.Mode == ModeFull && !result.Pending && !result.StagingPresent && result.IntentError == nil && result.selectErr == nil && result.intentErr == nil {
					r.physical.Files = native.physical.files
				}
			}
		}()
	}
	before, e := r.capture(lim, cp != nil)
	if e != nil {
		return nil, e
	}
	if r.afterCapture != nil {
		r.afterCapture()
	}
	if r.observedIntent {
		selected = observedSelection(before)
	}
	var bodyErr error
	if cp != nil {
		result, bodyErr = r.walkTail(before, cp, selected, lim)
	} else {
		result, bodyErr = r.walk(before, selected, request, lim, checkIntent)
	}
	if result != nil && r.observedIntent {
		result.selection = selected
	}
	after, e := r.capture(lim, cp != nil)
	if e != nil {
		return nil, e
	}
	if !sameObservation(before, after) {
		return nil, moved("inventory")
	}
	return result, bodyErr
}

func sameObservation(a, b *observation) bool {
	if a.identity != b.identity || len(a.files) != len(b.files) {
		return false
	}
	for p, info := range a.files {
		other, ok := b.files[p]
		if !ok || !os.SameFile(info, other) {
			return false
		}
	}
	return true
}

func optionalRead(s Source, p string, max int) ([]byte, bool, error) {
	raw, err := s.Read(p, max)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(raw) > max {
		return nil, false, wire.Errorf(wire.CodeLimitExceeded, p, "byte source exceeded read bound")
	}
	return raw, true, nil
}

func requiredRead(s Source, p string, max int) ([]byte, error) {
	raw, present, err := optionalRead(s, p, max)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, wire.Errorf(wire.CodeJournalForked, p, "required retained bytes absent")
	}
	if raw == nil {
		raw = []byte{}
	}
	return raw, nil
}

func (r Reader) capture(lim limits, lite bool) (*observation, error) {
	o := &observation{files: map[string]os.FileInfo{}, lite: lite, intentDigests: map[string]wire.Digest{}}
	remaining := lim.scan
	if err := r.scan(o, ".", &remaining, true); err != nil {
		return nil, err
	}
	intentRemaining := intent.MaxIntentRootEntries + wire.MaxTicketsPerQueue + wire.MaxReleasesPerQueue
	if err := r.scan(o, "intent", &intentRemaining, true); err != nil {
		return nil, err
	}
	if _, ok := o.files["RESTORE_INCOMPLETE"]; ok {
		return nil, wire.Errorf(wire.CodeRestoreIncomplete, "RESTORE_INCOMPLETE", "interrupted restore")
	}
	raw, present, err := optionalRead(r.Source, "head.json", wire.MaxJournalHeadBytes)
	if err != nil {
		return nil, err
	}
	o.headRaw = raw
	if present {
		o.head, err = snapshot.DecodeHead(raw)
		if err != nil {
			return nil, err
		}
	}
	var paths []string
	for p := range o.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	inventory := sha256.New()
	var intents []intent.File
	total := 0
	for _, p := range paths {
		info := o.files[p]
		fmt.Fprintf(inventory, "%s\x00%d\x00%d\x00%d\n", p, info.Mode(), info.Size(), info.ModTime().UnixNano())
		if strings.HasPrefix(p, "receipts/") && !info.IsDir() && !isTemp(p) {
			o.receipts = append(o.receipts, strings.TrimPrefix(p, "receipts/"))
		}
		if strings.HasPrefix(p, "intent/") && !info.IsDir() {
			max, err := snapshot.PostBound(p)
			if err != nil {
				return nil, err
			}
			if info.Size() < 1 && !strings.HasPrefix(p, "intent/tickets/") {
				return nil, wire.Errorf(wire.CodeMalformed, p, "empty intent file")
			}
			if info.Size() > int64(max) || info.Size() > int64(wire.MaxIntentTreeBytes-total) {
				return nil, wire.Errorf(wire.CodeLimitExceeded, p, "intent byte bound")
			}
			raw, err := requiredRead(r.Source, p, max)
			if err != nil {
				return nil, err
			}
			if len(raw) == 0 && !strings.HasPrefix(p, "intent/tickets/") {
				return nil, wire.Errorf(wire.CodeMalformed, p, "empty intent file")
			}
			if len(raw) > wire.MaxIntentTreeBytes-total {
				return nil, wire.Errorf(wire.CodeLimitExceeded, p, "intent tree grew beyond bound")
			}
			total += len(raw)
			sum := wire.Sum(raw)
			o.intentDigests[p] = sum
			intents = append(intents, intent.File{Path: strings.TrimPrefix(p, "intent/"), Sha256: sum, Bytes: len(raw)})
		}
	}
	stageFiles, stageErr, err := r.readStage(o)
	o.stageErr = stageErr
	if err != nil {
		return nil, err
	}
	if o.stageErr == nil {
		o.stage, o.stageErr = snapshot.ObserveStage(stageFiles)
	}
	for _, f := range stageFiles {
		sum := wire.Sum(f.Raw)
		o.stageDigests = append(o.stageDigests, stageDigest{f.Name, sum})
		fmt.Fprintf(inventory, "staging/%s\x00%s\n", f.Name, sum)
	}
	o.identity = Identity{HeadSha256: wire.Sum(raw), InventorySha256: wire.Digest(hex.EncodeToString(inventory.Sum(nil))), IntentTreeSha256: intent.DigestOfFiles(intents)}
	return o, nil
}

func isTemp(p string) bool {
	return p == "head.json.tmp" || ((strings.HasPrefix(p, "receipts/") || strings.HasPrefix(p, "effects/")) && strings.Contains(p, ".tmp-"))
}

func (r Reader) scan(o *observation, dir string, remaining *int, optional bool) error {
	max := *remaining
	if dir == "intent" && max > intent.MaxIntentRootEntries {
		max = intent.MaxIntentRootEntries
	}
	if dir == "intent/tickets" && max > wire.MaxTicketsPerQueue {
		max = wire.MaxTicketsPerQueue
	}
	if dir == "intent/releases" && max > wire.MaxReleasesPerQueue {
		max = wire.MaxReleasesPerQueue
	}
	if dir == "staging" && max > snapshot.MaxStageChildren {
		max = snapshot.MaxStageChildren
	}
	listing, err := r.Source.List(dir, max)
	if optional && os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if listing.DirectoryInfo == nil || !listing.DirectoryInfo.IsDir() {
		return wire.Errorf(wire.CodeMalformed, dir, "missing listed directory metadata")
	}
	o.files[dir] = listing.DirectoryInfo
	entries := listing.Entries
	if len(entries) > max {
		return wire.Errorf(wire.CodeLimitExceeded, dir, "directory scan bound")
	}
	*remaining -= len(entries)
	seen := map[string]bool{}
	for _, e := range entries {
		if e.Name == "" || e.Name == "." || e.Name == ".." || strings.ContainsAny(e.Name, "/\\") || seen[e.Name] {
			return wire.Errorf(wire.CodeJournalForked, dir, "duplicate or aliased inventory name")
		}
		seen[e.Name] = true
		p := e.Name
		if dir != "." {
			p = dir + "/" + e.Name
		}
		if e.Info == nil {
			return wire.Errorf(wire.CodeMalformed, p, "missing directory metadata")
		}
		if !e.Info.IsDir() && !e.Info.Mode().IsRegular() {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, p, "only regular files and directories allowed")
		}
		o.files[p] = e.Info
		if e.Info.IsDir() {
			if !allowedDir(p) {
				return wire.Errorf(wire.CodeMalformed, p, "unexpected store directory")
			}
			if p == "worktrees" || (o.lite && (p == "receipts" || p == "requests" || p == "evidence")) {
				continue
			}
			if err := r.scan(o, p, remaining, false); err != nil {
				return err
			}
			continue
		}
		if dir == "staging" {
			if !snapshot.StageName(e.Name) {
				return wire.Errorf(wire.CodeMalformed, p, "unexpected staging child")
			}
			continue
		}
		if isTemp(p) {
			o.staging = true
			continue
		}
		if strings.HasPrefix(p, "receipts/") {
			if _, err := receiptSeq(strings.TrimPrefix(p, "receipts/")); err != nil {
				return err
			}
			continue
		}
		if p == "head.json" || p == "RESTORE_INCOMPLETE" {
			continue
		}
		if strings.HasPrefix(p, "effects/") && (strings.HasSuffix(p, ".boot") || strings.HasSuffix(p, ".ack")) {
			name := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(p, "effects/"), ".boot"), ".ack")
			if _, err := wire.ParseDigest(p, name); err != nil {
				return err
			}
			continue
		}
		if _, err := snapshot.PostBound(p); err != nil {
			return err
		}
	}
	return nil
}

func allowedDir(p string) bool {
	switch p {
	case "staging", "receipts", "requests", "attempts", "effects", "pinned", "evidence", "worktrees", "intent/tickets", "intent/releases":
		return true
	}
	if strings.HasPrefix(p, "requests/") && len(p) == len("requests/")+2 {
		_, err := strconv.ParseUint(strings.TrimPrefix(p, "requests/"), 16, 8)
		return err == nil && strings.ToLower(p) == p
	}
	return false
}

func receiptSeq(name string) (uint64, error) {
	if len(name) != 17 || !strings.HasSuffix(name, ".json") {
		return 0, wire.Errorf(wire.CodeJournalForked, name, "invalid receipt filename")
	}
	n, err := strconv.ParseUint(name[:12], 10, 64)
	canonical, _ := snapshot.ReceiptName(n)
	if err != nil || n == 0 || canonical != name {
		return 0, wire.Errorf(wire.CodeJournalForked, name, "invalid receipt filename")
	}
	return n, nil
}
