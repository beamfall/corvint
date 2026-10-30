package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const sourceIDPrefix = "dashboard-acquisition:sha256:"

const issuePlatformIdentityUnsupported IssueCode = "internal-platform-identity-unsupported"

// Read snapshots one explicitly registered source beneath root. The caller is
// responsible for opening and independently authorizing root.
func Read(root *os.Root, relativePath string, kind Kind, verifier VerifierID, budget *Budget, consume StableConsumer) Result {
	return ReadOrdinal(root, relativePath, 0, kind, verifier, budget, consume)
}

// ReadOrdinal performs the exact stable-read acquisition for one configured
// source. configuredOrdinal is evidence only and never participates in a path
// or product source identifier.
func ReadOrdinal(root *os.Root, relativePath string, configuredOrdinal uint64, kind Kind, verifier VerifierID, budget *Budget, consume StableConsumer) Result {
	return ReadOrdinalWithClock(root, relativePath, configuredOrdinal, kind, verifier, budget, ProcessClock(), consume)
}

// ReadOrdinalWithClock is the closed conformance-capable acquisition entry.
// Production callers pass ProcessClock; only an explicitly constructed
// FixedClock can supply deterministic observation boundaries.
func ReadOrdinalWithClock(root *os.Root, relativePath string, configuredOrdinal uint64, kind Kind, verifier VerifierID, budget *Budget, clock Clock, consume StableConsumer) Result {
	return readOrdinal(root, relativePath, configuredOrdinal, kind, verifier, budget, clock, consume, acquisitionHooks{})
}

type component struct {
	name string
	info os.FileInfo
	leaf bool
}

func read(root *os.Root, relativePath string, kind Kind, verifier VerifierID, budget *Budget, consume StableConsumer, afterPreflight func()) Result {
	return readOrdinal(root, relativePath, 0, kind, verifier, budget, ProcessClock(), consume, acquisitionHooks{afterPreflight: func(int) {
		if afterPreflight != nil {
			afterPreflight()
		}
	}})
}

type acquisitionHooks struct {
	afterPreflight func(attempt int)
	afterStat1     func(attempt int)
	afterStat2     func(attempt int)
	afterStat3     func(attempt int)
	afterStat4     func(attempt int)
}

func runAcquisitionHook(hook func(int), attempt int) {
	if hook != nil {
		hook(attempt)
	}
}

type acquisition struct {
	contents []byte
	before   FileIdentity
	after    FileIdentity
	start    time.Time
	end      time.Time
	size     uint64
}

func readOrdinal(root *os.Root, relativePath string, configuredOrdinal uint64, kind Kind, verifier VerifierID, budget *Budget, clock Clock, consume StableConsumer, hooks acquisitionHooks) Result {
	result := failureResult(relativePath, kind, verifier, IssueSourceUnreadable)
	if !registered(kind, verifier) {
		result.Kind = KindUnknown
		result.VerifierID = VerifierUnknown
		result.sourceID = sourceID(relativePath, result.Kind, result.VerifierID)
		result.Issue = IssueInvalidRegistration
		return result
	}
	if !validRelativePath(relativePath) {
		result.Issue = IssueInvalidPath
		return result
	}
	if root == nil {
		return result
	}
	if !clock.valid() {
		result.Issue = IssueInvalidRegistration
		result.Validity = ValidityInvalid
		return result
	}

	var admitted acquisition
	for attempt := 0; attempt < 2; attempt++ {
		var issue IssueCode
		admitted, issue = acquireOnce(root, relativePath, configuredOrdinal, kind, verifier, budget, attempt, clock, hooks)
		if issue == IssueNone {
			break
		}
		result.Bytes = admitted.size
		result.Validity, result.Issue = classifyFailure(issue)
		if issue != IssueSourceUnstable || attempt == 1 {
			return result
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
	if consume != nil {
		content, expire := newStableContent(admitted.contents, digestText)
		func() {
			defer expire()
			consume(content)
		}()
	}
	return result
}

func acquireOnce(root *os.Root, relativePath string, configuredOrdinal uint64, kind Kind, verifier VerifierID, budget *Budget, attempt int, clock Clock, hooks acquisitionHooks) (acquisition, IssueCode) {
	pathBefore, issue := inspect(root, relativePath)
	if issue != IssueNone {
		return acquisition{}, issue
	}
	leaf := pathBefore[len(pathBefore)-1].info
	if leaf.Size() < 0 {
		return acquisition{}, IssueSourceUnreadable
	}
	if uint64(leaf.Size()) > MaxSourceBytes {
		return acquisition{size: uint64(leaf.Size())}, IssueSourceTooLarge
	}
	runAcquisitionHook(hooks.afterPreflight, attempt)
	file, err := openSourceLeaf(root, relativePath)
	if err != nil {
		if os.IsPermission(err) {
			return acquisition{}, IssueSourceUnreadable
		}
		return acquisition{}, IssueSourceUnstable
	}
	defer file.Close()

	startClock := clock.sample()
	start := startClock
	stat1, err := file.Stat()
	if err != nil {
		return acquisition{}, IssueSourceUnreadable
	}
	identity1, qualified := qualifiedDescriptorIdentity(file, stat1)
	runAcquisitionHook(hooks.afterStat1, attempt)
	if !qualified {
		return acquisition{}, issuePlatformIdentityUnsupported
	}
	if !stat1.Mode().IsRegular() {
		return acquisition{}, IssueSourceNotRegular
	}
	if identity1.platform.LinkCount != 1 {
		return acquisition{}, IssueSourceHardLinked
	}
	if !sameIdentity(leaf, stat1, true) {
		return acquisition{}, IssueSourceUnstable
	}
	if stat1.Size() < 0 {
		return acquisition{}, IssueSourceUnreadable
	}
	size := uint64(stat1.Size())
	if size > MaxSourceBytes {
		return acquisition{size: size}, IssueSourceTooLarge
	}
	adapterID, qualified := acquisitionAdapterID(kind, verifier)
	if !qualified || !budget.Preflight(adapterID, configuredOrdinal, MaxSourceBytes).Allowed() {
		return acquisition{}, IssueAggregateBudgetExceeded
	}

	probe := overflowProbeKey{source: budgetKey{adapterID: adapterID, configuredOrdinal: configuredOrdinal}}
	first, firstCount, issue := boundedReadFor(file, budget, size, probe)
	stat2, err := file.Stat()
	if err != nil {
		return acquisition{}, IssueSourceUnreadable
	}
	identity2, qualified := qualifiedDescriptorIdentity(file, stat2)
	runAcquisitionHook(hooks.afterStat2, attempt)
	if issue != IssueNone {
		return acquisition{size: firstCount}, issue
	}
	if !qualified || identity2.platform.LinkCount != 1 || identity2 != identity1 {
		return acquisition{}, IssueSourceUnstable
	}
	if firstCount != size {
		return acquisition{size: firstCount}, IssueSourceUnstable
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return acquisition{}, IssueSourceUnreadable
	}
	stat3, err := file.Stat()
	if err != nil {
		return acquisition{}, IssueSourceUnreadable
	}
	identity3, qualified := qualifiedDescriptorIdentity(file, stat3)
	runAcquisitionHook(hooks.afterStat3, attempt)
	if !qualified || identity3.platform.LinkCount != 1 || identity3 != identity1 {
		return acquisition{}, IssueSourceUnstable
	}
	second, secondCount, issue := boundedReadFor(file, budget, size, probe)
	stat4, err := file.Stat()
	if err != nil {
		return acquisition{}, IssueSourceUnreadable
	}
	identity4, qualified := qualifiedDescriptorIdentity(file, stat4)
	endClock := clock.sample()
	if endClock.Before(startClock) {
		endClock = startClock
	}
	end := endClock
	runAcquisitionHook(hooks.afterStat4, attempt)
	if issue != IssueNone {
		return acquisition{size: secondCount}, issue
	}
	if !qualified || identity4.platform.LinkCount != 1 || identity4 != identity1 {
		return acquisition{}, IssueSourceUnstable
	}
	if secondCount != size || !bytes.Equal(first, second) {
		return acquisition{}, IssueSourceUnstable
	}
	pathAfter, issue := inspect(root, relativePath)
	if issue != IssueNone || len(pathBefore) != len(pathAfter) {
		return acquisition{}, IssueSourceUnstable
	}
	for index := range pathBefore {
		if !sameIdentity(pathBefore[index].info, pathAfter[index].info, pathBefore[index].leaf) {
			return acquisition{}, IssueSourceUnstable
		}
	}
	return acquisition{contents: second, before: identity1, after: identity4, start: start, end: end, size: size}, IssueNone
}

func classifyFailure(issue IssueCode) (Validity, IssueCode) {
	switch issue {
	case IssueSourceUnavailable:
		return ValidityNotPresent, issue
	case IssueSourceUnreadable:
		return ValidityInaccessible, issue
	case issuePlatformIdentityUnsupported:
		return ValidityUnsupported, IssueSourceHardLinked
	default:
		return ValidityInvalid, issue
	}
}

func boundedRead(file *os.File, budget *Budget, declaredSize uint64) ([]byte, uint64, IssueCode) {
	return boundedReadFor(file, budget, declaredSize, overflowProbeKey{source: budgetKey{adapterID: "internal-compatibility"}})
}

func boundedReadFor(file *os.File, budget *Budget, declaredSize uint64, key overflowProbeKey) ([]byte, uint64, IssueCode) {
	if declaredSize > MaxSourceBytes {
		return nil, 0, IssueSourceTooLarge
	}
	readLimit, decision := budget.boundedReadLimit(key, declaredSize)
	if !decision.Allowed() || readLimit == 0 {
		return nil, 0, IssueAggregateBudgetExceeded
	}
	buffer := make([]byte, int(readLimit))
	count, err := io.ReadFull(file, buffer)
	charge, overflow := budget.chargeBoundedRead(key, uint64(count), readLimit-1)
	if !charge.Allowed() {
		return nil, uint64(count), IssueAggregateBudgetExceeded
	}
	if overflow {
		if readLimit-1 < declaredSize {
			return nil, uint64(count), IssueAggregateBudgetExceeded
		}
		if declaredSize == MaxSourceBytes {
			return nil, uint64(count), IssueSourceTooLarge
		}
		return nil, uint64(count), IssueSourceUnstable
	}
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, uint64(count), IssueSourceUnstable
	}
	return buffer[:count], uint64(count), IssueNone
}

func acquisitionAdapterID(kind Kind, verifier VerifierID) (string, bool) {
	if kind == KindLocalTrace && verifier == VerifierLocalTraceV1 {
		return "local-trace-v1", true
	}
	return "", false
}

func inspect(root *os.Root, relativePath string) ([]component, IssueCode) {
	parts := strings.Split(relativePath, "/")
	components := make([]component, 0, len(parts))
	for i := range parts {
		name := strings.Join(parts[:i+1], "/")
		info, err := root.Lstat(name)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, IssueSourceUnavailable
			}
			return nil, IssueSourceUnreadable
		}
		leaf := i == len(parts)-1
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, IssueSourceSymlink
		}
		if !leaf && !info.IsDir() {
			return nil, IssueSourceNotRegular
		}
		if leaf {
			if !info.Mode().IsRegular() {
				return nil, IssueSourceNotRegular
			}
			if linked, known := multipleLinks(info); known && linked {
				return nil, IssueSourceHardLinked
			}
		}
		components = append(components, component{name: name, info: info, leaf: leaf})
	}
	return components, IssueNone
}

func validRelativePath(name string) bool {
	if name == "" || name == "." || len(name) > MaxPathBytes || !utf8.ValidString(name) {
		return false
	}
	if path.IsAbs(name) || filepath.IsAbs(name) || filepath.VolumeName(name) != "" || strings.Contains(name, "\\") {
		return false
	}
	if len(name) >= 2 && name[1] == ':' {
		return false
	}
	if path.Clean(name) != name {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	for _, r := range name {
		if r == 0 || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func sameIdentity(left, right os.FileInfo, leaf bool) bool {
	if left == nil || right == nil || !os.SameFile(left, right) || left.Mode() != right.Mode() {
		return false
	}
	if !leaf {
		return true
	}
	return left.Size() == right.Size() && left.ModTime().Equal(right.ModTime())
}

func multipleLinks(info os.FileInfo) (multiple bool, known bool) {
	if info == nil || info.Sys() == nil {
		return false, false
	}
	value := reflect.ValueOf(info.Sys())
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false, false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return false, false
	}
	for _, name := range [...]string{"Nlink", "NumberOfLinks"} {
		field := value.FieldByName(name)
		if !field.IsValid() {
			continue
		}
		switch field.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return field.Uint() > 1, true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return field.Int() > 1, true
		}
	}
	return false, false
}

func qualifiedIdentity(info os.FileInfo) (FileIdentity, bool) {
	if info == nil || info.Size() < 0 || info.Sys() == nil {
		return FileIdentity{}, false
	}
	value := reflect.ValueOf(info.Sys())
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return FileIdentity{}, false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return FileIdentity{}, false
	}
	device, hasDevice := unsignedField(value, "Dev")
	inode, hasInode := unsignedField(value, "Ino")
	links, hasLinks := unsignedField(value, "Nlink")
	if !hasDevice || !hasInode || !hasLinks {
		// Windows requires handle-qualified file index, link count, and volume
		// serial evidence. os.FileInfo alone cannot establish that profile.
		return FileIdentity{}, false
	}
	return FileIdentity{
		mode: uint32(info.Mode()), modTimeNanoseconds: info.ModTime().UnixNano(), size: uint64(info.Size()),
		platform: PlatformIdentity{Kind: "POSIX", Device: device, Inode: inode, LinkCount: links},
	}, true
}

func unsignedField(value reflect.Value, name string) (uint64, bool) {
	field := value.FieldByName(name)
	if !field.IsValid() {
		return 0, false
	}
	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return field.Uint(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Int() < 0 {
			return 0, false
		}
		return uint64(field.Int()), true
	default:
		return 0, false
	}
}

func windowsPlatformIdentity(volumeSerialNumber, linkCount, fileIndexHigh, fileIndexLow uint32) (PlatformIdentity, bool) {
	if linkCount == 0 {
		return PlatformIdentity{}, false
	}
	return PlatformIdentity{
		Kind:               "WINDOWS",
		LinkCount:          uint64(linkCount),
		FileIndex:          uint64(fileIndexHigh)<<32 | uint64(fileIndexLow),
		VolumeSerialNumber: uint64(volumeSerialNumber),
	}, true
}

func failureResult(relativePath string, kind Kind, verifier VerifierID, issue IssueCode) Result {
	return Result{
		sourceID:   sourceID(relativePath, kind, verifier),
		Kind:       kind,
		VerifierID: verifier,
		Validity:   ValidityInvalid,
		SHA256:     nil,
		Issue:      issue,
	}
}

func sourceID(relativePath string, kind Kind, verifier VerifierID) string {
	if len(relativePath) > MaxPathBytes {
		relativePath = "<invalid-path-over-limit>"
	}
	if len(kind) > maxIdentifierBytes {
		kind = KindUnknown
	}
	if len(verifier) > maxIdentifierBytes {
		verifier = VerifierUnknown
	}
	hash := sha256.New()
	writeFrame(hash, "corvint.dashboard.private-acquisition.v0")
	writeFrame(hash, string(kind))
	writeFrame(hash, string(verifier))
	writeFrame(hash, relativePath)
	return sourceIDPrefix + hex.EncodeToString(hash.Sum(nil))
}

func writeFrame(writer io.Writer, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = io.WriteString(writer, value)
}
