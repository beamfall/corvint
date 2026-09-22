package repository

import (
	"context"
	"os"
	"sync"

	dashboardauthority "github.com/Beamfall/corvint/internal/dashboard/authority"
)

const Profile = "corvint-dashboard-git-authority/0"

type FailureCode string

const (
	FailureRepositoryUnavailable FailureCode = "DASHBOARD_REPOSITORY_UNAVAILABLE"
	FailureRepositoryChanged     FailureCode = "DASHBOARD_REPOSITORY_CHANGED"
)

type FailureReason string

const (
	ReasonUnavailable           FailureReason = "REPOSITORY_UNAVAILABLE"
	ReasonUnsupportedAlternates FailureReason = "UNSUPPORTED_OBJECT_ALTERNATES"
	ReasonObjectUnavailable     FailureReason = "REPOSITORY_OBJECT_UNAVAILABLE"
	ReasonChanged               FailureReason = "REPOSITORY_CHANGED"
)

// Failure is deliberately closed. It contains no path, command, environment,
// executable identity, Git output, or operating-system error text.
type Failure struct {
	Code   FailureCode
	Reason FailureReason
}

func (failure *Failure) Error() string {
	if failure == nil {
		return ""
	}
	return string(failure.Code)
}

type fileIdentity struct {
	mode               uint32
	modTimeNanoseconds int64
	size               uint64
	platformKind       uint8
	device             uint64
	inode              uint64
	linkCount          uint64
	fileIndex          uint64
	volumeSerial       uint64
}

// directoryIdentity is the path re-identification projection for directories:
// modification time, size, and link count are excluded (decision 0175).
type directoryIdentity struct {
	mode         uint32
	platformKind uint8
	device       uint64
	inode        uint64
	owner        uint64
	group        uint64
	fileIndex    uint64
	volumeSerial uint64
}

type executableEvidence struct {
	identity fileIdentity
	bytes    uint64
	digest   [32]byte
}

type stableFileEvidence struct {
	identity fileIdentity
	bytes    uint64
	digest   [32]byte
}

type directoryBinding struct {
	path     string
	file     *os.File
	root     *os.Root
	identity fileIdentity
}

type layout struct {
	worktree             directoryBinding
	gitDir               directoryBinding
	common               directoryBinding
	objects              directoryBinding
	gitEntry             stableFileEvidence
	pointer              stableFileEvidence
	commonFile           stableFileEvidence
	reciprocal           stableFileEvidence
	configCount          uint8
	configs              [3]stableFileEvidence
	commonWorktreeConfig bool
	gitWorktreeConfig    bool
}

type layoutProof struct {
	worktree, gitEntry, gitDir, common, objects fileIdentity
	pointer, commonFile, reciprocal             stableFileEvidence
	configCount                                 uint8
	configs                                     [3]stableFileEvidence
	commonWorktreeConfig                        bool
	gitWorktreeConfig                           bool
}

type probeState struct {
	snapshot dashboardauthority.Snapshot
	layout   layoutProof
}

// Authority owns one resolved executable, one reciprocal repository binding,
// and one 30-second scan lifetime. Calls are serialized so Finish cannot race a
// member qualification and no capability can outlive the final probe.
type Authority struct {
	mu sync.Mutex

	life      context.Context
	cancel    context.CancelFunc
	budget    *Budget
	ownBudget bool

	executable       string
	executableFile   *os.File
	executableBefore executableEvidence
	environment      []string
	root             string
	layout           layout
	initial          probeState
	snapshot         dashboardauthority.Snapshot

	finished        bool
	objectFailed    bool
	closed          bool
	containmentOkay bool
	drifted         bool
	objectChecks    map[string]objectCheck
}

var _ dashboardauthority.Authority = (*Authority)(nil)

// Budget is the cumulative, non-refundable authority ledger shared by both
// whole-scan attempts. Its deadline begins at construction and never resets.
type Budget struct {
	mu sync.Mutex

	context context.Context
	cancel  context.CancelFunc
	closed  bool

	children    uint64
	stdoutBytes uint64
	pathKeys    map[string]struct{}
	objectKeys  map[string]string
	catInput    uint64
	catOutput   uint64
}

func NewBudget(parent context.Context) *Budget {
	if parent == nil {
		parent = context.Background()
	}
	budgetContext, cancel := context.WithTimeout(parent, wholeDeadline)
	return &Budget{
		context: budgetContext, cancel: cancel,
		pathKeys: make(map[string]struct{}), objectKeys: make(map[string]string),
	}
}

func (budget *Budget) Close() {
	if budget == nil {
		return
	}
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.closed {
		return
	}
	budget.closed = true
	budget.cancel()
	budget.pathKeys = nil
	budget.objectKeys = nil
}

func (budget *Budget) usable() bool {
	if budget == nil {
		return false
	}
	budget.mu.Lock()
	defer budget.mu.Unlock()
	return !budget.closed && budget.context.Err() == nil
}

func (budget *Budget) reserveChild() bool {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.closed || budget.context.Err() != nil || budget.children >= maxChildren {
		return false
	}
	budget.children++
	return true
}

func (budget *Budget) addStdout(observed uint64) bool {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.closed || budget.stdoutBytes > maxScanStdoutBytes || observed > maxScanStdoutBytes-budget.stdoutBytes {
		return false
	}
	budget.stdoutBytes += observed
	return true
}

func (budget *Budget) reservePath(key string) bool {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.closed {
		return false
	}
	if _, exists := budget.pathKeys[key]; exists {
		return true
	}
	if len(budget.pathKeys) >= maxSemanticResolutions {
		return false
	}
	budget.pathKeys[key] = struct{}{}
	return true
}

type objectReservation uint8

const (
	objectReserved objectReservation = iota
	objectTypeConflict
	objectExhausted
)

func (budget *Budget) reserveObject(objectID, expectedType string) objectReservation {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.closed {
		return objectExhausted
	}
	if existing, exists := budget.objectKeys[objectID]; exists {
		if existing != expectedType {
			return objectTypeConflict
		}
		return objectReserved
	}
	if len(budget.objectKeys) >= maxSemanticResolutions {
		return objectExhausted
	}
	budget.objectKeys[objectID] = expectedType
	return objectReserved
}

func (budget *Budget) addCatInput(observed uint64) bool {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.closed || budget.catInput > maxCatTotalBytes || observed > maxCatTotalBytes-budget.catInput {
		return false
	}
	budget.catInput += observed
	return true
}

func (budget *Budget) addCatOutput(observed uint64) bool {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.closed || budget.catOutput > maxCatTotalBytes || observed > maxCatTotalBytes-budget.catOutput {
		return false
	}
	budget.catOutput += observed
	return true
}
