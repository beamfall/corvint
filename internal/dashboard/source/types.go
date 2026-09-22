package source

import (
	"io"
	"sync"
	"time"
)

const (
	MaxSourceBytes    uint64 = 16 << 20
	MaxAggregateBytes uint64 = 256 << 20
	MaxPhysicalBytes  uint64 = 1 << 30
	MaxArtifactCount  uint64 = 10_000
	MaxPathBytes             = 4096
)

// Kind is a closed identifier for a dashboard source family.
type Kind string

const (
	KindUnknown          Kind = "unknown"
	KindQueryEnvelope    Kind = "query-envelope"
	KindImpactEnvelope   Kind = "impact-envelope"
	KindLocalTrace       Kind = "LOCAL_TRACE_STORE"
	KindCEM              Kind = "cem"
	KindOCM              Kind = "ocm"
	KindFrontier         Kind = "frontier"
	KindPulseReceipt     Kind = "pulse-receipt"
	KindLiveVerification Kind = "live-verification"
	KindHarnessEvent     Kind = "harness-event"
	KindSpecIndex        Kind = "spec-index"
	KindBeamfallShadow   Kind = "beamfall-shadow"
)

// VerifierID names a closed, versioned verifier contract. This package records
// the identifier; it never invokes a verifier.
type VerifierID string

const (
	VerifierUnknown            VerifierID = "unknown"
	VerifierContextEnvelopeV1  VerifierID = "corvint-context-envelope/1"
	VerifierLocalTraceV1       VerifierID = "go-local-trace-v1"
	VerifierCEMV01             VerifierID = "cem/0.1"
	VerifierCEMV02             VerifierID = "cem/0.2"
	VerifierOCMV01Experimental VerifierID = "ocm/0.1-experimental"
	VerifierFrontierV0         VerifierID = "frontier/0"
	VerifierPulseSnapshotV0    VerifierID = "corvint-pulse-snapshot/0"
	VerifierGoLiveRunV0        VerifierID = "go-live-run/0"
	VerifierHarnessEventV0     VerifierID = "corvint-harness-event/0"
	VerifierSpecIndexV0        VerifierID = "corvint-spec-index/0"
	VerifierFirstRunResultV0   VerifierID = "cem-first-run-result/0"
	VerifierFirstRunResultV01  VerifierID = "cem-first-run-result/0.1"
)

type Validity string

const (
	ValidityInvalid      Validity = "INVALID"
	ValidityStable       Validity = "VALID"
	ValidityNotPresent   Validity = "NOT_PRESENT"
	ValidityInaccessible Validity = "INACCESSIBLE"
	ValidityUnsupported  Validity = "UNSUPPORTED"
)

type IssueCode string

const (
	IssueNone                    IssueCode = ""
	IssueInvalidRegistration     IssueCode = "DASHBOARD_INVALID_ARGUMENT"
	IssueInvalidPath             IssueCode = "DASHBOARD_INVALID_ARGUMENT"
	IssueSourceUnavailable       IssueCode = "SOURCE_NOT_PRESENT"
	IssueSourceUnreadable        IssueCode = "SOURCE_INACCESSIBLE"
	IssueSourceSymlink           IssueCode = "SOURCE_SYMLINK"
	IssueSourceHardLinked        IssueCode = "SOURCE_MULTILINK_UNQUALIFIED"
	IssueSourceNotRegular        IssueCode = "SOURCE_SPECIAL_FILE"
	IssueSourceTooLarge          IssueCode = "SOURCE_OVERSIZED"
	IssueAggregateBudgetExceeded IssueCode = "DASHBOARD_RESOURCE_EXHAUSTED"
	IssueSourceUnstable          IssueCode = "SOURCE_CHANGED_DURING_READ"
	IssueStoreChanged            IssueCode = "STORE_CHANGED"
	IssueTraceStoreBound         IssueCode = "TRACE_STORE_BOUND"
)

// Result is deliberately closed: it cannot carry source contents, paths, or
// operating-system error text.
type Result struct {
	sourceID   string
	Kind       Kind       `json:"kind"`
	VerifierID VerifierID `json:"verifier_id"`
	Validity   Validity   `json:"validity"`
	Bytes      uint64     `json:"bytes"`
	SHA256     *string    `json:"sha256"`
	Issue      IssueCode  `json:"issue_code"`
	Evidence   *Evidence  `json:"-"`
}

type PlatformIdentity struct {
	Kind               string
	Device             uint64
	Inode              uint64
	LinkCount          uint64
	FileIndex          uint64
	VolumeSerialNumber uint64
}

type FileIdentity struct {
	mode               uint32
	modTimeNanoseconds int64
	platform           PlatformIdentity
	size               uint64
}

func (i FileIdentity) Mode() uint32                       { return i.mode }
func (i FileIdentity) ModTimeNanoseconds() int64          { return i.modTimeNanoseconds }
func (i FileIdentity) PlatformIdentity() PlatformIdentity { return i.platform }
func (i FileIdentity) Size() uint64                       { return i.size }

// Evidence is an internal, non-serializable stable-read envelope.
type Evidence struct {
	adapterID         string
	byteCount         uint64
	configuredOrdinal uint64
	contentSHA256     string
	start             time.Time
	end               time.Time
	before            FileIdentity
	after             FileIdentity
	validity          Validity
}

func (e Evidence) AdapterID() string                { return e.adapterID }
func (e Evidence) ByteCount() uint64                { return e.byteCount }
func (e Evidence) ConfiguredOrdinal() uint64        { return e.configuredOrdinal }
func (e Evidence) ContentSHA256() string            { return e.contentSHA256 }
func (e Evidence) Start() time.Time                 { return e.start }
func (e Evidence) End() time.Time                   { return e.end }
func (e Evidence) FileIdentityBefore() FileIdentity { return e.before }
func (e Evidence) FileIdentityAfter() FileIdentity  { return e.after }
func (e Evidence) Validity() Validity               { return e.validity }

// StableContent is an immutable view of the exact bytes admitted by Read.
// Reader returns a fresh in-memory reader; it never reopens the source path.
type StableContent struct {
	state *contentState
}

type contentState struct {
	mu     sync.Mutex
	data   []byte
	digest string
	live   bool
}

func (content StableContent) Len() uint64 {
	if content.state == nil {
		return 0
	}
	content.state.mu.Lock()
	defer content.state.mu.Unlock()
	if !content.state.live {
		return 0
	}
	return uint64(len(content.state.data))
}

func (content StableContent) Reader() io.Reader {
	return &ephemeralReader{state: content.state}
}

func (content StableContent) SHA256() string {
	if content.state == nil {
		return ""
	}
	content.state.mu.Lock()
	defer content.state.mu.Unlock()
	if !content.state.live {
		return ""
	}
	return content.state.digest
}

type ephemeralReader struct {
	state  *contentState
	offset int
}

func (reader *ephemeralReader) Read(destination []byte) (int, error) {
	if reader.state == nil {
		return 0, io.ErrClosedPipe
	}
	reader.state.mu.Lock()
	defer reader.state.mu.Unlock()
	if !reader.state.live {
		return 0, io.ErrClosedPipe
	}
	if reader.offset >= len(reader.state.data) {
		return 0, io.EOF
	}
	count := copy(destination, reader.state.data[reader.offset:])
	reader.offset += count
	return count, nil
}

func newStableContent(data []byte, digest string) (StableContent, func()) {
	state := &contentState{data: data, digest: digest, live: true}
	return StableContent{state: state}, func() {
		state.mu.Lock()
		defer state.mu.Unlock()
		state.live = false
		state.data = nil
		state.digest = ""
	}
}

// StableConsumer receives admitted content after all stability checks pass.
// It cannot mutate the admitted buffer or recover its registered path.
type StableConsumer func(StableContent)

type ObjectFormat string

const (
	ObjectFormatSHA1   ObjectFormat = "sha1"
	ObjectFormatSHA256 ObjectFormat = "sha256"
)

type TraceMemberConsumer func(revision string, content StableContent, evidence Evidence)

type TraceMemberResult struct {
	Revision string
	Result   Result
}

type TraceStoreEvidence struct {
	configuredOrdinal uint64
	start             time.Time
	end               time.Time
	directoryBefore   FileIdentity
	directoryAfter    FileIdentity
	entryCount        uint64
}

func (e TraceStoreEvidence) ConfiguredOrdinal() uint64     { return e.configuredOrdinal }
func (e TraceStoreEvidence) Start() time.Time              { return e.start }
func (e TraceStoreEvidence) End() time.Time                { return e.end }
func (e TraceStoreEvidence) DirectoryBefore() FileIdentity { return e.directoryBefore }
func (e TraceStoreEvidence) DirectoryAfter() FileIdentity  { return e.directoryAfter }
func (e TraceStoreEvidence) EntryCount() uint64            { return e.entryCount }

type TraceStoreResult struct {
	Validity Validity
	Issue    IssueCode
	Limit    uint64
	Members  []TraceMemberResult
	Evidence *TraceStoreEvidence
}

type BudgetCode uint8

const (
	BudgetInvalid BudgetCode = iota
	BudgetOK
	BudgetLogicalExhausted
	BudgetPhysicalExhausted
)

// BudgetResult is a closed, text-free budget decision.
type BudgetResult struct {
	code BudgetCode
}

func (r BudgetResult) Allowed() bool    { return r.code == BudgetOK }
func (r BudgetResult) Code() BudgetCode { return r.code }

type budgetKey struct {
	adapterID         string
	configuredOrdinal uint64
}

// overflowProbeKey scopes the overflow-detection allowance. A singular artifact
// uses its source key alone; a trace-store member adds its entry name and the
// store attempt, so one unstable member cannot spend another member's probes.
type overflowProbeKey struct {
	source       budgetKey
	member       string
	storeAttempt int
}

// Budget is a view of the shared whole-invocation source ledger. Logical
// reservations are keyed by product source identity and are never refunded or
// double charged across acquisition or repository-stability retries. Physical
// bytes are charged independently for every actual read, including rejected
// reads. Overflow probes belong to one whole-scan attempt view.
type Budget struct {
	mu               sync.Mutex
	limit            uint64
	used             uint64
	count            uint64
	physicalLimit    uint64
	physicalUsed     uint64
	overflowProbes   map[overflowProbeKey]uint8
	reservations     map[budgetKey]uint64
	syntheticOrdinal uint64
	invocation       *Budget
}

func NewBudget(limit uint64) *Budget {
	if limit > MaxAggregateBytes {
		limit = MaxAggregateBytes
	}
	return &Budget{
		limit:          limit,
		physicalLimit:  MaxPhysicalBytes,
		reservations:   make(map[budgetKey]uint64),
		overflowProbes: make(map[overflowProbeKey]uint8),
	}
}

// ForWholeScanAttempt returns a fresh overflow-probe allowance backed by the
// same invocation-wide logical and physical byte ledger.
func (b *Budget) ForWholeScanAttempt() *Budget {
	invocation := b.invocationBudget()
	if invocation == nil {
		return nil
	}
	return &Budget{
		overflowProbes: make(map[overflowProbeKey]uint8),
		invocation:     invocation,
	}
}

func (b *Budget) invocationBudget() *Budget {
	if b == nil {
		return nil
	}
	if b.invocation != nil {
		return b.invocation
	}
	return b
}

func (b *Budget) Used() uint64 {
	invocation := b.invocationBudget()
	if invocation == nil {
		return 0
	}
	invocation.mu.Lock()
	defer invocation.mu.Unlock()
	return invocation.used
}

func (b *Budget) PhysicalUsed() uint64 {
	invocation := b.invocationBudget()
	if invocation == nil {
		return 0
	}
	invocation.mu.Lock()
	defer invocation.mu.Unlock()
	return invocation.physicalUsed
}

// Preflight atomically reserves the declared logical size for one configured
// source. Repeating a key with an equal or smaller bound is free; growth only
// charges the delta. A rejected reservation does not partially mutate state.
func (b *Budget) Preflight(adapterID string, configuredOrdinal, declaredBound uint64) BudgetResult {
	invocation := b.invocationBudget()
	if invocation == nil || adapterID == "" || len(adapterID) > maxIdentifierBytes {
		return BudgetResult{code: BudgetInvalid}
	}
	invocation.mu.Lock()
	defer invocation.mu.Unlock()
	if invocation.reservations == nil || invocation.used > invocation.limit {
		return BudgetResult{code: BudgetInvalid}
	}
	key := budgetKey{adapterID: adapterID, configuredOrdinal: configuredOrdinal}
	previous, exists := invocation.reservations[key]
	if exists && declaredBound <= previous {
		return BudgetResult{code: BudgetOK}
	}
	delta := declaredBound
	if exists {
		delta -= previous
	}
	if delta > invocation.limit-invocation.used || (!exists && invocation.count >= MaxArtifactCount) {
		return BudgetResult{code: BudgetLogicalExhausted}
	}
	invocation.used += delta
	if !exists {
		invocation.count++
	}
	invocation.reservations[key] = declaredBound
	return BudgetResult{code: BudgetOK}
}

// ChargePhysical accounts actual bytes already returned by the operating
// system. Callers must stop immediately when it rejects a charge.
func (b *Budget) ChargePhysical(bytes uint64) BudgetResult {
	invocation := b.invocationBudget()
	if invocation == nil {
		return BudgetResult{code: BudgetInvalid}
	}
	invocation.mu.Lock()
	defer invocation.mu.Unlock()
	if invocation.physicalUsed > invocation.physicalLimit {
		return BudgetResult{code: BudgetInvalid}
	}
	if bytes > invocation.physicalLimit-invocation.physicalUsed {
		return BudgetResult{code: BudgetPhysicalExhausted}
	}
	invocation.physicalUsed += bytes
	return BudgetResult{code: BudgetOK}
}

func (b *Budget) reserve(size uint64) bool {
	invocation := b.invocationBudget()
	if invocation == nil {
		return false
	}
	invocation.mu.Lock()
	ordinal := invocation.syntheticOrdinal
	invocation.syntheticOrdinal++
	invocation.mu.Unlock()
	return b.Preflight("internal-compatibility", ordinal, size).Allowed()
}

// chargeBoundedRead reserves no work before I/O. It records up to the
// remaining physical payload and permits exactly one extra detection byte for
// a crossed global or per-source bound. The detection byte is tracked
// separately and cannot be spent as payload.
func (b *Budget) chargeBoundedRead(key overflowProbeKey, bytes, payloadBound uint64) (BudgetResult, bool) {
	invocation := b.invocationBudget()
	if invocation == nil || bytes > payloadBound+1 {
		return BudgetResult{code: BudgetInvalid}, false
	}
	invocation.mu.Lock()
	defer invocation.mu.Unlock()
	if invocation.physicalUsed > invocation.physicalLimit {
		return BudgetResult{code: BudgetInvalid}, false
	}
	remaining := invocation.physicalLimit - invocation.physicalUsed
	overflow := bytes > payloadBound
	payload := bytes
	if overflow {
		payload--
	}
	if payload > remaining {
		return BudgetResult{code: BudgetPhysicalExhausted}, false
	}
	if overflow {
		if b.overflowProbes == nil || b.overflowProbes[key] >= 2 {
			return BudgetResult{code: BudgetPhysicalExhausted}, false
		}
		b.overflowProbes[key]++
	}
	invocation.physicalUsed += payload
	return BudgetResult{code: BudgetOK}, overflow
}

func (b *Budget) boundedReadLimit(key overflowProbeKey, payloadBound uint64) (uint64, BudgetResult) {
	invocation := b.invocationBudget()
	if invocation == nil {
		return 0, BudgetResult{code: BudgetInvalid}
	}
	invocation.mu.Lock()
	defer invocation.mu.Unlock()
	if invocation.physicalUsed > invocation.physicalLimit {
		return 0, BudgetResult{code: BudgetInvalid}
	}
	if b.overflowProbes == nil || b.overflowProbes[key] >= 2 {
		return 0, BudgetResult{code: BudgetPhysicalExhausted}
	}
	remaining := invocation.physicalLimit - invocation.physicalUsed
	if payloadBound > remaining {
		payloadBound = remaining
	}
	return payloadBound + 1, BudgetResult{code: BudgetOK}
}
