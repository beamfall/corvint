package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const MaxTraceStoreEntries = 1_000

type entryRecord struct {
	name     string
	identity FileIdentity
	reparse  bool
}

type retainedMember struct {
	result   TraceMemberResult
	contents []byte
}

type traceStoreHooks struct {
	afterFirstEnumeration   func(attempt int)
	beforeSecondEnumeration func(attempt int)
	beforeMemberRead        func(name string, attempt int)
}

// ScanTraceStore performs bounded, no-follow pre/post enumeration and stable
// acquisition of immediate revision-named members. It retries the entire store
// once when the directory identity set changes. It does not interpret traces.
func ScanTraceStore(root *os.Root, relativeDirectory string, configuredOrdinal uint64, format ObjectFormat, budget *Budget, consume TraceMemberConsumer) TraceStoreResult {
	return ScanTraceStoreWithClock(root, relativeDirectory, configuredOrdinal, format, budget, ProcessClock(), consume)
}

func ScanTraceStoreWithClock(root *os.Root, relativeDirectory string, configuredOrdinal uint64, format ObjectFormat, budget *Budget, clock Clock, consume TraceMemberConsumer) TraceStoreResult {
	return scanTraceStore(root, relativeDirectory, configuredOrdinal, format, budget, clock, consume, nil)
}

func scanTraceStore(root *os.Root, relativeDirectory string, configuredOrdinal uint64, format ObjectFormat, budget *Budget, clock Clock, consume TraceMemberConsumer, beforeSecondEnumeration func(attempt int)) TraceStoreResult {
	return scanTraceStoreWithHooks(root, relativeDirectory, configuredOrdinal, format, budget, clock, consume, traceStoreHooks{beforeSecondEnumeration: beforeSecondEnumeration})
}

func scanTraceStoreWithHooks(root *os.Root, relativeDirectory string, configuredOrdinal uint64, format ObjectFormat, budget *Budget, clock Clock, consume TraceMemberConsumer, hooks traceStoreHooks) TraceStoreResult {
	if root == nil || !validRelativePath(relativeDirectory) || !clock.valid() || (format != ObjectFormatSHA1 && format != ObjectFormatSHA256) {
		return TraceStoreResult{Validity: ValidityInvalid, Issue: IssueInvalidPath}
	}
	if !budget.Preflight("local-trace-v1", configuredOrdinal, MaxSourceBytes).Allowed() {
		return TraceStoreResult{Validity: ValidityInvalid, Issue: IssueAggregateBudgetExceeded}
	}
	for attempt := 0; attempt < 2; attempt++ {
		result, retained, changed := scanTraceStoreAttempt(root, relativeDirectory, configuredOrdinal, format, budget, attempt, clock, hooks)
		if changed {
			if attempt == 0 {
				continue
			}
			return TraceStoreResult{Validity: ValidityInvalid, Issue: IssueStoreChanged}
		}
		if result.Issue != IssueNone {
			return result
		}
		if consume != nil {
			for _, member := range retained {
				if member.result.Result.Validity != ValidityStable || member.result.Result.Evidence == nil {
					continue
				}
				digest := *member.result.Result.SHA256
				content, expire := newStableContent(member.contents, digest)
				func() {
					defer expire()
					consume(member.result.Revision, content, *member.result.Result.Evidence)
				}()
			}
		}
		return result
	}
	return TraceStoreResult{Validity: ValidityInvalid, Issue: IssueStoreChanged}
}

func scanTraceStoreAttempt(root *os.Root, relativeDirectory string, configuredOrdinal uint64, format ObjectFormat, budget *Budget, attempt int, clock Clock, hooks traceStoreHooks) (TraceStoreResult, []retainedMember, bool) {
	directory, issue := openHeldTraceDirectory(root, relativeDirectory)
	if issue != IssueNone {
		if issue != IssueSourceUnstable {
			validity, publicIssue := classifyFailure(issue)
			return TraceStoreResult{Validity: validity, Issue: publicIssue}, nil, false
		}
		return TraceStoreResult{}, nil, true
	}
	defer directory.close()

	startClock := clock.sample()
	start := startClock
	statBefore, err := directory.file.Stat()
	if err != nil || !statBefore.IsDir() {
		return TraceStoreResult{}, nil, true
	}
	directoryBefore, qualified := qualifiedDescriptorIdentity(directory.file, statBefore)
	if !qualified {
		return TraceStoreResult{Validity: ValidityUnsupported, Issue: IssueSourceHardLinked}, nil, false
	}
	entriesBefore, enumerationIssue := enumerateDirectory(directory)
	if enumerationIssue != IssueNone && enumerationIssue != IssueTraceStoreBound {
		if enumerationIssue == IssueSourceUnreadable {
			return TraceStoreResult{}, nil, true
		}
		return TraceStoreResult{Validity: ValidityInvalid, Issue: enumerationIssue}, nil, false
	}
	if hooks.afterFirstEnumeration != nil {
		hooks.afterFirstEnumeration(attempt)
	}

	var declaredBytes uint64
	boundLimit := uint64(0)
	if enumerationIssue == IssueTraceStoreBound {
		boundLimit = MaxTraceStoreEntries
	}
	retained := make([]retainedMember, 0)
	memberResults := make([]TraceMemberResult, 0)
	var terminalResult *TraceStoreResult
	for _, entry := range entriesBefore {
		if boundLimit != 0 {
			break
		}
		revision, candidate := traceRevision(entry.name, format)
		if !candidate {
			continue
		}
		memberResult, contents, terminalIssue := readTraceStoreMember(directory, entry, configuredOrdinal, attempt, budget, clock, hooks)
		if terminalIssue != IssueNone {
			result := TraceStoreResult{Validity: ValidityInvalid, Issue: terminalIssue}
			if terminalIssue == IssueTraceStoreBound {
				result.Limit = MaxSourceBytes
			}
			terminalResult = &result
			break
		}
		row := TraceMemberResult{Revision: revision, Result: memberResult}
		memberResults = append(memberResults, row)
		retained = append(retained, retainedMember{result: row, contents: contents})
		if memberResult.Validity == ValidityStable {
			if memberResult.Bytes > MaxSourceBytes-declaredBytes {
				boundLimit = MaxSourceBytes
			} else {
				declaredBytes += memberResult.Bytes
			}
		}
	}

	if hooks.beforeSecondEnumeration != nil {
		hooks.beforeSecondEnumeration(attempt)
	}
	entriesAfter, afterEnumerationIssue := enumerateDirectory(directory)
	if afterEnumerationIssue != enumerationIssue {
		return TraceStoreResult{}, nil, true
	}
	if afterEnumerationIssue != IssueNone && afterEnumerationIssue != IssueTraceStoreBound {
		return TraceStoreResult{}, nil, true
	}
	statAfter, err := directory.file.Stat()
	endClock := clock.sample()
	if endClock.Before(startClock) {
		endClock = startClock
	}
	end := endClock
	if err != nil {
		return TraceStoreResult{}, nil, true
	}
	directoryAfter, qualified := qualifiedDescriptorIdentity(directory.file, statAfter)
	if !qualified {
		return TraceStoreResult{Validity: ValidityUnsupported, Issue: IssueSourceHardLinked}, nil, false
	}
	if !directory.bindingsStable() || directoryBefore != directoryAfter || !equalEntrySets(entriesBefore, entriesAfter) {
		return TraceStoreResult{}, nil, true
	}
	if boundLimit != 0 {
		return TraceStoreResult{Validity: ValidityInvalid, Issue: IssueTraceStoreBound, Limit: boundLimit}, nil, false
	}
	if terminalResult != nil {
		return *terminalResult, nil, false
	}
	evidence := &TraceStoreEvidence{
		configuredOrdinal: configuredOrdinal, start: start, end: end,
		directoryBefore: directoryBefore, directoryAfter: directoryAfter,
		entryCount: uint64(len(entriesBefore)),
	}
	return TraceStoreResult{Validity: ValidityStable, Issue: IssueNone, Members: memberResults, Evidence: evidence}, retained, false
}

func enumerateDirectory(directory *heldTraceDirectory) ([]entryRecord, IssueCode) {
	if _, err := directory.file.Seek(0, io.SeekStart); err != nil {
		return nil, IssueSourceUnreadable
	}
	entries, err := directory.file.ReadDir(MaxTraceStoreEntries + 1)
	if err != nil && err != io.EOF {
		return nil, IssueSourceUnreadable
	}
	overBound := len(entries) > MaxTraceStoreEntries
	result := make([]entryRecord, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || !utf8.ValidString(name) || strings.ContainsAny(name, "/\x00") || strings.ContainsRune(name, filepath.Separator) {
			return nil, IssueInvalidPath
		}
		_, identity, reparse, err := directory.lstat(name)
		if err != nil {
			return nil, IssueSourceUnreadable
		}
		result = append(result, entryRecord{name: name, identity: identity, reparse: reparse})
	}
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare([]byte(result[left].name), []byte(result[right].name)) < 0
	})
	if overBound {
		return result, IssueTraceStoreBound
	}
	return result, IssueNone
}

func readTraceStoreMember(directory *heldTraceDirectory, entry entryRecord, configuredOrdinal uint64, storeAttempt int, budget *Budget, clock Clock, hooks traceStoreHooks) (Result, []byte, IssueCode) {
	result := failureResult(entry.name, KindLocalTrace, VerifierLocalTraceV1, IssueSourceUnreadable)
	var admitted acquisition
	for attempt := 0; attempt < 2; attempt++ {
		var issue IssueCode
		admitted, issue = acquireTraceStoreMemberOnce(directory, entry, configuredOrdinal, storeAttempt, budget, clock, hooks)
		if issue == IssueNone {
			break
		}
		if issue == IssueTraceStoreBound || issue == IssueAggregateBudgetExceeded {
			return result, nil, issue
		}
		result.Bytes = admitted.size
		result.Validity, result.Issue = classifyFailure(issue)
		if issue != IssueSourceUnstable || attempt == 1 {
			return result, nil, IssueNone
		}
	}

	digest := sha256.Sum256(admitted.contents)
	digestText := "sha256:" + hex.EncodeToString(digest[:])
	result.Validity = ValidityStable
	result.Bytes = admitted.size
	result.SHA256 = &digestText
	result.Issue = IssueNone
	evidence := Evidence{
		adapterID: "local-trace-v1", byteCount: admitted.size,
		configuredOrdinal: configuredOrdinal, contentSHA256: digestText,
		start: admitted.start, end: admitted.end,
		before: admitted.before, after: admitted.after, validity: ValidityStable,
	}
	result.Evidence = &evidence
	return result, admitted.contents, IssueNone
}

func acquireTraceStoreMemberOnce(directory *heldTraceDirectory, entry entryRecord, configuredOrdinal uint64, storeAttempt int, budget *Budget, clock Clock, hooks traceStoreHooks) (acquisition, IssueCode) {
	pathInfo, pathIdentity, reparse, err := directory.lstat(entry.name)
	if err != nil {
		return acquisition{}, IssueSourceUnstable
	}
	if entry.reparse || reparse || pathInfo.Mode()&os.ModeSymlink != 0 {
		return acquisition{}, IssueSourceSymlink
	}
	if !pathInfo.Mode().IsRegular() {
		return acquisition{}, IssueSourceNotRegular
	}
	if pathIdentity.platform.LinkCount != 1 {
		return acquisition{}, IssueSourceHardLinked
	}
	if pathIdentity != entry.identity {
		return acquisition{}, IssueSourceUnstable
	}
	if pathInfo.Size() < 0 {
		return acquisition{}, IssueSourceUnreadable
	}
	size := uint64(pathInfo.Size())
	if size > MaxSourceBytes {
		return acquisition{size: size}, IssueSourceTooLarge
	}
	file, err := directory.openMember(entry.name)
	if err != nil {
		if os.IsPermission(err) {
			return acquisition{}, IssueSourceUnreadable
		}
		return acquisition{}, IssueSourceUnstable
	}
	defer file.Close()

	start := clock.sample()
	stat1, err := file.Stat()
	if err != nil {
		return acquisition{}, IssueSourceUnreadable
	}
	if reparse, known := descriptorReparse(file); !known {
		return acquisition{}, issuePlatformIdentityUnsupported
	} else if reparse {
		return acquisition{}, IssueSourceSymlink
	}
	identity1, qualified := qualifiedDescriptorIdentity(file, stat1)
	if !qualified {
		return acquisition{}, issuePlatformIdentityUnsupported
	}
	if !stat1.Mode().IsRegular() {
		return acquisition{}, IssueSourceNotRegular
	}
	if identity1.platform.LinkCount != 1 {
		return acquisition{}, IssueSourceHardLinked
	}
	if identity1 != pathIdentity {
		return acquisition{}, IssueSourceUnstable
	}
	if hooks.beforeMemberRead != nil {
		hooks.beforeMemberRead(entry.name, storeAttempt)
	}
	probe := overflowProbeKey{source: budgetKey{adapterID: "local-trace-v1", configuredOrdinal: configuredOrdinal}, member: entry.name, storeAttempt: storeAttempt}
	first, firstCount, issue := boundedReadFor(file, budget, size, probe)
	if issue != IssueNone {
		return acquisition{size: firstCount}, issue
	}
	if firstCount != size {
		return acquisition{size: firstCount}, IssueSourceUnstable
	}
	stat2, err := file.Stat()
	if err != nil {
		return acquisition{}, IssueSourceUnreadable
	}
	identity2, qualified := qualifiedDescriptorIdentity(file, stat2)
	if !qualified || identity2.platform.LinkCount != 1 || identity2 != identity1 {
		return acquisition{}, IssueSourceUnstable
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return acquisition{}, IssueSourceUnreadable
	}
	stat3, err := file.Stat()
	if err != nil {
		return acquisition{}, IssueSourceUnreadable
	}
	identity3, qualified := qualifiedDescriptorIdentity(file, stat3)
	if !qualified || identity3.platform.LinkCount != 1 || identity3 != identity1 {
		return acquisition{}, IssueSourceUnstable
	}
	second, secondCount, issue := boundedReadFor(file, budget, size, probe)
	if issue != IssueNone {
		return acquisition{size: secondCount}, issue
	}
	if secondCount != size {
		return acquisition{size: secondCount}, IssueSourceUnstable
	}
	stat4, err := file.Stat()
	end := clock.sample()
	if end.Before(start) {
		end = start
	}
	if err != nil {
		return acquisition{}, IssueSourceUnreadable
	}
	identity4, qualified := qualifiedDescriptorIdentity(file, stat4)
	if !qualified || identity4.platform.LinkCount != 1 || identity4 != identity1 || !bytes.Equal(first, second) {
		return acquisition{}, IssueSourceUnstable
	}
	pathAfter, pathAfterIdentity, pathAfterReparse, err := directory.lstat(entry.name)
	if err != nil {
		return acquisition{}, IssueSourceUnstable
	}
	if pathAfterIdentity != identity1 || pathAfterReparse || pathAfter.Mode()&os.ModeSymlink != 0 {
		return acquisition{}, IssueSourceUnstable
	}
	return acquisition{contents: second, before: identity1, after: identity4, start: start, end: end, size: size}, IssueNone
}

func traceRevision(name string, format ObjectFormat) (string, bool) {
	length := 40
	if format == ObjectFormatSHA256 {
		length = 64
	}
	if len(name) != length+len(".jsonl") || name[length:] != ".jsonl" {
		return "", false
	}
	revision := name[:length]
	for index := range revision {
		if (revision[index] < '0' || revision[index] > '9') && (revision[index] < 'a' || revision[index] > 'f') {
			return "", false
		}
	}
	return revision, true
}

func equalEntrySets(before, after []entryRecord) bool {
	if len(before) != len(after) {
		return false
	}
	for index := range before {
		if before[index] != after[index] {
			return false
		}
	}
	return true
}
