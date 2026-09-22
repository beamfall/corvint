package trace

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// MigrationAuthority is the immutable repository evidence used to classify trace files.
type MigrationAuthority struct {
	CommitRevision string
	TreeRevision   string
	ObjectFormat   string
	DirtyPaths     []string
	ProfileID      string
	Commits        map[string]Revision
	Trees          map[string][]string
	TrackedPaths   func(string) ([]string, error)
}

// MigrationEntry is one legacy tree file and its canonical commit replacement.
type MigrationEntry struct {
	TreeRevision   string `json:"tree_revision"`
	CommitRevision string `json:"commit_revision"`
	SourceSHA256   string `json:"source_sha256"`
	TargetSHA256   string `json:"target_sha256"`
	RowCount       int    `json:"row_count"`
	sourceBytes    []byte
	targetBytes    []byte
}

type migrationCandidate struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	SHA256   string `json:"sha256"`
	RowCount int    `json:"row_count"`
	source   []byte
}

// MigrationPlan is a fully validated, byte-bound trace migration plan.
type MigrationPlan struct {
	Authority  MigrationAuthority
	Digest     string
	Entries    []MigrationEntry
	Candidates []migrationCandidate
}

// MigrationResult is the Python-compatible public command result.
type MigrationResult struct {
	Mutates             bool
	Mode                string
	PlanDigest          string
	CommitRevision      string
	TreeRevision        string
	CandidateTraceFiles int
	LegacyTraceFiles    int
	TraceRows           int
	Entries             []MigrationEntry
}

// PlanMigration validates the entire trace store without writing it.
func PlanMigration(root string, authority MigrationAuthority, checkStable func() error) (MigrationPlan, error) {
	if checkStable == nil {
		return MigrationPlan{}, fmt.Errorf("trace migration requires a repository stability check")
	}
	if err := checkStable(); err != nil {
		return MigrationPlan{}, fmt.Errorf("repository changed while planning trace migration")
	}
	directory, err := openTraceDirectory(root, false)
	if errors.Is(err, os.ErrNotExist) {
		plan, digestErr := emptyMigrationPlan(authority)
		if digestErr != nil {
			return MigrationPlan{}, digestErr
		}
		if stableErr := checkStable(); stableErr != nil {
			return MigrationPlan{}, fmt.Errorf("repository changed while planning trace migration")
		}
		return plan, nil
	}
	if err != nil {
		return MigrationPlan{}, err
	}
	defer directory.close()
	return planMigrationPinned(root, directory, authority, checkStable)
}

func emptyMigrationPlan(authority MigrationAuthority) (MigrationPlan, error) {
	plan := MigrationPlan{Authority: authority, Entries: []MigrationEntry{}, Candidates: []migrationCandidate{}}
	digest, err := migrationDigest(plan)
	plan.Digest = digest
	return plan, err
}

func planMigrationPinned(root string, directory *heldDirectory, authority MigrationAuthority, checkStable func() error) (MigrationPlan, error) {
	if err := directory.confirm(); err != nil {
		return MigrationPlan{}, err
	}
	names, entries, migrationTemporary, err := directory.namesForMigration()
	if err != nil {
		return MigrationPlan{}, err
	}
	quarantine, quarantineErr := openPrivateDirectory(root, "legacy-traces", false)
	if quarantineErr != nil && !errors.Is(quarantineErr, os.ErrNotExist) {
		return MigrationPlan{}, quarantineErr
	}
	if quarantine != nil {
		defer quarantine.close()
	}
	plan := MigrationPlan{Authority: authority, Entries: []MigrationEntry{}, Candidates: []migrationCandidate{}}
	totalBytes, totalRows := 0, 0
	for _, name := range names {
		revision := strings.TrimSuffix(name, ".jsonl")
		if !exactObjectID(revision, objectIDLength(authority.ObjectFormat)) {
			return MigrationPlan{}, fmt.Errorf("local trace store contains unreachable revision: %s", revision)
		}
		policy, canonical := authority.Commits[revision]
		kind, commit := "canonical", revision
		if !canonical {
			commits := authority.Trees[revision]
			if len(commits) == 0 {
				return MigrationPlan{}, fmt.Errorf("local trace store contains unreachable revision: %s", revision)
			}
			if len(commits) != 1 {
				return MigrationPlan{}, fmt.Errorf("legacy trace tree maps to %d reachable commits: %s", len(commits), revision)
			}
			commit, kind = commits[0], "legacy"
			policy = authority.Commits[commit]
		}
		var data []byte
		if canonical {
			data, err = readPublished(directory, name, ".migrate-"+revision+".tmp")
		} else {
			data, err = directory.readRegular(name, MaxTraceStoreBytes)
		}
		if err != nil {
			return MigrationPlan{}, err
		}
		tracked := policy.TrackedPaths
		if tracked == nil && authority.TrackedPaths != nil {
			tracked, err = authority.TrackedPaths(policy.TreeRevision)
			if err != nil {
				return MigrationPlan{}, err
			}
		}
		rowRecords, err := DecodeStore(data, revision, tracked)
		if err != nil {
			return MigrationPlan{}, err
		}
		totalBytes += len(data)
		if totalBytes > MaxTraceStoreBytes {
			return MigrationPlan{}, fmt.Errorf("local trace store exceeds %d bytes", MaxTraceStoreBytes)
		}
		totalRows += len(rowRecords)
		if totalRows > MaxTraces {
			return MigrationPlan{}, fmt.Errorf("local trace store exceeds %d rows", MaxTraces)
		}
		candidate := migrationCandidate{Name: name, Kind: kind, SHA256: sha256Hex(data), RowCount: len(rowRecords), source: data}
		plan.Candidates = append(plan.Candidates, candidate)
		if kind == "canonical" {
			continue
		}
		target, err := TransformLegacy(data, revision, commit, tracked)
		if err != nil {
			return MigrationPlan{}, err
		}
		if _, err := DecodeStore(target, commit, tracked); err != nil {
			return MigrationPlan{}, err
		}
		entry := MigrationEntry{
			TreeRevision: revision, CommitRevision: commit, SourceSHA256: sha256Hex(data),
			TargetSHA256: sha256Hex(target), RowCount: len(rowRecords), sourceBytes: data, targetBytes: target,
		}
		if err := confirmMigrationCollision(directory, quarantine, entry); err != nil {
			return MigrationPlan{}, err
		}
		plan.Entries = append(plan.Entries, entry)
	}
	if err := enforceMigrationEntryBound(plan, directory, names, entries, migrationTemporary); err != nil {
		return MigrationPlan{}, err
	}
	if err := directory.confirm(); err != nil {
		return MigrationPlan{}, err
	}
	if quarantine != nil {
		if err := quarantine.confirm(); err != nil {
			return MigrationPlan{}, err
		}
	}
	if err := checkStable(); err != nil {
		return MigrationPlan{}, fmt.Errorf("repository changed while planning trace migration")
	}
	digest, err := migrationDigest(plan)
	if err != nil {
		return MigrationPlan{}, err
	}
	plan.Digest = digest
	return plan, nil
}

func confirmMigrationCollision(directory, quarantine *heldDirectory, entry MigrationEntry) error {
	targetName := entry.CommitRevision + ".jsonl"
	target, err := readOptionalPublished(directory, targetName, ".migrate-"+entry.CommitRevision+".tmp")
	if err != nil {
		return err
	}
	if target != nil && !bytes.Equal(target, entry.targetBytes) {
		return fmt.Errorf("canonical trace collision: %s", targetName)
	}
	if target == nil {
		staged, err := readOptionalRegular(directory, ".migrate-"+entry.CommitRevision+".tmp")
		if err != nil {
			return err
		}
		if staged != nil && !bytes.Equal(staged, entry.targetBytes) {
			return fmt.Errorf("staged trace collision: .migrate-%s.tmp", entry.CommitRevision)
		}
	}
	if quarantine == nil {
		return nil
	}
	legacyName := entry.TreeRevision + ".jsonl"
	preserved, err := readOptionalPublished(quarantine, legacyName, ".migrate-"+entry.TreeRevision+".tmp")
	if err != nil {
		return err
	}
	if preserved != nil && !bytes.Equal(preserved, entry.sourceBytes) {
		return fmt.Errorf("legacy trace quarantine collision: %s", legacyName)
	}
	if preserved == nil {
		staged, err := readOptionalRegular(quarantine, ".migrate-"+entry.TreeRevision+".tmp")
		if err != nil {
			return err
		}
		if staged != nil && !bytes.Equal(staged, entry.sourceBytes) {
			return fmt.Errorf("staged trace collision: .migrate-%s.tmp", entry.TreeRevision)
		}
	}
	return nil
}

func migrationDigest(plan MigrationPlan) (string, error) {
	candidates := make([]any, 0, len(plan.Candidates))
	for _, candidate := range plan.Candidates {
		candidates = append(candidates, map[string]any{
			"name": candidate.Name, "kind": candidate.Kind, "sha256": candidate.SHA256, "row_count": candidate.RowCount,
		})
	}
	entries := make([]any, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		entries = append(entries, map[string]any{
			"tree_revision": entry.TreeRevision, "commit_revision": entry.CommitRevision,
			"source_sha256": entry.SourceSHA256, "target_sha256": entry.TargetSHA256, "row_count": entry.RowCount,
		})
	}
	payload := map[string]any{
		"schema_version": 1,
		"repository": map[string]any{
			"commit_revision": plan.Authority.CommitRevision, "tree_revision": plan.Authority.TreeRevision,
			"object_format": plan.Authority.ObjectFormat, "dirty_paths": plan.Authority.DirtyPaths,
			"profile_id": plan.Authority.ProfileID,
		},
		"candidates": candidates, "entries": entries,
	}
	encoded, err := contextindex.CanonicalJSON(payload)
	if err != nil {
		return "", fmt.Errorf("cannot encode trace migration plan: %w", err)
	}
	return sha256Hex(encoded), nil
}

// ApplyMigration validates the digest before mutation, locks, replans, and publishes verified files.
func ApplyMigration(root string, authority MigrationAuthority, digest string, checkStable func() error) (MigrationPlan, error) {
	preflight, err := PlanMigration(root, authority, checkStable)
	if err != nil {
		return MigrationPlan{}, err
	}
	if preflight.Digest != digest {
		return MigrationPlan{}, fmt.Errorf("trace migration plan drifted after dry run")
	}
	appendProcessLock.Lock()
	defer appendProcessLock.Unlock()
	directory, err := openTraceDirectory(root, true)
	if err != nil {
		return MigrationPlan{}, err
	}
	defer directory.close()
	operationLock, err := directory.openRegular(".trace-operation.lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return MigrationPlan{}, err
	}
	defer operationLock.Close()
	if err := lockDescriptor(operationLock); err != nil {
		return MigrationPlan{}, fmt.Errorf("cannot lock local trace store: %w", err)
	}
	defer unlockDescriptor(operationLock)
	if err := directory.confirmMember(".trace-operation.lock", operationLock); err != nil {
		return MigrationPlan{}, err
	}
	plan, err := planMigrationPinned(root, directory, authority, checkStable)
	if err != nil {
		return MigrationPlan{}, err
	}
	if plan.Digest != digest {
		return MigrationPlan{}, fmt.Errorf("trace migration plan drifted after dry run")
	}
	if err := confirmPlanCandidates(plan, directory, nil, false); err != nil {
		return MigrationPlan{}, err
	}
	quarantine, err := openPrivateDirectory(root, "legacy-traces", true)
	if err != nil {
		return MigrationPlan{}, err
	}
	defer quarantine.close()
	for _, entry := range plan.Entries {
		if err := confirmMigrationState(directory, operationLock, checkStable); err != nil {
			return MigrationPlan{}, err
		}
		if err := requireBytes(directory, entry.TreeRevision+".jsonl", entry.sourceBytes); err != nil {
			return MigrationPlan{}, fmt.Errorf("legacy trace drifted before staging: %s.jsonl", entry.TreeRevision)
		}
		if err := stagePrivateFile(directory, entry.CommitRevision+".jsonl", ".migrate-"+entry.CommitRevision+".tmp", entry.targetBytes); err != nil {
			return MigrationPlan{}, err
		}
		if err := confirmPlanCandidates(plan, directory, nil, true); err != nil {
			return MigrationPlan{}, err
		}
	}
	for _, entry := range plan.Entries {
		if err := confirmMigrationState(directory, operationLock, checkStable); err != nil {
			return MigrationPlan{}, err
		}
		if err := quarantine.confirm(); err != nil {
			return MigrationPlan{}, err
		}
		if err := confirmPlanCandidates(plan, directory, nil, true); err != nil {
			return MigrationPlan{}, err
		}
		if err := requireBytes(directory, entry.TreeRevision+".jsonl", entry.sourceBytes); err != nil {
			return MigrationPlan{}, fmt.Errorf("legacy trace drifted before quarantine: %s.jsonl", entry.TreeRevision)
		}
		if err := stagePrivateFile(quarantine, entry.TreeRevision+".jsonl", ".migrate-"+entry.TreeRevision+".tmp", entry.sourceBytes); err != nil {
			return MigrationPlan{}, err
		}
	}
	removed := make(map[string]struct{}, len(plan.Entries))
	for _, entry := range plan.Entries {
		if err := confirmMigrationState(directory, operationLock, checkStable); err != nil {
			return MigrationPlan{}, err
		}
		if err := quarantine.confirm(); err != nil {
			return MigrationPlan{}, err
		}
		if err := confirmPlanCandidates(plan, directory, removed, true); err != nil {
			return MigrationPlan{}, err
		}
		if err := requireBytes(directory, entry.TreeRevision+".jsonl", entry.sourceBytes); err != nil {
			return MigrationPlan{}, fmt.Errorf("trace migration verification drift: %s.jsonl", entry.TreeRevision)
		}
		if err := requireBytes(directory, entry.CommitRevision+".jsonl", entry.targetBytes); err != nil {
			return MigrationPlan{}, fmt.Errorf("trace migration verification drift: %s.jsonl", entry.TreeRevision)
		}
		if err := requireBytes(quarantine, entry.TreeRevision+".jsonl", entry.sourceBytes); err != nil {
			return MigrationPlan{}, fmt.Errorf("trace migration verification drift: %s.jsonl", entry.TreeRevision)
		}
		if err := quarantine.confirm(); err != nil {
			return MigrationPlan{}, err
		}
		if err := directory.traceRoot.Remove(entry.TreeRevision + ".jsonl"); err != nil {
			return MigrationPlan{}, fmt.Errorf("cannot quarantine legacy trace: %w", err)
		}
		if err := directory.traceFile.Sync(); err != nil {
			return MigrationPlan{}, fmt.Errorf("cannot quarantine legacy trace: %w", err)
		}
		removed[entry.TreeRevision+".jsonl"] = struct{}{}
		if err := confirmMigrationState(directory, operationLock, checkStable); err != nil {
			return MigrationPlan{}, err
		}
	}
	return plan, nil
}

func confirmPlanCandidates(plan MigrationPlan, directory *heldDirectory, removed map[string]struct{}, allowTargets bool) error {
	expected := make(map[string]migrationCandidate, len(plan.Candidates))
	allowed := make(map[string]struct{}, len(plan.Candidates)+len(plan.Entries))
	for _, candidate := range plan.Candidates {
		if _, wasRemoved := removed[candidate.Name]; wasRemoved {
			continue
		}
		expected[candidate.Name] = candidate
		allowed[candidate.Name] = struct{}{}
	}
	if allowTargets {
		for _, entry := range plan.Entries {
			allowed[entry.CommitRevision+".jsonl"] = struct{}{}
		}
	}
	names, entries, migrationTemporary, err := directory.namesForMigration()
	if err != nil {
		return err
	}
	if err := enforceMigrationEntryBound(plan, directory, names, entries, migrationTemporary); err != nil {
		return err
	}
	if len(names) < len(expected) || len(names) > len(allowed) {
		return fmt.Errorf("trace migration candidate set changed after planning")
	}
	for _, name := range names {
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("trace migration candidate set changed after planning")
		}
	}
	for name, candidate := range expected {
		var data []byte
		if candidate.Kind == "canonical" {
			data, err = readPublished(directory, name, ".migrate-"+strings.TrimSuffix(name, ".jsonl")+".tmp")
		} else {
			data, err = directory.readRegular(name, MaxTraceStoreBytes)
		}
		if err != nil || sha256Hex(data) != candidate.SHA256 {
			return fmt.Errorf("trace migration candidate drifted: %s", name)
		}
	}
	return nil
}

func enforceMigrationEntryBound(plan MigrationPlan, directory *heldDirectory, names []string, entries int, migrationTemporary string) error {
	candidates := make(map[string]struct{}, len(names))
	for _, name := range names {
		candidates[name] = struct{}{}
	}
	missingTargets := 0
	for _, entry := range plan.Entries {
		if _, exists := candidates[entry.CommitRevision+".jsonl"]; !exists {
			missingTargets++
		}
	}
	required := entries + missingTargets
	if required <= MaxTraceFiles {
		return nil
	}
	if required == MaxTraceFiles+1 && qualifiedMigrationTemporary(plan, directory, migrationTemporary) {
		return nil
	}
	return fmt.Errorf("trace migration staging exceeds %d directory entries", MaxTraceFiles)
}

func qualifiedMigrationTemporary(plan MigrationPlan, directory *heldDirectory, name string) bool {
	match := stagedMigrationPattern.FindStringSubmatch(name)
	if len(match) != 2 || !exactObjectID(match[1], objectIDLength(plan.Authority.ObjectFormat)) {
		return false
	}
	targetName := match[1] + ".jsonl"
	for _, candidate := range plan.Candidates {
		if candidate.Kind != "canonical" || candidate.Name != targetName {
			continue
		}
		data, err := readLinkedPair(directory, targetName, name)
		return err == nil && bytes.Equal(data, candidate.source)
	}
	for _, entry := range plan.Entries {
		if entry.CommitRevision != match[1] {
			continue
		}
		if _, err := directory.traceRoot.Lstat(targetName); !errors.Is(err, os.ErrNotExist) {
			return false
		}
		data, info, err := readRegularLinks(directory, name, 1)
		return err == nil && info.Mode().Perm() == 0o600 && bytes.Equal(data, entry.targetBytes)
	}
	return false
}

func confirmMigrationState(directory *heldDirectory, operationLock *os.File, checkStable func() error) error {
	if err := checkStable(); err != nil {
		return fmt.Errorf("repository changed during trace migration")
	}
	if err := directory.confirm(); err != nil {
		return err
	}
	return directory.confirmMember(".trace-operation.lock", operationLock)
}

func stagePrivateFile(directory *heldDirectory, targetName, temporaryName string, expected []byte) error {
	existing, err := readOptionalPublished(directory, targetName, temporaryName)
	if err != nil {
		return err
	}
	if existing != nil {
		if !bytes.Equal(existing, expected) {
			return fmt.Errorf("trace migration collision: %s", targetName)
		}
		if err := clearVerifiedTemporary(directory, temporaryName, expected); err != nil {
			return err
		}
		return syncPublished(directory, targetName, expected)
	}
	staged, err := readOptionalRegular(directory, temporaryName)
	if err != nil {
		return err
	}
	if staged == nil {
		file, err := directory.openRegular(temporaryName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("cannot stage trace migration: %w", err)
		}
		writeErr := writeAllAndSync(file, expected)
		closeErr := file.Close()
		if writeErr != nil {
			return fmt.Errorf("cannot stage trace migration: %w", writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("cannot stage trace migration: %w", closeErr)
		}
		staged, err = directory.readRegular(temporaryName, MaxTraceStoreBytes)
		if err != nil {
			return err
		}
	}
	if !bytes.Equal(staged, expected) {
		return fmt.Errorf("staged trace collision: %s", temporaryName)
	}
	if err := directory.traceRoot.Link(temporaryName, targetName); err != nil {
		return fmt.Errorf("cannot publish staged trace: %w", err)
	}
	if err := directory.traceFile.Sync(); err != nil {
		return fmt.Errorf("interrupted trace publication requires resume: %s", targetName)
	}
	pair, err := readLinkedPair(directory, targetName, temporaryName)
	if err != nil || !bytes.Equal(pair, expected) {
		return fmt.Errorf("published trace verification failed: %s", targetName)
	}
	if err := directory.traceRoot.Remove(temporaryName); err != nil {
		return fmt.Errorf("interrupted trace publication requires resume: %s", targetName)
	}
	if err := directory.traceFile.Sync(); err != nil {
		return fmt.Errorf("interrupted trace publication requires resume: %s", targetName)
	}
	return syncPublished(directory, targetName, expected)
}

func writeAllAndSync(file *os.File, data []byte) error {
	for len(data) != 0 {
		written, err := file.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return file.Sync()
}

func clearVerifiedTemporary(directory *heldDirectory, name string, expected []byte) error {
	temporary, err := readOptionalRegular(directory, name)
	if err != nil {
		if strings.Contains(err.Error(), "unsafe local trace file") {
			pair, pairErr := readLinkedPair(directory, strings.TrimPrefix(strings.TrimSuffix(name, ".tmp"), ".migrate-")+".jsonl", name)
			if pairErr == nil && bytes.Equal(pair, expected) {
				if removeErr := directory.traceRoot.Remove(name); removeErr == nil {
					return directory.traceFile.Sync()
				}
			}
		}
		return err
	}
	if temporary == nil {
		return nil
	}
	if !bytes.Equal(temporary, expected) {
		return fmt.Errorf("staged trace collision: %s", name)
	}
	if err := directory.traceRoot.Remove(name); err != nil {
		return fmt.Errorf("cannot clear verified staged trace: %s", name)
	}
	return directory.traceFile.Sync()
}

func syncPublished(directory *heldDirectory, name string, expected []byte) error {
	file, err := directory.openRegular(name, os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	file.Close()
	return requireBytes(directory, name, expected)
}

func requireBytes(directory *heldDirectory, name string, expected []byte) error {
	actual, err := directory.readRegular(name, MaxTraceStoreBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("trace bytes changed: %s", name)
	}
	return nil
}

func readOptionalRegular(directory *heldDirectory, name string) ([]byte, error) {
	data, err := directory.readRegular(name, MaxTraceStoreBytes)
	if errors.Is(err, os.ErrNotExist) || err != nil && strings.Contains(err.Error(), "no such file") {
		return nil, nil
	}
	return data, err
}

func readOptionalPublished(directory *heldDirectory, targetName, temporaryName string) ([]byte, error) {
	data, err := readPublished(directory, targetName, temporaryName)
	if errors.Is(err, os.ErrNotExist) || err != nil && strings.Contains(err.Error(), "no such file") {
		return nil, nil
	}
	return data, err
}

func readPublished(directory *heldDirectory, targetName, temporaryName string) ([]byte, error) {
	info, err := directory.traceRoot.Lstat(targetName)
	if err != nil {
		return nil, err
	}
	if descriptorLinks(info) == 1 {
		return directory.readRegular(targetName, MaxTraceStoreBytes)
	}
	if descriptorLinks(info) != 2 {
		return nil, fmt.Errorf("unsafe local trace file: %s", targetName)
	}
	pair, err := readLinkedPair(directory, targetName, temporaryName)
	if err != nil {
		return nil, fmt.Errorf("unsafe interrupted trace publication: %s", targetName)
	}
	return pair, nil
}

func readLinkedPair(directory *heldDirectory, targetName, temporaryName string) ([]byte, error) {
	target, targetInfo, err := readRegularLinks(directory, targetName, 2)
	if err != nil {
		return nil, err
	}
	temporary, temporaryInfo, err := readRegularLinks(directory, temporaryName, 2)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(targetInfo, temporaryInfo) || targetInfo.Mode().Perm() != 0o600 || temporaryInfo.Mode().Perm() != 0o600 || !bytes.Equal(target, temporary) {
		return nil, fmt.Errorf("unsafe interrupted trace publication: %s", targetName)
	}
	return target, nil
}

func readRegularLinks(directory *heldDirectory, name string, links uint64) ([]byte, os.FileInfo, error) {
	file, err := openNoFollowMember(directory.traceRoot, name, os.O_RDONLY, 0)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || descriptorLinks(before) != links {
		return nil, nil, fmt.Errorf("unsafe local trace file: %s", name)
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxTraceStoreBytes+1))
	if err != nil || len(data) > MaxTraceStoreBytes {
		return nil, nil, fmt.Errorf("cannot read local trace store: %s", name)
	}
	after, err := file.Stat()
	pathname, pathErr := directory.traceRoot.Lstat(name)
	if err != nil || pathErr != nil || !sameMetadata(before, after) || !os.SameFile(after, pathname) || descriptorLinks(pathname) != links {
		return nil, nil, fmt.Errorf("local trace pathname changed while reading: %s", name)
	}
	return data, after, nil
}

// Result converts a plan to the public command result.
func (plan MigrationPlan) Result(apply bool) MigrationResult {
	rows := 0
	for _, entry := range plan.Entries {
		rows += entry.RowCount
	}
	mode := "dry-run"
	if apply {
		mode = "apply"
	}
	return MigrationResult{
		Mutates: apply, Mode: mode, PlanDigest: plan.Digest,
		CommitRevision: plan.Authority.CommitRevision, TreeRevision: plan.Authority.TreeRevision,
		CandidateTraceFiles: len(plan.Candidates), LegacyTraceFiles: len(plan.Entries), TraceRows: rows,
		Entries: append([]MigrationEntry(nil), plan.Entries...),
	}
}

func objectIDLength(format string) int {
	if format == "sha256" {
		return 64
	}
	return 40
}

func sha256Hex(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
