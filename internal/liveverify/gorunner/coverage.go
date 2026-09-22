package gorunner

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type CoverageMode string

const (
	CoverageModeSet    CoverageMode = "set"
	CoverageModeCount  CoverageMode = "count"
	CoverageModeAtomic CoverageMode = "atomic"
)

type CoverageCompleteness string

const (
	CoverageAbsent   CoverageCompleteness = "ABSENT"
	CoverageComplete CoverageCompleteness = "COMPLETE"
	CoveragePartial  CoverageCompleteness = "PARTIAL"
	CoverageRejected CoverageCompleteness = "REJECTED"
)

type CoverageSourceFile struct {
	ProfilePath string
	SourcePath  string
	RawSHA256   string
}

type CoveragePackage struct {
	ImportPath string
	Files      []CoverageSourceFile
}

type CoverageRequest struct {
	Directory      string
	ProfilePath    string
	RepositoryRoot string
	Mode           CoverageMode
	Packages       []CoveragePackage
}

type CoverageObservation struct {
	ArtifactSHA256 string
	Completeness   CoverageCompleteness
	Mode           string
	Packages       []string
	RawRootSHA256  string
}

type coverageCapture struct {
	request       CoverageRequest
	root          *os.Root
	directoryInfo os.FileInfo
	firstInfo     os.FileInfo
	stop          chan struct{}
	done          chan struct{}
	violation     bool
}

type coverageBlock struct {
	line        string
	packageName string
}

func absentCoverageObservation() CoverageObservation {
	return CoverageObservation{Completeness: CoverageAbsent, Mode: "NONE", Packages: []string{}}
}

func rejectedCoverageObservation(mode CoverageMode) CoverageObservation {
	return CoverageObservation{Completeness: CoverageRejected, Mode: coverageObservationMode(mode), Packages: []string{}}
}

func coverageObservationMode(mode CoverageMode) string {
	switch mode {
	case CoverageModeSet:
		return "UNIT_COVERPROFILE_SET"
	case CoverageModeCount:
		return "UNIT_COVERPROFILE_COUNT"
	case CoverageModeAtomic:
		return "UNIT_COVERPROFILE_ATOMIC"
	default:
		return "NONE"
	}
}

func validateCoverageRequest(request CoverageRequest) error {
	if request.Mode != CoverageModeSet && request.Mode != CoverageModeCount && request.Mode != CoverageModeAtomic {
		return invalid("Coverage mode is not admitted")
	}
	for label, value := range map[string]string{
		"Coverage directory":  request.Directory,
		"Coverage profile":    request.ProfilePath,
		"Coverage repository": request.RepositoryRoot,
	} {
		if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value {
			return invalid(label + " must be a clean absolute path")
		}
	}
	if filepath.Dir(request.ProfilePath) != request.Directory || filepath.Base(request.ProfilePath) == "." {
		return invalid("Coverage profile must be a direct child of its directory")
	}
	if !resolvedCoveragePath(request.Directory) || !resolvedCoveragePath(request.RepositoryRoot) {
		return invalid("Coverage roots must have resolved parent components")
	}
	if withinPath(request.RepositoryRoot, request.Directory) {
		return invalid("Coverage directory must be outside the repository")
	}
	if len(request.Packages) == 0 || len(request.Packages) > MaxPackages {
		return invalid("Coverage packages must be non-empty and bounded")
	}
	previousPackage := ""
	seenProfiles := make(map[string]struct{})
	fileCount := 0
	for _, pack := range request.Packages {
		if pack.ImportPath <= previousPackage || !validImportPath(pack.ImportPath) || len(pack.Files) == 0 {
			return invalid("Coverage packages must be sorted, unique, and mapped")
		}
		previousPackage = pack.ImportPath
		previousProfile := ""
		for _, file := range pack.Files {
			fileCount++
			if fileCount > 4_096 {
				return invalid("Coverage files exceed the admitted bound")
			}
			if file.ProfilePath <= previousProfile || !validCoverageProfilePath(pack.ImportPath, file.ProfilePath) {
				return invalid("Coverage files must be sorted and package-mapped")
			}
			previousProfile = file.ProfilePath
			if _, duplicate := seenProfiles[file.ProfilePath]; duplicate {
				return invalid("Coverage profile paths must be unique")
			}
			seenProfiles[file.ProfilePath] = struct{}{}
			if file.SourcePath == "" || !filepath.IsAbs(file.SourcePath) || filepath.Clean(file.SourcePath) != file.SourcePath ||
				!resolvedCoveragePath(file.SourcePath) || !withinPath(request.RepositoryRoot, file.SourcePath) || !validLowerDigest(file.RawSHA256) {
				return invalid("Coverage source mapping is invalid")
			}
		}
	}
	return nil
}

func resolvedCoveragePath(value string) bool {
	resolved, err := filepath.EvalSymlinks(value)
	return err == nil && filepath.Clean(resolved) == value
}

func validCoverageProfilePath(importPath, name string) bool {
	if name == "" || !utf8.ValidString(name) || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return false
	}
	prefix := importPath + "/"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	relative := strings.TrimPrefix(name, prefix)
	return relative != "" && filepath.ToSlash(filepath.Clean(filepath.FromSlash(relative))) == relative &&
		relative != ".." && !strings.HasPrefix(relative, "../")
}

func prepareCoverage(request *CoverageRequest) (*coverageCapture, error) {
	if request == nil {
		return nil, nil
	}
	info, err := os.Lstat(request.Directory)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 || !coverageOwnedByCurrentUser(info) {
		return nil, invalid("Coverage directory must be a non-symlink mode-0700 directory")
	}
	root, err := os.OpenRoot(request.Directory)
	if err != nil {
		return nil, invalid("Coverage directory could not be opened")
	}
	capture := &coverageCapture{
		request: *request, root: root, directoryInfo: info,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
	entries, err := capture.entries()
	if err != nil || len(entries) != 0 {
		_ = root.Close()
		return nil, invalid("Coverage directory must be fresh and empty")
	}
	if _, err := root.Lstat(filepath.Base(request.ProfilePath)); !errors.Is(err, os.ErrNotExist) {
		_ = root.Close()
		return nil, invalid("Coverage profile target must be absent")
	}
	go capture.watch()
	return capture, nil
}

func (capture *coverageCapture) watch() {
	defer close(capture.done)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		capture.inspect()
		select {
		case <-capture.stop:
			capture.inspect()
			return
		case <-ticker.C:
		}
	}
}

func (capture *coverageCapture) inspect() {
	entries, err := capture.entries()
	if err != nil || len(entries) > 1 || len(entries) == 1 && entries[0].Name() != filepath.Base(capture.request.ProfilePath) {
		capture.violation = true
		return
	}
	info, err := capture.root.Lstat(filepath.Base(capture.request.ProfilePath))
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		capture.violation = true
		return
	}
	if capture.firstInfo == nil {
		capture.firstInfo = info
		return
	}
	if !os.SameFile(capture.firstInfo, info) {
		capture.violation = true
	}
}

func (capture *coverageCapture) entries() ([]os.DirEntry, error) {
	directory, err := capture.root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := directory.ReadDir(2)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	return entries, closeErr
}

func (capture *coverageCapture) finish(result Result, runErr error) CoverageObservation {
	close(capture.stop)
	<-capture.done
	defer capture.root.Close()
	observation := rejectedCoverageObservation(capture.request.Mode)
	if capture.violation || capture.firstInfo == nil {
		return observation
	}
	directoryInfo, err := capture.root.Stat(".")
	if err != nil || !os.SameFile(capture.directoryInfo, directoryInfo) || directoryInfo.Mode().Perm() != 0o700 ||
		!coverageOwnedByCurrentUser(directoryInfo) {
		return observation
	}
	entries, err := capture.entries()
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(capture.request.ProfilePath) {
		return observation
	}
	raw, err := capture.readProfile()
	if err != nil {
		return observation
	}
	canonical, packages, err := parseCoverageProfile(raw, capture.request)
	if err != nil {
		return observation
	}
	rawDigest := sha256.Sum256(raw)
	artifactDigest := sha256.Sum256(canonical)
	observation.ArtifactSHA256 = hex.EncodeToString(artifactDigest[:])
	observation.RawRootSHA256 = hex.EncodeToString(rawDigest[:])
	observation.Packages = packages
	if coverageTerminalComplete(result, runErr) {
		observation.Completeness = CoverageComplete
	} else {
		observation.Completeness = CoveragePartial
	}
	return observation
}

func (capture *coverageCapture) readProfile() ([]byte, error) {
	name := filepath.Base(capture.request.ProfilePath)
	before, err := capture.root.Lstat(name)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || !coverageOwnedByCurrentUser(before) ||
		!coverageSingleLink(before) || !os.SameFile(capture.firstInfo, before) || before.Size() > MaxCoverageBytes {
		return nil, errors.New("coverage target is not the bound regular file")
	}
	file, err := openCoverageNoFollow(capture.root, name)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	if statErr != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		_ = file.Close()
		return nil, errors.New("coverage target identity changed")
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, MaxCoverageBytes+1))
	after, afterErr := file.Stat()
	closeErr := file.Close()
	pathAfter, pathErr := capture.root.Lstat(name)
	if readErr != nil || afterErr != nil || closeErr != nil || pathErr != nil || int64(len(raw)) > MaxCoverageBytes ||
		!os.SameFile(opened, after) || !os.SameFile(after, pathAfter) || after.Size() != int64(len(raw)) ||
		opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return nil, errors.New("coverage target changed while reading")
	}
	return raw, nil
}

func parseCoverageProfile(raw []byte, request CoverageRequest) ([]byte, []string, error) {
	if len(raw) == 0 || !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		return nil, nil, errors.New("malformed coverage profile")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	if !scanner.Scan() || scanner.Text() != "mode: "+string(request.Mode) {
		return nil, nil, errors.New("coverage mode mismatch")
	}
	files := make(map[string]CoverageSourceFile)
	filePackages := make(map[string]string)
	for _, pack := range request.Packages {
		for _, file := range pack.Files {
			files[file.ProfilePath] = file
			filePackages[file.ProfilePath] = pack.ImportPath
			if validateCoverageSource(file) != nil {
				return nil, nil, errors.New("coverage source mapping rejected")
			}
		}
	}
	var blocks []coverageBlock
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode: ") {
			return nil, nil, errors.New("mixed coverage mode")
		}
		block, profilePath, err := parseCoverageBlock(line, request.Mode)
		if err != nil {
			return nil, nil, err
		}
		_, admitted := files[profilePath]
		if !admitted {
			return nil, nil, errors.New("coverage source mapping rejected")
		}
		blocks = append(blocks, coverageBlock{line: block, packageName: filePackages[profilePath]})
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	sort.Slice(blocks, func(left, right int) bool { return blocks[left].line < blocks[right].line })
	packages := make([]string, 0)
	seenPackages := make(map[string]struct{})
	canonical := []byte("mode: " + string(request.Mode) + "\n")
	previous := ""
	for _, block := range blocks {
		if block.line == previous {
			return nil, nil, errors.New("duplicate coverage block")
		}
		previous = block.line
		canonical = append(canonical, block.line...)
		canonical = append(canonical, '\n')
		seenPackages[block.packageName] = struct{}{}
	}
	for packageName := range seenPackages {
		packages = append(packages, packageName)
	}
	sort.Strings(packages)
	return canonical, packages, nil
}

func parseCoverageBlock(line string, mode CoverageMode) (string, string, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 || strings.ContainsAny(line, "\r\t") {
		return "", "", errors.New("malformed coverage block")
	}
	colon := strings.LastIndexByte(fields[0], ':')
	if colon <= 0 {
		return "", "", errors.New("coverage location is absent")
	}
	profilePath := fields[0][:colon]
	rangeParts := strings.Split(fields[0][colon+1:], ",")
	if len(rangeParts) != 2 {
		return "", "", errors.New("coverage range is malformed")
	}
	startLine, startColumn, err := parseCoveragePosition(rangeParts[0])
	if err != nil {
		return "", "", err
	}
	endLine, endColumn, err := parseCoveragePosition(rangeParts[1])
	if err != nil || endLine < startLine || endLine == startLine && endColumn <= startColumn {
		return "", "", errors.New("coverage range is invalid")
	}
	statements, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil || statements == 0 {
		return "", "", errors.New("coverage statement count is invalid")
	}
	count, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil || mode == CoverageModeSet && count > 1 {
		return "", "", errors.New("coverage counter is invalid")
	}
	canonical := fmt.Sprintf("%s:%d.%d,%d.%d %d %d", profilePath, startLine, startColumn, endLine, endColumn, statements, count)
	return canonical, profilePath, nil
}

func parseCoveragePosition(value string) (uint64, uint64, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return 0, 0, errors.New("coverage position is malformed")
	}
	line, lineErr := strconv.ParseUint(parts[0], 10, 32)
	column, columnErr := strconv.ParseUint(parts[1], 10, 32)
	if lineErr != nil || columnErr != nil || line == 0 || column == 0 {
		return 0, 0, errors.New("coverage position is invalid")
	}
	return line, column, nil
}

func validateCoverageSource(file CoverageSourceFile) error {
	before, err := os.Lstat(file.SourcePath)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return errors.New("coverage source is not regular")
	}
	handle, err := os.Open(file.SourcePath)
	if err != nil {
		return err
	}
	opened, statErr := handle.Stat()
	digest := sha256.New()
	_, readErr := io.CopyBuffer(digest, handle, make([]byte, 32<<10))
	after, afterErr := handle.Stat()
	closeErr := handle.Close()
	if statErr != nil || readErr != nil || afterErr != nil || closeErr != nil || !os.SameFile(before, opened) ||
		!os.SameFile(opened, after) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) ||
		hex.EncodeToString(digest.Sum(nil)) != file.RawSHA256 {
		return errors.New("coverage source identity changed")
	}
	return nil
}

func coverageTerminalComplete(result Result, runErr error) bool {
	return runErr == nil && result.Started && result.Exited && result.ExitCode == 0 && !result.Cancelled && !result.TimedOut &&
		!result.OutputLimitExceeded && !result.PipeWaitExpired && result.ProcessCleanupDone && result.Stdout.Drained && result.Stderr.Drained &&
		result.Containment == ContainmentProcessGroupBestEffort && validLowerDigest(result.Stdout.RawSHA256) && validLowerDigest(result.Stderr.RawSHA256)
}

func withinPath(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validLowerDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}
