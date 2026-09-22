package trace

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	StateAbsent = "absent"
	StateReady  = "ready"
)

var revisionPattern = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

var stagedEvictionPattern = regexp.MustCompile(`^\.evict-([0-9a-f]{40,64})-([0-9a-f]{64})-([0-9a-f]{40,64})\.tmp$`)

var stagedRecordPattern = regexp.MustCompile(`^\.record-([0-9a-f]{40,64})\.tmp$`)

var stagedMigrationPattern = regexp.MustCompile(`^\.migrate-([0-9a-f]{40,64})\.tmp$`)

var errStagedEvictionResidue = errors.New("staged trace eviction residue")

// Revision binds one candidate filename to one immutable repository snapshot.
type Revision struct {
	TreeRevision     string
	Legacy           bool
	TrackedPaths     []string
	AncestryDistance int
}

type revisionPolicy struct {
	treeRevision     string
	legacy           bool
	trackedPaths     map[string]struct{}
	ancestryDistance int
}

// Store owns bounded, stable reads and appends beneath .context-corvint/traces.
// checkStable must revalidate the caller's clean commit/tree snapshot.
type Store struct {
	root        string
	revisions   map[string]revisionPolicy
	checkStable func() error
	hooks       storeHooks
}

type storeHooks struct {
	write                      func(*os.File, []byte) (int, error)
	sync                       func(*os.File) error
	publishSync                func(*os.File) error
	rename                     func(*os.Root, string, string) error
	afterRecoveryTargetCleanup func() error
	beforeCommit               func()
	duringRead                 func()
}

type storeSnapshot struct {
	records              []Record
	fileBytes            map[string][]byte
	totalBytes           int
	totalRows            int
	totalEntries         int
	operationLockPresent bool
}

type retentionCandidate struct {
	name             string
	rows             int
	bytes            int
	unreachable      bool
	hasPassed        bool
	ancestryDistance int
}

type stagedEviction struct {
	original string
	hidden   string
}

type heldDirectory struct {
	repository     *os.Root
	corvintFile    *os.File
	corvintRoot    *os.Root
	traceFile      *os.File
	traceRoot      *os.Root
	leaf           string
	createdCorvint bool
	createdTrace   bool
}

var appendProcessLock sync.Mutex

// NewStore freezes revision authority and tracked paths for one repository snapshot.
func NewStore(root string, revisions map[string]Revision, checkStable func() error) (*Store, error) {
	if checkStable == nil {
		return nil, fmt.Errorf("trace store requires a repository stability check")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve trace root: %w", err)
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve trace root: %w", err)
	}
	policies := make(map[string]revisionPolicy, len(revisions))
	objectIDLength := 0
	for revision, value := range revisions {
		if !exactObjectID(revision, len(revision)) || !exactObjectID(value.TreeRevision, len(revision)) {
			return nil, fmt.Errorf("invalid local trace revision: %s", revision)
		}
		if value.AncestryDistance < 0 {
			return nil, fmt.Errorf("invalid local trace ancestry distance: %d", value.AncestryDistance)
		}
		if objectIDLength == 0 {
			objectIDLength = len(revision)
		}
		if len(revision) != objectIDLength {
			return nil, fmt.Errorf("mixed local trace object formats")
		}
		policies[revision] = revisionPolicy{
			treeRevision:     value.TreeRevision,
			legacy:           value.Legacy,
			trackedPaths:     stringSet(value.TrackedPaths),
			ancestryDistance: value.AncestryDistance,
		}
	}
	return &Store{
		root: absolute, revisions: policies, checkStable: checkStable,
		hooks: storeHooks{
			write:                      func(file *os.File, data []byte) (int, error) { return file.Write(data) },
			sync:                       func(file *os.File) error { return file.Sync() },
			publishSync:                func(file *os.File) error { return file.Sync() },
			rename:                     func(root *os.Root, oldName, newName string) error { return root.Rename(oldName, newName) },
			afterRecoveryTargetCleanup: func() error { return nil },
			beforeCommit:               func() {},
			duringRead:                 func() {},
		},
	}, nil
}

// StorePath returns the Python-compatible path for a revision candidate.
func StorePath(root, revision string) string {
	absolute, err := filepath.Abs(root)
	if err != nil {
		absolute = root
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	return filepath.Join(absolute, ".context-corvint", "traces", revision+".jsonl")
}

// RecoverInterruptedAppend resolves retention residue for the explicit record
// path before strict candidate discovery. Read paths never call it. Validation
// runs only when recovery would mutate the trace store.
func RecoverInterruptedAppend(root string, checkStable func() error, validateBeforeMutation func() error) (resultErr error) {
	store, err := NewStore(root, map[string]Revision{}, checkStable)
	if err != nil {
		return err
	}
	present, err := store.hasInterruptedAppendResidue()
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	if err := store.checkStable(); err != nil {
		return fmt.Errorf("repository changed before trace recovery: %w", err)
	}
	if validateBeforeMutation == nil {
		return fmt.Errorf("trace recovery requires pre-mutation validation")
	}
	if err := validateBeforeMutation(); err != nil {
		return err
	}
	appendProcessLock.Lock()
	defer appendProcessLock.Unlock()
	directory, err := openTraceDirectory(store.root, false)
	if errors.Is(err, os.ErrNotExist) {
		return store.checkStable()
	}
	if err != nil {
		return err
	}
	defer directory.close()
	operationLock, lockCreated, err := directory.openOperationLock()
	if err != nil {
		return err
	}
	defer operationLock.Close()
	if err := lockDescriptor(operationLock); err != nil {
		return fmt.Errorf("cannot lock local trace store: %w", err)
	}
	defer unlockDescriptor(operationLock)
	if lockCreated {
		defer func() {
			if resultErr != nil {
				resultErr = errors.Join(resultErr, directory.removeCreatedMember(".trace-operation.lock", operationLock))
			}
		}()
	}
	if err := directory.confirmMember(".trace-operation.lock", operationLock); err != nil {
		return err
	}
	if err := store.checkStable(); err != nil {
		return fmt.Errorf("repository changed before trace recovery: %w", err)
	}
	if err := recoverStagedEvictions(directory, store.hooks.rename, store.hooks.afterRecoveryTargetCleanup); err != nil {
		return err
	}
	if err := store.checkStable(); err != nil {
		return fmt.Errorf("repository changed during trace recovery: %w", err)
	}
	return nil
}

func (store *Store) hasInterruptedAppendResidue() (bool, error) {
	directory, err := openTraceDirectory(store.root, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer directory.close()
	staged, _, _, err := stagedEvictionResidue(directory)
	if err != nil {
		return false, err
	}
	temporaries, err := stagedRecordResidue(directory)
	if err != nil {
		return false, err
	}
	if err := directory.confirm(); err != nil {
		return false, markDrift(err)
	}
	return len(staged) != 0 || len(temporaries) != 0, nil
}

// CandidateRevisions returns the bounded, pinned revision filenames currently in the store.
func CandidateRevisions(root string) ([]string, error) {
	return candidateRevisions(root, false)
}

// CandidateRevisionsForAppend permits only the single persistent operation-lock
// overage that an interrupted exact-cap append can leave after recovery.
func CandidateRevisionsForAppend(root string) ([]string, error) {
	return candidateRevisions(root, true)
}

func candidateRevisions(root string, appendMode bool) ([]string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve trace root: %w", err)
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve trace root: %w", err)
	}
	directory, err := openTraceDirectory(absolute, false)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer directory.close()
	var names []string
	if appendMode {
		var entries int
		names, entries, err = directory.namesForAppend()
		if err == nil && entries > MaxTraceFiles {
			operationLockPresent, lockErr := directory.hasOperationLock()
			if lockErr != nil {
				return nil, lockErr
			}
			if entries != MaxTraceFiles+1 || !operationLockPresent {
				return nil, fmt.Errorf("local trace store exceeds %d directory entries", MaxTraceFiles)
			}
		}
	} else {
		names, _, err = directory.names()
	}
	if err != nil {
		return nil, err
	}
	revisions := make([]string, len(names))
	for index, name := range names {
		revision := strings.TrimSuffix(name, ".jsonl")
		if !revisionPattern.MatchString(revision) {
			return nil, fmt.Errorf("invalid local trace revision filename: %s", name)
		}
		revisions[index] = revision
	}
	if err := directory.confirm(); err != nil {
		return nil, markDrift(err)
	}
	return revisions, nil
}

// Read validates the complete candidate set and returns records sorted by TraceID.
func (store *Store) Read() ([]Record, string, error) {
	if err := store.checkStable(); err != nil {
		return nil, "", fmt.Errorf("repository changed while reading local traces: %w", err)
	}
	directory, err := openTraceDirectory(store.root, false)
	if errors.Is(err, os.ErrNotExist) {
		// An absent store has nothing to bracket: the opening check proved the
		// repository matched the index before the store was looked at, and the
		// empty answer does not depend on the repository, so no closing
		// observation is spawned here. Append keeps its bracket.
		return []Record{}, StateAbsent, nil
	}
	if err != nil {
		return nil, "", err
	}
	defer directory.close()
	snapshot, err := store.readPinned(directory)
	if err != nil {
		return nil, "", err
	}
	if err := directory.confirm(); err != nil {
		return nil, "", markDrift(err)
	}
	if err := store.checkStable(); err != nil {
		return nil, "", fmt.Errorf("repository changed while reading local traces: %w", err)
	}
	return snapshot.records, StateReady, nil
}

// Append atomically publishes one canonical row. It returns false when the row already exists.
func (store *Store) Append(record Record) (written bool, resultErr error) {
	policy, ok := store.revisions[record.Revision]
	if !ok {
		return false, fmt.Errorf("local trace store contains unreachable revision: %s", record.Revision)
	}
	if policy.legacy {
		return false, fmt.Errorf("cannot append to a legacy trace revision: %s", record.Revision)
	}
	if err := validateAppendRecord(record, policy.trackedPaths); err != nil {
		return false, err
	}
	row, err := Encode(record)
	if err != nil {
		return false, err
	}
	preflight, candidates, _, err := store.readSnapshotForAppend()
	if err != nil {
		if !errors.Is(err, errStagedEvictionResidue) {
			return false, err
		}
	} else {
		if duplicateRecord(preflight.records, record.TraceID) {
			return false, nil
		}
		if _, err := planRetention(preflight, candidates, record.Revision+".jsonl", len(row)); err != nil {
			return false, err
		}
	}
	appendProcessLock.Lock()
	defer appendProcessLock.Unlock()
	directory, err := openTraceDirectory(store.root, true)
	if err != nil {
		return false, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, directory.cleanupCreated())
		}
		directory.close()
	}()
	operationLock, lockCreated, err := directory.openOperationLock()
	if err != nil {
		return false, err
	}
	defer operationLock.Close()
	if err := lockDescriptor(operationLock); err != nil {
		return false, fmt.Errorf("cannot lock local trace store: %w", err)
	}
	defer unlockDescriptor(operationLock)
	if lockCreated {
		defer func() {
			if resultErr != nil {
				resultErr = errors.Join(resultErr, directory.removeCreatedMember(".trace-operation.lock", operationLock))
			}
		}()
	}
	if err := directory.confirmMember(".trace-operation.lock", operationLock); err != nil {
		return false, err
	}
	if err := store.checkStable(); err != nil {
		return false, fmt.Errorf("repository changed before trace recovery: %w", err)
	}
	if err := recoverStagedEvictions(directory, store.hooks.rename, store.hooks.afterRecoveryTargetCleanup); err != nil {
		return false, err
	}
	current, candidates, err := store.readPinnedForAppend(directory)
	if err != nil {
		return false, err
	}
	if duplicateRecord(current.records, record.TraceID) {
		return false, nil
	}
	name := record.Revision + ".jsonl"
	evictions, err := planRetention(current, candidates, name, len(row))
	if err != nil {
		return false, err
	}
	if err := store.checkStable(); err != nil {
		return false, fmt.Errorf("repository changed before trace append: %w", err)
	}
	if len(evictions) == 0 {
		if err := validateAppendBounds(current, name, len(row)); err != nil {
			return false, err
		}
	}

	expected := append(append([]byte(nil), current.fileBytes[name]...), row...)
	temporaryName := ".record-" + record.Revision + ".tmp"
	temporary, err := directory.openRegular(temporaryName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false, fmt.Errorf("cannot stage local trace: %w", err)
	}
	temporaryCreated := true
	defer func() {
		if temporaryCreated {
			resultErr = errors.Join(resultErr, directory.removeCreatedMember(temporaryName, temporary))
		}
		temporary.Close()
	}()
	if err := store.writeAndSync(temporary, expected); err != nil {
		return false, err
	}
	if err := confirmStagedTrace(directory, temporaryName, temporary, expected); err != nil {
		return false, err
	}
	store.hooks.beforeCommit()
	if err := directory.confirmMember(".trace-operation.lock", operationLock); err != nil {
		return false, err
	}
	if err := directory.confirm(); err != nil {
		return false, err
	}
	latest, _, err := store.readPinnedMode(directory, true, temporaryName)
	if err != nil {
		return false, err
	}
	if !sameSnapshot(current, latest) {
		return false, fmt.Errorf("local trace candidate set changed during append")
	}
	if err := store.checkStable(); err != nil {
		return false, fmt.Errorf("repository changed before trace append: %w", err)
	}
	if err := confirmStagedTrace(directory, temporaryName, temporary, expected); err != nil {
		return false, err
	}
	var stagedEvictions []stagedEviction
	retentionPublished := false
	defer func() {
		if len(stagedEvictions) != 0 && !retentionPublished {
			resultErr = errors.Join(resultErr, restoreStagedEvictions(directory, store.hooks.rename, stagedEvictions))
		}
	}()
	if len(evictions) != 0 {
		if err := directory.confirm(); err != nil {
			return false, err
		}
		stagedEvictions, err = stageTraceEvictions(directory, store.hooks.rename, record.Revision, record.TraceID, evictions)
		if err != nil {
			return false, err
		}
	}
	if err := store.hooks.rename(directory.traceRoot, temporaryName, name); err != nil {
		return false, fmt.Errorf("cannot publish local trace: %w", err)
	}
	temporaryCreated = false
	retentionPublished = true
	if err := store.hooks.publishSync(directory.traceFile); err != nil {
		return true, fmt.Errorf("cannot sync local trace directory: %w", err)
	}
	if err := clearStagedEvictions(directory, stagedEvictions); err != nil {
		return true, err
	}
	return true, nil
}

func (store *Store) readSnapshotForAppend() (storeSnapshot, []retentionCandidate, string, error) {
	if err := store.checkStable(); err != nil {
		return storeSnapshot{}, nil, "", fmt.Errorf("repository changed while reading local traces: %w", err)
	}
	directory, err := openTraceDirectory(store.root, false)
	if errors.Is(err, os.ErrNotExist) {
		if stableErr := store.checkStable(); stableErr != nil {
			return storeSnapshot{}, nil, "", fmt.Errorf("repository changed while reading local traces: %w", stableErr)
		}
		return storeSnapshot{records: []Record{}, fileBytes: map[string][]byte{}}, nil, StateAbsent, nil
	}
	if err != nil {
		return storeSnapshot{}, nil, "", err
	}
	defer directory.close()
	snapshot, candidates, err := store.readPinnedForAppend(directory)
	if err != nil {
		return storeSnapshot{}, nil, "", err
	}
	if err := directory.confirm(); err != nil {
		return storeSnapshot{}, nil, "", err
	}
	if err := store.checkStable(); err != nil {
		return storeSnapshot{}, nil, "", fmt.Errorf("repository changed while reading local traces: %w", err)
	}
	return snapshot, candidates, StateReady, nil
}

func duplicateRecord(records []Record, traceID string) bool {
	for _, record := range records {
		if record.TraceID == traceID {
			return true
		}
	}
	return false
}

func validateAppendBounds(snapshot storeSnapshot, target string, rowBytes int) error {
	_, targetExists := snapshot.fileBytes[target]
	if appendFits(snapshot.totalRows, snapshot.totalBytes, snapshot.totalEntries, targetExists, snapshot.operationLockPresent, rowBytes) {
		return nil
	}
	return retentionFailure(snapshot, target, rowBytes)
}

func planRetention(snapshot storeSnapshot, candidates []retentionCandidate, target string, rowBytes int) ([]retentionCandidate, error) {
	_, targetExists := snapshot.fileBytes[target]
	if appendFits(snapshot.totalRows, snapshot.totalBytes, snapshot.totalEntries, targetExists, snapshot.operationLockPresent, rowBytes) {
		for _, candidate := range candidates {
			if candidate.unreachable {
				return nil, fmt.Errorf("local trace store contains unreachable revision: %s", strings.TrimSuffix(candidate.name, ".jsonl"))
			}
		}
		return nil, nil
	}
	eligible := make([]retentionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.name == target {
			continue
		}
		eligible = append(eligible, candidate)
	}
	sort.Slice(eligible, func(left, right int) bool {
		leftClass := retentionClass(eligible[left])
		rightClass := retentionClass(eligible[right])
		if leftClass != rightClass {
			return leftClass < rightClass
		}
		if leftClass != 0 && eligible[left].ancestryDistance != eligible[right].ancestryDistance {
			return eligible[left].ancestryDistance > eligible[right].ancestryDistance
		}
		return eligible[left].name < eligible[right].name
	})
	rows := snapshot.totalRows
	bytes := snapshot.totalBytes
	entries := snapshot.totalEntries
	evictions := make([]retentionCandidate, 0, len(eligible))
	for _, candidate := range eligible {
		evictions = append(evictions, candidate)
		rows -= candidate.rows
		bytes -= candidate.bytes
		entries--
		if appendFits(rows, bytes, entries, targetExists, snapshot.operationLockPresent, rowBytes) {
			return evictions, nil
		}
	}
	return nil, retentionFailure(snapshot, target, rowBytes)
}

func retentionClass(candidate retentionCandidate) int {
	if candidate.unreachable {
		return 0
	}
	if !candidate.hasPassed {
		return 1
	}
	return 2
}

func appendFits(rows, bytes, entries int, targetExists, operationLockPresent bool, rowBytes int) bool {
	if rows+1 > MaxTraces || bytes+rowBytes > MaxTraceStoreBytes {
		return false
	}
	if !operationLockPresent {
		entries++
	}
	if !targetExists {
		entries++
	}
	return entries <= MaxTraceFiles
}

func retentionFailure(snapshot storeSnapshot, target string, rowBytes int) error {
	limit := fmt.Sprintf("exceeds %d files", MaxTraceFiles)
	if snapshot.totalRows+1 > MaxTraces {
		limit = fmt.Sprintf("exceeds %d rows", MaxTraces)
	} else if snapshot.totalBytes+rowBytes > MaxTraceStoreBytes {
		limit = fmt.Sprintf("exceeds %d bytes", MaxTraceStoreBytes)
	}
	return fmt.Errorf("local trace store %s and cannot reclaim without evicting append target %s; record at a newer revision or remove the target trace file", limit, strings.TrimSuffix(target, ".jsonl"))
}

func stageTraceEvictions(directory *heldDirectory, rename func(*os.Root, string, string) error, targetRevision, traceID string, candidates []retentionCandidate) ([]stagedEviction, error) {
	staged := make([]stagedEviction, 0, len(candidates))
	for _, candidate := range candidates {
		hidden := stagedEvictionName(targetRevision, traceID, candidate.name)
		if _, err := directory.traceRoot.Lstat(hidden); err == nil {
			return staged, fmt.Errorf("staged trace eviction collision: %s", hidden)
		} else if !errors.Is(err, os.ErrNotExist) {
			return staged, fmt.Errorf("cannot inspect staged trace eviction %s: %w", hidden, err)
		}
	}
	for _, candidate := range candidates {
		hidden := stagedEvictionName(targetRevision, traceID, candidate.name)
		file, err := directory.openRegular(candidate.name, os.O_RDONLY, 0)
		if err != nil {
			return staged, err
		}
		if err := directory.confirmMember(candidate.name, file); err != nil {
			file.Close()
			return staged, err
		}
		if err := rename(directory.traceRoot, candidate.name, hidden); err != nil {
			file.Close()
			return staged, fmt.Errorf("cannot stage local trace eviction %s: %w", candidate.name, err)
		}
		staged = append(staged, stagedEviction{original: candidate.name, hidden: hidden})
		if err := directory.confirmMember(hidden, file); err != nil {
			file.Close()
			return staged, err
		}
		file.Close()
	}
	if err := directory.traceFile.Sync(); err != nil {
		return staged, fmt.Errorf("cannot sync staged local trace evictions: %w", err)
	}
	return staged, nil
}

func stagedEvictionName(targetRevision, traceID, candidateName string) string {
	return ".evict-" + targetRevision + "-" + traceID + "-" + strings.TrimSuffix(candidateName, ".jsonl") + ".tmp"
}

func recoverStagedEvictions(directory *heldDirectory, rename func(*os.Root, string, string) error, afterTargetCleanup func() error) error {
	staged, targetRevision, traceID, err := stagedEvictionResidue(directory)
	if err != nil {
		return err
	}
	temporaries, err := stagedRecordResidue(directory)
	if err != nil {
		return err
	}
	if len(staged) == 0 {
		if len(temporaries) == 0 {
			return nil
		}
		if len(temporaries) != 1 {
			return fmt.Errorf("mixed staged local trace targets")
		}
		return directory.removeRegular(temporaries[0])
	}
	temporaryName := ".record-" + targetRevision + ".tmp"
	for _, name := range temporaries {
		if name != temporaryName {
			return fmt.Errorf("mixed staged local trace targets")
		}
	}
	temporaryPresent := len(temporaries) == 1
	published, err := traceFileContainsID(directory, targetRevision+".jsonl", traceID)
	if err != nil {
		return err
	}
	if published {
		if temporaryPresent {
			return fmt.Errorf("staged local trace recovery contains both published and temporary targets: %s", targetRevision)
		}
		return clearStagedEvictions(directory, staged)
	}
	if temporaryPresent {
		if err := directory.removeRegular(temporaryName); err != nil {
			return err
		}
		if err := afterTargetCleanup(); err != nil {
			return fmt.Errorf("interrupted after staged local trace cleanup: %w", err)
		}
	}
	if err := restoreStagedEvictions(directory, rename, staged); err != nil {
		return err
	}
	return nil
}

func stagedRecordResidue(directory *heldDirectory) ([]string, error) {
	handle, err := directory.traceRoot.Open(".")
	if err != nil {
		return nil, fmt.Errorf("cannot inspect staged local traces: %w", err)
	}
	defer handle.Close()
	entries, err := handle.ReadDir(MaxTraceFiles + 3)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("cannot inspect staged local traces: %w", err)
	}
	if len(entries) > MaxTraceFiles+2 {
		return nil, fmt.Errorf("local trace store exceeds %d directory entries", MaxTraceFiles)
	}
	var names []string
	for _, entry := range entries {
		if !isStagedRecordName(entry.Name()) {
			continue
		}
		match := stagedRecordPattern.FindStringSubmatch(entry.Name())
		if match == nil || !exactObjectID(match[1], len(match[1])) {
			return nil, fmt.Errorf("invalid staged local trace residue: %s", entry.Name())
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

func stagedEvictionResidue(directory *heldDirectory) ([]stagedEviction, string, string, error) {
	handle, err := directory.traceRoot.Open(".")
	if err != nil {
		return nil, "", "", fmt.Errorf("cannot inspect staged trace evictions: %w", err)
	}
	defer handle.Close()
	entries, err := handle.ReadDir(MaxTraceFiles + 3)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, "", "", fmt.Errorf("cannot inspect staged trace evictions: %w", err)
	}
	if len(entries) > MaxTraceFiles+2 {
		return nil, "", "", fmt.Errorf("local trace store exceeds %d directory entries", MaxTraceFiles)
	}
	names := make([]string, 0)
	for _, entry := range entries {
		if isStagedEvictionName(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	staged := make([]stagedEviction, 0, len(names))
	var targetRevision string
	var traceID string
	for _, name := range names {
		parts := stagedEvictionPattern.FindStringSubmatch(name)
		if parts == nil || !exactObjectID(parts[1], len(parts[1])) || !exactObjectID(parts[3], len(parts[1])) {
			return nil, "", "", fmt.Errorf("invalid staged trace eviction residue: %s", name)
		}
		if targetRevision == "" {
			targetRevision, traceID = parts[1], parts[2]
		}
		if parts[1] != targetRevision || parts[2] != traceID {
			return nil, "", "", fmt.Errorf("mixed staged trace eviction residue")
		}
		staged = append(staged, stagedEviction{original: parts[3] + ".jsonl", hidden: name})
	}
	return staged, targetRevision, traceID, nil
}

func traceFileContainsID(directory *heldDirectory, name, traceID string) (bool, error) {
	present, err := directory.memberExists(name)
	if err != nil || !present {
		return false, err
	}
	data, err := directory.readRegular(name, MaxTraceStoreBytes)
	if err != nil {
		return false, err
	}
	revision := strings.TrimSuffix(name, ".jsonl")
	lines, err := splitRows(data, MaxTraces, MaxTraceStoreBytes, name)
	if err != nil {
		return false, err
	}
	found := false
	for number, line := range lines {
		record, err := decodeRecord(line)
		if err != nil {
			var syntaxError *recordJSONError
			if errors.As(err, &syntaxError) {
				return false, fmt.Errorf("invalid local trace JSON in %s at line %d", name, number+1)
			}
			return false, err
		}
		if err := validateUnreachableRecord(record, revision); err != nil {
			return false, err
		}
		found = found || record.TraceID == traceID
	}
	return found, nil
}

func restoreStagedEvictions(directory *heldDirectory, rename func(*os.Root, string, string) error, staged []stagedEviction) error {
	var restoreErr error
	for index := len(staged) - 1; index >= 0; index-- {
		eviction := staged[index]
		if _, err := directory.traceRoot.Lstat(eviction.original); err == nil {
			restoreErr = errors.Join(restoreErr, fmt.Errorf("local trace eviction rollback collision: %s", eviction.original))
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			restoreErr = errors.Join(restoreErr, fmt.Errorf("cannot inspect local trace eviction rollback %s: %w", eviction.original, err))
			continue
		}
		if err := rename(directory.traceRoot, eviction.hidden, eviction.original); err != nil {
			restoreErr = errors.Join(restoreErr, fmt.Errorf("cannot restore local trace eviction %s: %w", eviction.original, err))
		}
	}
	if err := directory.traceFile.Sync(); err != nil {
		restoreErr = errors.Join(restoreErr, fmt.Errorf("cannot sync restored local trace evictions: %w", err))
	}
	return restoreErr
}

func clearStagedEvictions(directory *heldDirectory, staged []stagedEviction) error {
	for _, eviction := range staged {
		file, err := directory.openRegular(eviction.hidden, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		if err := directory.confirmMember(eviction.hidden, file); err != nil {
			file.Close()
			return err
		}
		if err := directory.traceRoot.Remove(eviction.hidden); err != nil {
			file.Close()
			return fmt.Errorf("cannot clear staged local trace eviction %s: %w", eviction.hidden, err)
		}
		file.Close()
	}
	if len(staged) != 0 {
		if err := directory.traceFile.Sync(); err != nil {
			return fmt.Errorf("cannot sync local trace retention: %w", err)
		}
	}
	return nil
}

func sameSnapshot(left, right storeSnapshot) bool {
	if left.totalRows != right.totalRows || left.totalBytes != right.totalBytes || left.totalEntries != right.totalEntries || left.operationLockPresent != right.operationLockPresent || len(left.fileBytes) != len(right.fileBytes) {
		return false
	}
	for name, data := range left.fileBytes {
		other, ok := right.fileBytes[name]
		if !ok || !bytes.Equal(data, other) {
			return false
		}
	}
	return true
}

func (store *Store) writeAndSync(file *os.File, data []byte) error {
	for len(data) != 0 {
		written, err := store.hooks.write(file, data)
		if err != nil {
			return fmt.Errorf("cannot write local trace: %w", err)
		}
		if written < 1 || written > len(data) {
			return fmt.Errorf("cannot write local trace: short write")
		}
		data = data[written:]
	}
	if err := store.hooks.sync(file); err != nil {
		return fmt.Errorf("cannot sync local trace: %w", err)
	}
	return nil
}

func confirmStagedTrace(directory *heldDirectory, name string, file *os.File, expected []byte) error {
	actual, err := readStableDescriptor(directory, name, file, MaxTraceStoreBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("staged local trace changed before publication: %s", name)
	}
	return nil
}

func (store *Store) readPinned(directory *heldDirectory) (storeSnapshot, error) {
	snapshot, _, err := store.readPinnedMode(directory, false)
	return snapshot, err
}

func (store *Store) readPinnedForAppend(directory *heldDirectory) (storeSnapshot, []retentionCandidate, error) {
	return store.readPinnedMode(directory, true)
}

func (store *Store) readPinnedMode(directory *heldDirectory, tolerateUnreachable bool, ignored ...string) (storeSnapshot, []retentionCandidate, error) {
	var names []string
	var entries int
	var err error
	if tolerateUnreachable || len(ignored) != 0 {
		names, entries, err = directory.namesForAppend(ignored...)
	} else {
		names, entries, err = directory.names()
	}
	if err != nil {
		return storeSnapshot{}, nil, err
	}
	if entries > MaxTraceFiles && !tolerateUnreachable {
		return storeSnapshot{}, nil, fmt.Errorf("local trace store exceeds %d directory entries", MaxTraceFiles)
	}
	operationLockPresent, err := directory.hasOperationLock()
	if err != nil {
		return storeSnapshot{}, nil, err
	}
	snapshot := storeSnapshot{
		fileBytes:            make(map[string][]byte, len(names)),
		totalEntries:         entries,
		operationLockPresent: operationLockPresent,
	}
	candidates := make([]retentionCandidate, 0, len(names))
	canonical := make(map[string]map[string]struct{})
	legacy := make(map[string]map[string]struct{})
	ids := make(map[string]struct{})
	for _, name := range names {
		revision := strings.TrimSuffix(name, ".jsonl")
		if !revisionPattern.MatchString(revision) {
			return storeSnapshot{}, nil, fmt.Errorf("invalid local trace revision filename: %s", name)
		}
		policy, ok := store.revisions[revision]
		if !ok && !tolerateUnreachable {
			return storeSnapshot{}, nil, fmt.Errorf("local trace store contains unreachable revision: %s", revision)
		}
		data, err := directory.readRegular(name, MaxTraceStoreBytes-snapshot.totalBytes)
		if err != nil {
			return storeSnapshot{}, nil, err
		}
		snapshot.fileBytes[name] = data
		snapshot.totalBytes += len(data)
		remainingRows := MaxTraces - snapshot.totalRows
		lines, err := splitRows(data, remainingRows, MaxTraceStoreBytes-(snapshot.totalBytes-len(data)), name)
		if err != nil {
			return storeSnapshot{}, nil, err
		}
		candidate := retentionCandidate{name: name, rows: len(lines), bytes: len(data), unreachable: !ok}
		if !ok {
			for number, line := range lines {
				record, err := decodeRecord(line)
				if err != nil {
					var syntaxError *recordJSONError
					if errors.As(err, &syntaxError) {
						return storeSnapshot{}, nil, fmt.Errorf("invalid local trace JSON in %s at line %d", name, number+1)
					}
					return storeSnapshot{}, nil, err
				}
				if err := validateUnreachableRecord(record, revision); err != nil {
					return storeSnapshot{}, nil, err
				}
				if _, duplicate := ids[record.TraceID]; duplicate {
					return storeSnapshot{}, nil, fmt.Errorf("local trace store contains duplicate trace ids")
				}
				ids[record.TraceID] = struct{}{}
			}
			snapshot.totalRows += len(lines)
			candidates = append(candidates, candidate)
			continue
		}
		candidate.ancestryDistance = policy.ancestryDistance
		records := make([]Record, 0, len(lines))
		for number, line := range lines {
			record, err := decodeRecord(line)
			if err != nil {
				var syntaxError *recordJSONError
				if errors.As(err, &syntaxError) {
					return storeSnapshot{}, nil, fmt.Errorf("invalid local trace JSON in %s at line %d", name, number+1)
				}
				return storeSnapshot{}, nil, err
			}
			record, err = normalizeStoredRecord(record, revision, policy.trackedPaths)
			if err != nil {
				return storeSnapshot{}, nil, err
			}
			candidate.hasPassed = candidate.hasPassed || record.Outcome == "passed"
			if _, duplicate := ids[record.TraceID]; duplicate {
				return storeSnapshot{}, nil, fmt.Errorf("local trace store contains duplicate trace ids")
			}
			ids[record.TraceID] = struct{}{}
			semantic, err := semanticID(record)
			if err != nil {
				return storeSnapshot{}, nil, err
			}
			semantics := canonical
			if policy.legacy {
				semantics = legacy
			}
			if semantics[policy.treeRevision] == nil {
				semantics[policy.treeRevision] = make(map[string]struct{})
			}
			semantics[policy.treeRevision][semantic] = struct{}{}
			records = append(records, record)
		}
		snapshot.totalRows += len(records)
		snapshot.records = append(snapshot.records, records...)
		candidates = append(candidates, candidate)
	}
	store.hooks.duringRead()
	afterNames, _, err := directory.namesForMode(tolerateUnreachable, ignored...)
	if err != nil || !equalStrings(names, afterNames) {
		return storeSnapshot{}, nil, markDrift(fmt.Errorf("local trace candidate set changed while reading"))
	}
	for name, expected := range snapshot.fileBytes {
		actual, err := directory.readRegular(name, len(expected))
		if err != nil || !bytes.Equal(actual, expected) {
			return storeSnapshot{}, nil, markDrift(fmt.Errorf("local trace file changed while reading: %s", name))
		}
	}
	finalNames, _, err := directory.namesForMode(tolerateUnreachable, ignored...)
	if err != nil || !equalStrings(names, finalNames) {
		return storeSnapshot{}, nil, markDrift(fmt.Errorf("local trace candidate set changed while reading"))
	}
	for tree, legacySet := range legacy {
		for semantic := range legacySet {
			if _, overlap := canonical[tree][semantic]; overlap {
				return storeSnapshot{}, nil, fmt.Errorf("local trace store contains an incomplete legacy migration")
			}
		}
	}
	sort.Slice(snapshot.records, func(left, right int) bool { return snapshot.records[left].TraceID < snapshot.records[right].TraceID })
	return snapshot, candidates, nil
}

func openTraceDirectory(root string, create bool) (*heldDirectory, error) {
	return openPrivateDirectory(root, "traces", create)
}

func openPrivateDirectory(root, leaf string, create bool) (*heldDirectory, error) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("unsafe local trace store: %w", err)
	}
	directory := &heldDirectory{repository: repository, leaf: leaf}
	corvintFile, corvintRoot, createdCorvint, err := openPrivateChild(repository, ".context-corvint", create)
	if err != nil {
		directory.close()
		return nil, err
	}
	directory.corvintFile, directory.corvintRoot, directory.createdCorvint = corvintFile, corvintRoot, createdCorvint
	traceFile, traceRoot, createdTrace, err := openPrivateChild(corvintRoot, leaf, create)
	if err != nil {
		_ = directory.cleanupCreated()
		directory.close()
		return nil, err
	}
	directory.traceFile, directory.traceRoot, directory.createdTrace = traceFile, traceRoot, createdTrace
	if err := directory.confirm(); err != nil {
		_ = directory.cleanupCreated()
		directory.close()
		return nil, err
	}
	return directory, nil
}

func openPrivateChild(parent *os.Root, name string, create bool) (*os.File, *os.Root, bool, error) {
	created := false
	if create {
		err := parent.Mkdir(name, 0o700)
		if err == nil {
			created = true
		} else if !errors.Is(err, os.ErrExist) {
			return nil, nil, false, fmt.Errorf("unsafe local trace store: %w", err)
		}
	}
	fail := func(err error) (*os.File, *os.Root, bool, error) {
		if created {
			_ = parent.Remove(name)
		}
		return nil, nil, false, err
	}
	info, err := parent.Lstat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fail(os.ErrNotExist)
		}
		return fail(fmt.Errorf("unsafe local trace store: %w", err))
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fail(fmt.Errorf("unsafe local trace store: %s is not a private directory", name))
	}
	file, root, err := openNoFollowDirectory(parent, name)
	if err != nil {
		return fail(fmt.Errorf("unsafe local trace store: %w", err))
	}
	descriptor, err := file.Stat()
	if err != nil || !descriptor.IsDir() || !os.SameFile(info, descriptor) {
		file.Close()
		root.Close()
		return fail(fmt.Errorf("unsafe local trace store: directory changed: %s", name))
	}
	if create {
		if err := file.Chmod(0o700); err != nil {
			file.Close()
			root.Close()
			return fail(fmt.Errorf("unsafe local trace store: %w", err))
		}
	}
	if created {
		parentFile, openErr := parent.Open(".")
		if openErr != nil {
			file.Close()
			root.Close()
			return fail(fmt.Errorf("unsafe local trace store: %w", openErr))
		}
		syncErr := parentFile.Sync()
		parentFile.Close()
		if syncErr != nil {
			file.Close()
			root.Close()
			return fail(fmt.Errorf("unsafe local trace store: %w", syncErr))
		}
	}
	return file, root, created, nil
}

func (directory *heldDirectory) openOperationLock() (*os.File, bool, error) {
	file, err := directory.openRegular(".trace-operation.lock", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		return file, true, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return nil, false, err
	}
	file, err = directory.openRegular(".trace-operation.lock", os.O_RDWR, 0o600)
	return file, false, err
}

func (directory *heldDirectory) removeCreatedMember(name string, file *os.File) error {
	if err := directory.confirmMember(name, file); err != nil {
		return err
	}
	if err := directory.traceRoot.Remove(name); err != nil {
		return fmt.Errorf("cannot remove staged local trace %s: %w", name, err)
	}
	return directory.traceFile.Sync()
}

func (directory *heldDirectory) cleanupCreated() error {
	var cleanupErr error
	if directory.createdTrace && directory.corvintRoot != nil {
		if directory.traceRoot != nil {
			directory.traceRoot.Close()
			directory.traceRoot = nil
		}
		if directory.traceFile != nil {
			directory.traceFile.Close()
			directory.traceFile = nil
		}
		if err := directory.corvintRoot.Remove(directory.leaf); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cannot remove empty local trace directory: %w", err))
		} else {
			directory.createdTrace = false
			if directory.corvintFile != nil {
				cleanupErr = errors.Join(cleanupErr, directory.corvintFile.Sync())
			}
		}
	}
	if directory.createdCorvint && directory.repository != nil && !directory.createdTrace {
		if directory.corvintRoot != nil {
			directory.corvintRoot.Close()
			directory.corvintRoot = nil
		}
		if directory.corvintFile != nil {
			directory.corvintFile.Close()
			directory.corvintFile = nil
		}
		if err := directory.repository.Remove(".context-corvint"); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cannot remove empty local trace root: %w", err))
		} else {
			directory.createdCorvint = false
			if repositoryFile, err := directory.repository.Open("."); err == nil {
				cleanupErr = errors.Join(cleanupErr, repositoryFile.Sync())
				repositoryFile.Close()
			}
		}
	}
	return cleanupErr
}

func (directory *heldDirectory) names() ([]string, int, error) {
	handle, err := directory.traceRoot.Open(".")
	if err != nil {
		return nil, 0, fmt.Errorf("cannot inspect local trace store: %w", err)
	}
	defer handle.Close()
	entries, err := handle.ReadDir(MaxTraceFiles + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, fmt.Errorf("cannot inspect local trace store: %w", err)
	}
	if len(entries) > MaxTraceFiles {
		return nil, len(entries), fmt.Errorf("local trace store exceeds %d directory entries", MaxTraceFiles)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if isStagedEvictionName(entry.Name()) || isStagedRecordName(entry.Name()) {
			return nil, len(entries), fmt.Errorf("%w: %s", errStagedEvictionResidue, entry.Name())
		}
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, len(entries), nil
}

func (directory *heldDirectory) namesForMigration() ([]string, int, string, error) {
	handle, err := directory.traceRoot.Open(".")
	if err != nil {
		return nil, 0, "", fmt.Errorf("cannot inspect local trace store: %w", err)
	}
	defer handle.Close()
	entries, err := handle.ReadDir(MaxTraceFiles + 4)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, "", fmt.Errorf("cannot inspect local trace store: %w", err)
	}
	if len(entries) > MaxTraceFiles+3 {
		return nil, len(entries), "", fmt.Errorf("local trace store exceeds %d directory entries", MaxTraceFiles)
	}
	names := make([]string, 0, len(entries))
	boundedEntries := len(entries)
	operationLocks := 0
	migrationTemporary := ""
	for _, entry := range entries {
		name := entry.Name()
		if isStagedEvictionName(name) || isStagedRecordName(name) {
			return nil, boundedEntries, "", fmt.Errorf("%w: %s", errStagedEvictionResidue, name)
		}
		switch {
		case name == ".trace-operation.lock":
			operationLocks++
			boundedEntries--
		case stagedMigrationPattern.MatchString(name):
			if migrationTemporary != "" {
				return nil, boundedEntries, "", fmt.Errorf("local trace store exceeds %d directory entries", MaxTraceFiles)
			}
			migrationTemporary = name
		case strings.HasSuffix(name, ".jsonl"):
			names = append(names, name)
		}
	}
	if operationLocks > 1 || boundedEntries > MaxTraceFiles+1 {
		return nil, boundedEntries, "", fmt.Errorf("local trace store exceeds %d directory entries", MaxTraceFiles)
	}
	sort.Strings(names)
	return names, boundedEntries, migrationTemporary, nil
}

func (directory *heldDirectory) namesForMode(appendMode bool, ignored ...string) ([]string, int, error) {
	if appendMode || len(ignored) != 0 {
		return directory.namesForAppend(ignored...)
	}
	return directory.names()
}

func (directory *heldDirectory) namesForAppend(ignored ...string) ([]string, int, error) {
	handle, err := directory.traceRoot.Open(".")
	if err != nil {
		return nil, 0, fmt.Errorf("cannot inspect local trace store: %w", err)
	}
	defer handle.Close()
	entries, err := handle.ReadDir(MaxTraceFiles + len(ignored) + 2)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, fmt.Errorf("cannot inspect local trace store: %w", err)
	}
	ignoredSet := stringSet(ignored)
	names := make([]string, 0, len(entries))
	count := 0
	for _, entry := range entries {
		if _, ok := ignoredSet[entry.Name()]; ok {
			continue
		}
		if isStagedEvictionName(entry.Name()) || isStagedRecordName(entry.Name()) {
			return nil, count, fmt.Errorf("%w: %s", errStagedEvictionResidue, entry.Name())
		}
		count++
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			names = append(names, entry.Name())
		}
	}
	if count > MaxTraceFiles+1 {
		return nil, count, fmt.Errorf("local trace store exceeds %d directory entries", MaxTraceFiles)
	}
	sort.Strings(names)
	return names, count, nil
}

func isStagedEvictionName(name string) bool {
	return strings.HasPrefix(name, ".evict-") && strings.HasSuffix(name, ".tmp")
}

func isStagedRecordName(name string) bool {
	return strings.HasPrefix(name, ".record-") && strings.HasSuffix(name, ".tmp")
}

func (directory *heldDirectory) hasOperationLock() (bool, error) {
	return directory.memberExists(".trace-operation.lock")
}

func (directory *heldDirectory) memberExists(name string) (bool, error) {
	_, err := directory.traceRoot.Lstat(name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("cannot inspect local trace member %s: %w", name, err)
}

func (directory *heldDirectory) removeRegular(name string) error {
	file, err := directory.openRegular(name, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := directory.traceRoot.Remove(name); err != nil {
		return fmt.Errorf("cannot remove staged local trace %s: %w", name, err)
	}
	if err := directory.traceFile.Sync(); err != nil {
		return fmt.Errorf("cannot sync removed staged local trace %s: %w", name, err)
	}
	return nil
}

func (directory *heldDirectory) openRegular(name string, flags int, mode os.FileMode) (*os.File, error) {
	// os.Root resolves a symlink that stays inside the root even with O_NOFOLLOW,
	// so an existing leaf must be regular before the open and the same file after
	// it; otherwise a writable open would create or re-permission the target.
	leaf, err := directory.traceRoot.Lstat(name)
	if flags&os.O_CREATE != 0 && errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("unsafe local trace store: %w", err)
	}
	if leaf != nil && !leaf.Mode().IsRegular() {
		return nil, fmt.Errorf("unsafe local trace file: %s", name)
	}
	file, err := openNoFollowMember(directory.traceRoot, name, flags, mode)
	if err != nil {
		return nil, fmt.Errorf("unsafe local trace store: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || descriptorLinks(info) != 1 || (leaf != nil && !os.SameFile(leaf, info)) {
		file.Close()
		return nil, fmt.Errorf("unsafe local trace file: %s", name)
	}
	if flags&(os.O_CREATE|os.O_WRONLY|os.O_RDWR) != 0 {
		if err := file.Chmod(0o600); err != nil {
			file.Close()
			return nil, fmt.Errorf("unsafe local trace store: %w", err)
		}
	}
	if err := directory.confirmMember(name, file); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func (directory *heldDirectory) readRegular(name string, maximum int) ([]byte, error) {
	file, err := directory.openRegular(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readStableDescriptor(directory, name, file, maximum)
}

func readStableDescriptor(directory *heldDirectory, name string, file *os.File, maximum int) ([]byte, error) {
	before, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("cannot read local trace store: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("cannot read local trace store: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read local trace store: %w", err)
	}
	if len(data) > maximum {
		return nil, fmt.Errorf("local trace store exceeds %d bytes", maximum)
	}
	after, err := file.Stat()
	if err != nil || !sameMetadata(before, after) || int64(len(data)) != after.Size() {
		return nil, markDrift(fmt.Errorf("local trace file changed while reading: %s", name))
	}
	if err := directory.confirmMember(name, file); err != nil {
		return nil, markDrift(err)
	}
	return data, nil
}

func (directory *heldDirectory) confirmMember(name string, file *os.File) error {
	descriptor, err := file.Stat()
	if err != nil || !descriptor.Mode().IsRegular() || descriptorLinks(descriptor) != 1 {
		return fmt.Errorf("unsafe local trace file: %s", name)
	}
	pathname, err := directory.traceRoot.Lstat(name)
	if err != nil || pathname.Mode()&os.ModeSymlink != 0 || !pathname.Mode().IsRegular() || descriptorLinks(pathname) != 1 || !os.SameFile(descriptor, pathname) {
		return fmt.Errorf("local trace pathname changed while pinned: %s", name)
	}
	return nil
}

func (directory *heldDirectory) confirm() error {
	if err := confirmDirectoryBinding(directory.repository, ".context-corvint", directory.corvintFile); err != nil {
		return err
	}
	return confirmDirectoryBinding(directory.corvintRoot, directory.leaf, directory.traceFile)
}

func confirmDirectoryBinding(parent *os.Root, name string, file *os.File) error {
	descriptor, err := file.Stat()
	if err != nil || !descriptor.IsDir() {
		return fmt.Errorf("local trace directory changed while pinned: %s", name)
	}
	pathname, err := parent.Lstat(name)
	if err != nil || pathname.Mode()&os.ModeSymlink != 0 || !pathname.IsDir() || !os.SameFile(descriptor, pathname) {
		return fmt.Errorf("local trace directory changed while pinned: %s", name)
	}
	return nil
}

func (directory *heldDirectory) close() {
	if directory.traceRoot != nil {
		directory.traceRoot.Close()
	}
	if directory.traceFile != nil {
		directory.traceFile.Close()
	}
	if directory.corvintRoot != nil {
		directory.corvintRoot.Close()
	}
	if directory.corvintFile != nil {
		directory.corvintFile.Close()
	}
	if directory.repository != nil {
		directory.repository.Close()
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func mapKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	return result
}

func exactObjectID(value string, length int) bool {
	if length != 40 && length != 64 {
		return false
	}
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
