package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	gitChildTimeout     = 10 * time.Second
	gitAuthorityTimeout = 30 * time.Second
	gitChildStdoutMax   = 8 << 20
	gitChildStderrMax   = 64 << 10
	gitScanStdoutMax    = 128 << 20
	gitChildMax         = 4_096
	gitExecutableMax    = 256 << 20
)

var gitCommonPrefix = []string{
	"--no-optional-locks", "--literal-pathspecs",
	"-c", "core.fsmonitor=false",
	"-c", "core.untrackedCache=false",
	"-c", "core.excludesFile=",
	"-c", "credential.helper=",
	"-c", "submodule.recurse=false",
	"-c", "fetch.recurseSubmodules=false",
	"-c", "protocol.allow=never",
	"-c", "protocol.file.allow=never",
	"-c", "advice.graftFileDeprecated=false",
}

type gitExecutableIdentity struct {
	info   os.FileInfo
	size   int64
	digest [sha256.Size]byte
}

type gitRepositoryProbe struct {
	format     string
	head       string
	tree       string
	status     [sha256.Size]byte
	dirtyPaths []string
	worktree   os.FileInfo
	gitDir     os.FileInfo
	commonDir  os.FileInfo
	objects    os.FileInfo
}

type closedGitAuthority struct {
	ctx            context.Context
	cancel         context.CancelFunc
	executable     string
	executableID   gitExecutableIdentity
	executableFD   *os.File
	environment    []string
	root           string
	gitDir         string
	commonDir      string
	objects        string
	initial        gitRepositoryProbe
	mu             sync.Mutex
	children       int
	stdoutBytes    int64
	catBytes       int64
	catInputBytes  int64
	objectReads    int
	seenObjects    map[string]string
	checkedObjects map[string]struct{}
	revisionTrees  map[string]string
	pathWitnesses  map[string]map[string]struct{}
	finished       bool
}

func newClosedGitAuthority(root string) (*closedGitAuthority, error) {
	if !safeAbsoluteArgument(root) {
		return nil, reject(rejectPrivacyText)
	}
	if os.Getenv("GIT_OBJECT_DIRECTORY") != "" || os.Getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES") != "" {
		return nil, reject(rejectIdentity)
	}
	startupDirectory, err := os.Getwd()
	if err != nil {
		return nil, reject(rejectIdentity)
	}
	executable, err := exec.LookPath("git")
	if err != nil {
		return nil, reject(rejectIdentity)
	}
	if !filepath.IsAbs(executable) {
		executable = filepath.Join(startupDirectory, executable)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil || !filepath.IsAbs(executable) {
		return nil, reject(rejectIdentity)
	}
	executableID, err := stableExecutableIdentity(executable)
	if err != nil {
		return nil, err
	}
	executableFD, err := os.Open(executable)
	if err != nil {
		return nil, reject(rejectIdentity)
	}
	heldInfo, err := executableFD.Stat()
	if err != nil || !os.SameFile(executableID.info, heldInfo) {
		executableFD.Close()
		return nil, reject(rejectIdentity)
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitAuthorityTimeout)
	authority := &closedGitAuthority{
		ctx: ctx, cancel: cancel, executable: executable, executableID: executableID, executableFD: executableFD,
		environment: closedGitEnvironment(), root: root, seenObjects: make(map[string]string), checkedObjects: make(map[string]struct{}), revisionTrees: make(map[string]string), pathWitnesses: make(map[string]map[string]struct{}),
	}
	probe, err := authority.discoverAndProbe(root)
	if err != nil {
		cancel()
		executableFD.Close()
		return nil, err
	}
	authority.initial = probe
	authority.seenObjects[probe.head] = "commit"
	authority.objectReads = 1
	matcher := gitSHA1RE
	if probe.format == "sha256" {
		matcher = gitSHA256RE
	}
	if err := authority.batchCheckObjects([]string{probe.head}, authority.seenObjects, matcher); err != nil {
		cancel()
		executableFD.Close()
		return nil, reject(rejectIdentity)
	}
	authority.checkedObjects[probe.head] = struct{}{}
	return authority, nil
}

func closedGitEnvironment() []string {
	environment := make([]string, 0, 21)
	for _, name := range []string{"SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	environment = append(environment,
		"LANG=C",
		"LC_ALL=C",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1",
		"GIT_GRAFT_FILE="+os.DevNull,
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=",
		"GIT_PROTOCOL_FROM_USER=0",
		"GCM_INTERACTIVE=never",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
	)
	return environment
}

func stableExecutableIdentity(path string) (gitExecutableIdentity, error) {
	read := func() (gitExecutableIdentity, error) {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > gitExecutableMax {
			return gitExecutableIdentity{}, reject(rejectIdentity)
		}
		file, err := os.Open(path)
		if err != nil {
			return gitExecutableIdentity{}, reject(rejectIdentity)
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, io.LimitReader(file, gitExecutableMax+1))
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return gitExecutableIdentity{}, reject(rejectIdentity)
		}
		var digest [sha256.Size]byte
		copy(digest[:], hash.Sum(nil))
		return gitExecutableIdentity{info: info, size: info.Size(), digest: digest}, nil
	}
	first, err := read()
	if err != nil {
		return gitExecutableIdentity{}, err
	}
	second, err := read()
	if err != nil || !sameExecutableIdentity(first, second) {
		return gitExecutableIdentity{}, reject(rejectIdentity)
	}
	return second, nil
}

func sameExecutableIdentity(left, right gitExecutableIdentity) bool {
	return left.size == right.size && left.digest == right.digest && os.SameFile(left.info, right.info)
}

func (authority *closedGitAuthority) discoverAndProbe(root string) (gitRepositoryProbe, error) {
	discoveryTail := []string{"rev-parse", "--path-format=absolute", "--show-toplevel", "--absolute-git-dir", "--git-common-dir"}
	fields, err := authority.runUTF8Lines(root, nil, discoveryTail, 3)
	if err != nil {
		return gitRepositoryProbe{}, err
	}
	for _, field := range fields {
		if !cleanAbsolute(field) {
			return gitRepositoryProbe{}, reject(rejectIdentity)
		}
	}
	worktree, gitDir, commonDir := fields[0], fields[1], fields[2]
	worktreeInfo, err := qualifiedDirectory(worktree)
	if err != nil {
		return gitRepositoryProbe{}, err
	}
	gitDirInfo, err := qualifiedDirectory(gitDir)
	if err != nil {
		return gitRepositoryProbe{}, err
	}
	commonDirInfo, err := qualifiedDirectory(commonDir)
	if err != nil {
		return gitRepositoryProbe{}, err
	}
	reciprocal, err := authority.runUTF8Lines(worktree, nil, discoveryTail, 3)
	if err != nil || !sameQualifiedDirectories(fields, reciprocal) {
		return gitRepositoryProbe{}, reject(rejectIdentity)
	}
	reciprocal, err = authority.runUTF8Lines(gitDir, []string{"--git-dir=.", "--work-tree=" + worktree}, discoveryTail, 3)
	if err != nil || !sameQualifiedDirectories(fields, reciprocal) {
		return gitRepositoryProbe{}, reject(rejectIdentity)
	}
	objectsFields, err := authority.runUTF8Lines(worktree, nil, []string{"rev-parse", "--path-format=absolute", "--git-path", "objects"}, 1)
	if err != nil || !cleanAbsolute(objectsFields[0]) {
		return gitRepositoryProbe{}, reject(rejectIdentity)
	}
	objectsInfo, err := qualifiedDirectory(objectsFields[0])
	expectedObjectsInfo, expectedObjectsErr := qualifiedDirectory(filepath.Join(commonDir, "objects"))
	if err != nil || expectedObjectsErr != nil || !os.SameFile(objectsInfo, expectedObjectsInfo) {
		return gitRepositoryProbe{}, reject(rejectIdentity)
	}
	for _, base := range []string{gitDir, commonDir} {
		if _, statErr := os.Lstat(filepath.Join(base, "objects", "info", "alternates")); statErr == nil || !errors.Is(statErr, os.ErrNotExist) {
			return gitRepositoryProbe{}, reject(rejectIdentity)
		}
	}
	authority.root, authority.gitDir, authority.commonDir, authority.objects = worktree, gitDir, commonDir, objectsFields[0]
	identityFields, err := authority.runLines(worktree, nil, []string{"rev-parse", "--show-object-format", "HEAD^{commit}", "HEAD^{tree}"}, 3)
	if err != nil || identityFields[0] != "sha1" && identityFields[0] != "sha256" {
		return gitRepositoryProbe{}, reject(rejectIdentity)
	}
	matcher := gitSHA1RE
	if identityFields[0] == "sha256" {
		matcher = gitSHA256RE
	}
	if !matcher.MatchString(identityFields[1]) || !matcher.MatchString(identityFields[2]) {
		return gitRepositoryProbe{}, reject(rejectIdentity)
	}
	status, exit, err := authority.run(worktree, nil, []string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none"})
	dirtyPaths, valid := parseStatus(status)
	if err != nil || exit != 0 || !valid {
		return gitRepositoryProbe{}, reject(rejectIdentity)
	}
	return gitRepositoryProbe{
		format: identityFields[0], head: identityFields[1], tree: identityFields[2], status: sha256.Sum256(status), dirtyPaths: dirtyPaths,
		worktree: worktreeInfo, gitDir: gitDirInfo, commonDir: commonDirInfo, objects: objectsInfo,
	}, nil
}

func cleanAbsolute(path string) bool {
	return path != "" && len(path) <= 4096 && filepath.IsAbs(path) && filepath.Clean(path) == path && strings.IndexByte(path, 0) < 0
}

func qualifiedDirectory(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, reject(rejectIdentity)
	}
	return info, nil
}

func sameQualifiedDirectories(expected, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}
	for index := range expected {
		if !cleanAbsolute(actual[index]) {
			return false
		}
		expectedInfo, expectedErr := qualifiedDirectory(expected[index])
		actualInfo, actualErr := qualifiedDirectory(actual[index])
		if expectedErr != nil || actualErr != nil || !os.SameFile(expectedInfo, actualInfo) {
			return false
		}
	}
	return true
}

func pathUnder(path, directory string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (authority *closedGitAuthority) repositoryIdentity() (string, string, error) {
	return authority.initial.format, authority.initial.head, nil
}

func (authority *closedGitAuthority) qualifyCommit(revision, head string) error {
	matcher := gitSHA1RE
	if authority.initial.format == "sha256" {
		matcher = gitSHA256RE
	}
	if !matcher.MatchString(revision) || head != authority.initial.head {
		return reject(rejectIdentity)
	}
	if expected, exists := authority.seenObjects[revision]; exists && expected != "commit" {
		return reject(rejectIdentity)
	}
	if _, exists := authority.seenObjects[revision]; !exists {
		authority.seenObjects[revision] = "commit"
		authority.objectReads++
		if authority.objectReads > 100_000 {
			return reject(rejectSize)
		}
	}
	fields, err := authority.runLines(authority.root, nil, []string{"rev-parse", "--verify", revision + "^{commit}"}, 1)
	if err != nil || fields[0] != revision {
		return reject(rejectIdentity)
	}
	treeFields, err := authority.runLines(authority.root, nil, []string{"rev-parse", "--verify", revision + "^{tree}"}, 1)
	if err != nil || !matcher.MatchString(treeFields[0]) {
		return reject(rejectIdentity)
	}
	authority.revisionTrees[revision] = treeFields[0]
	_, exit, err := authority.run(authority.root, nil, []string{"merge-base", "--is-ancestor", revision, head})
	if err != nil || exit != 0 {
		return reject(rejectIdentity)
	}
	counts, err := authority.runLines(authority.root, nil, []string{"rev-list", "--ancestry-path", "--count", "--max-count=10001", revision + ".." + head}, 1)
	if err != nil || !decimalRE.MatchString(counts[0]) {
		return reject(rejectIdentity)
	}
	count, parseErr := strconv.ParseUint(counts[0], 10, 64)
	if parseErr != nil || count > 10_000 {
		return reject(rejectIdentity)
	}
	return nil
}

func (authority *closedGitAuthority) qualifyPaths(revision string, relative []string) error {
	if len(relative) == 0 {
		return nil
	}
	if len(relative) > 256 {
		return reject(rejectSize)
	}
	arguments := []string{"ls-tree", "-z", "--full-tree", revision, "--"}
	logicalBytes := argumentBytes(gitCommonPrefix) + argumentBytes(arguments)
	for index, item := range relative {
		if !safeTraceRelativePath(item) || index > 0 && relative[index-1] >= item {
			return reject(rejectIdentity)
		}
		logicalBytes += len(item) + 1
	}
	if logicalBytes > 262_144 {
		return reject(rejectSize)
	}
	arguments = append(arguments, relative...)
	output, exit, err := authority.run(authority.root, nil, arguments)
	if err != nil || exit != 0 || len(output) == 0 || output[len(output)-1] != 0 {
		return reject(rejectIdentity)
	}
	rows := bytes.Split(output[:len(output)-1], []byte{0})
	if len(rows) != len(relative) {
		return reject(rejectIdentity)
	}
	matcher := gitSHA1RE
	if authority.initial.format == "sha256" {
		matcher = gitSHA256RE
	}
	objectIDs := make([]string, 0, len(rows))
	for index, rawRow := range rows {
		row := string(rawRow)
		tab := strings.IndexByte(row, '\t')
		if tab < 0 || row[tab+1:] != relative[index] {
			return reject(rejectIdentity)
		}
		fields := strings.Split(row[:tab], " ")
		if len(fields) != 3 || fields[0] != "100644" && fields[0] != "100755" || fields[1] != "blob" || !matcher.MatchString(fields[2]) {
			return reject(rejectIdentity)
		}
		objectIDs = append(objectIDs, fields[2])
		if authority.pathWitnesses[revision] == nil {
			authority.pathWitnesses[revision] = make(map[string]struct{})
		}
		authority.pathWitnesses[revision][fields[2]] = struct{}{}
	}
	sort.Strings(objectIDs)
	objectIDs = uniqueSorted(objectIDs)
	authority.mu.Lock()
	for _, objectID := range objectIDs {
		if expected, seen := authority.seenObjects[objectID]; seen && expected != "blob" {
			authority.mu.Unlock()
			return reject(rejectIdentity)
		} else if !seen {
			authority.seenObjects[objectID] = "blob"
			authority.objectReads++
		}
	}
	overObjects := authority.objectReads > 100_000
	authority.mu.Unlock()
	if overObjects {
		return reject(rejectSize)
	}
	return nil
}

func argumentBytes(arguments []string) int {
	total := 0
	for _, argument := range arguments {
		total += len(argument) + 1
	}
	return total
}

func uniqueSorted(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func (authority *closedGitAuthority) batchCheckObjects(objectIDs []string, expectedTypes map[string]string, matcher *regexp.Regexp) error {
	if len(objectIDs) == 0 {
		return nil
	}
	var stdin bytes.Buffer
	for _, objectID := range objectIDs {
		stdin.WriteString(objectID)
		stdin.WriteByte('\n')
	}
	if stdin.Len() > 4_194_304 {
		return reject(rejectSize)
	}
	authority.mu.Lock()
	authority.catInputBytes += int64(stdin.Len())
	inputOver := authority.catInputBytes > 8_388_608
	authority.mu.Unlock()
	if inputOver {
		return reject(rejectSize)
	}
	output, exit, err := authority.runWithInput(authority.root, nil, []string{"cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)"}, stdin.Bytes(), 4_194_304)
	if err != nil || exit != 0 || len(output) == 0 || output[len(output)-1] != '\n' || bytes.ContainsAny(output, "\r\x00") {
		return reject(rejectIdentity)
	}
	authority.mu.Lock()
	authority.catBytes += int64(len(output))
	over := authority.catBytes > 8_388_608
	authority.mu.Unlock()
	if over {
		return reject(rejectSize)
	}
	lines := strings.Split(string(output[:len(output)-1]), "\n")
	if len(lines) != len(objectIDs) {
		return reject(rejectIdentity)
	}
	for index, line := range lines {
		fields := strings.Split(line, " ")
		if len(fields) != 3 || fields[0] != objectIDs[index] || !matcher.MatchString(fields[0]) || fields[1] != expectedTypes[fields[0]] || !decimalRE.MatchString(fields[2]) {
			return reject(rejectIdentity)
		}
		size, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil || size > maxTraceStoreBytes {
			return reject(rejectSize)
		}
	}
	return nil
}

func longestCatFilePrefixes(objectIDs []string) ([][]string, error) {
	result := make([][]string, 0, (len(objectIDs)+49_999)/50_000)
	for len(objectIDs) != 0 {
		end, bytesUsed := 0, 0
		for end < len(objectIDs) && end < 50_000 && bytesUsed+len(objectIDs[end])+1 <= 4_194_304 {
			bytesUsed += len(objectIDs[end]) + 1
			end++
		}
		if end == 0 {
			return nil, reject(rejectSize)
		}
		result = append(result, objectIDs[:end:end])
		objectIDs = objectIDs[end:]
	}
	return result, nil
}

func (authority *closedGitAuthority) finish() error {
	authority.mu.Lock()
	if authority.finished {
		authority.mu.Unlock()
		return reject(rejectIdentity)
	}
	authority.mu.Unlock()
	defer authority.cancel()
	defer authority.executableFD.Close()
	objectIDs := make([]string, 0, len(authority.seenObjects))
	for objectID := range authority.seenObjects {
		if _, checked := authority.checkedObjects[objectID]; !checked {
			objectIDs = append(objectIDs, objectID)
		}
	}
	sort.Strings(objectIDs)
	matcher := gitSHA1RE
	if authority.initial.format == "sha256" {
		matcher = gitSHA256RE
	}
	prefixes, err := longestCatFilePrefixes(objectIDs)
	if err != nil {
		return err
	}
	for _, prefix := range prefixes {
		if authority.batchCheckObjects(prefix, authority.seenObjects, matcher) != nil {
			return reject(rejectIdentity)
		}
	}
	probe, err := authority.discoverAndProbe(authority.root)
	if err != nil || !sameRepositoryProbe(authority.initial, probe) {
		return reject(rejectIdentity)
	}
	executableID, err := stableExecutableIdentity(authority.executable)
	if err != nil || !sameExecutableIdentity(authority.executableID, executableID) {
		return reject(rejectIdentity)
	}
	authority.mu.Lock()
	authority.finished = true
	authority.mu.Unlock()
	return nil
}

func (authority *closedGitAuthority) probeExecutableBinding() error {
	held, err := authority.executableFD.Stat()
	if err != nil || !os.SameFile(authority.executableID.info, held) || held.Size() != authority.executableID.size {
		return reject(rejectIdentity)
	}
	pathInfo, err := os.Lstat(authority.executable)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() || !os.SameFile(authority.executableID.info, pathInfo) || pathInfo.Size() != authority.executableID.size {
		return reject(rejectIdentity)
	}
	return nil
}

func sameRepositoryProbe(left, right gitRepositoryProbe) bool {
	return left.format == right.format && left.head == right.head && left.tree == right.tree && left.status == right.status &&
		os.SameFile(left.worktree, right.worktree) && os.SameFile(left.gitDir, right.gitDir) &&
		os.SameFile(left.commonDir, right.commonDir) && os.SameFile(left.objects, right.objects)
}

type gitSemanticEvidence struct {
	format        string
	head          string
	tree          string
	dirtyPaths    []string
	revisionTrees map[string]string
	pathWitnesses map[string][]string
}

func (authority *closedGitAuthority) semanticEvidence() gitSemanticEvidence {
	evidence := gitSemanticEvidence{
		format: authority.initial.format, head: authority.initial.head, tree: authority.initial.tree,
		dirtyPaths: append([]string(nil), authority.initial.dirtyPaths...), revisionTrees: make(map[string]string), pathWitnesses: make(map[string][]string),
	}
	for revision, tree := range authority.revisionTrees {
		evidence.revisionTrees[revision] = tree
	}
	for revision, values := range authority.pathWitnesses {
		for objectID := range values {
			evidence.pathWitnesses[revision] = append(evidence.pathWitnesses[revision], objectID)
		}
		sort.Strings(evidence.pathWitnesses[revision])
	}
	return evidence
}

func (authority *closedGitAuthority) runLines(cwd string, inserted, tail []string, count int) ([]string, error) {
	return authority.runLineFields(cwd, inserted, tail, count, true)
}

func (authority *closedGitAuthority) runUTF8Lines(cwd string, inserted, tail []string, count int) ([]string, error) {
	return authority.runLineFields(cwd, inserted, tail, count, false)
}

func (authority *closedGitAuthority) runLineFields(cwd string, inserted, tail []string, count int, asciiOnly bool) ([]string, error) {
	output, exit, err := authority.run(cwd, inserted, tail)
	if err != nil || exit != 0 || len(output) == 0 || output[len(output)-1] != '\n' || bytes.ContainsAny(output, "\r\x00") {
		return nil, reject(rejectIdentity)
	}
	fields := strings.Split(string(output[:len(output)-1]), "\n")
	if len(fields) != count {
		return nil, reject(rejectIdentity)
	}
	for _, field := range fields {
		if field == "" || !utf8.ValidString(field) {
			return nil, reject(rejectIdentity)
		}
		for _, character := range field {
			if character < 0x20 || unicode.IsControl(character) || asciiOnly && character > 0x7e {
				return nil, reject(rejectIdentity)
			}
		}
	}
	return fields, nil
}

func (authority *closedGitAuthority) run(cwd string, inserted, tail []string) ([]byte, int, error) {
	return authority.runWithInput(cwd, inserted, tail, nil, gitChildStdoutMax)
}

func (authority *closedGitAuthority) runWithInput(cwd string, inserted, tail []string, stdin []byte, stdoutMaximum int) ([]byte, int, error) {
	if err := authority.probeExecutableBinding(); err != nil {
		return nil, -1, err
	}
	authority.mu.Lock()
	if authority.finished || authority.children >= gitChildMax || authority.stdoutBytes >= gitScanStdoutMax {
		authority.mu.Unlock()
		return nil, -1, reject(rejectSize)
	}
	authority.children++
	authority.mu.Unlock()
	ctx, cancel := context.WithTimeout(authority.ctx, gitChildTimeout)
	defer cancel()
	arguments := make([]string, 0, len(gitCommonPrefix)+len(inserted)+len(tail))
	arguments = append(arguments, gitCommonPrefix...)
	arguments = append(arguments, inserted...)
	arguments = append(arguments, tail...)
	command := exec.Command(authority.executable, arguments...)
	command.Args[0] = authority.executable
	command.Dir = cwd
	command.Env = append([]string(nil), authority.environment...)
	command.Stdin = bytes.NewReader(stdin)
	stdout := &boundedBuffer{maximum: stdoutMaximum}
	stderr := &boundedBuffer{maximum: gitChildStderrMax}
	command.Stdout, command.Stderr = stdout, stderr
	if !prepareContainedCommand(command) {
		return nil, -1, reject(rejectIdentity)
	}
	if err := command.Start(); err != nil {
		return nil, -1, reject(rejectIdentity)
	}
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	var err error
	select {
	case err = <-wait:
		signalContainedCommand(command, true)
	case <-ctx.Done():
		signalContainedCommand(command, false)
		select {
		case err = <-wait:
		case <-time.After(250 * time.Millisecond):
			signalContainedCommand(command, true)
			err = <-wait
		}
	}
	if bindingErr := authority.probeExecutableBinding(); bindingErr != nil {
		return nil, -1, bindingErr
	}
	if ctx.Err() != nil || stdout.exceeded || stderr.exceeded {
		return nil, -1, reject(rejectSize)
	}
	exit := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return nil, -1, reject(rejectIdentity)
		}
		exit = exitError.ExitCode()
	}
	authority.mu.Lock()
	authority.stdoutBytes += int64(stdout.buffer.Len())
	over := authority.stdoutBytes > gitScanStdoutMax
	authority.mu.Unlock()
	if over {
		return nil, -1, reject(rejectSize)
	}
	return bytes.Clone(stdout.buffer.Bytes()), exit, nil
}

type boundedBuffer struct {
	buffer   bytes.Buffer
	maximum  int
	exceeded bool
}

func (writer *boundedBuffer) Write(content []byte) (int, error) {
	remaining := writer.maximum - writer.buffer.Len()
	if len(content) > remaining {
		if remaining > 0 {
			_, _ = writer.buffer.Write(content[:remaining])
		}
		writer.exceeded = true
		return remaining, errors.New("bounded output")
	}
	return writer.buffer.Write(content)
}

func parseStatus(raw []byte) ([]string, bool) {
	if len(raw) == 0 {
		return []string{}, true
	}
	if raw[len(raw)-1] != 0 || !utf8.Valid(raw) {
		return nil, false
	}
	fields := bytes.Split(raw[:len(raw)-1], []byte{0})
	paths := 0
	result := make([]string, 0, len(fields))
	for index := 0; index < len(fields); index++ {
		record := fields[index]
		if len(record) < 4 || record[2] != ' ' || !validStatusCode(record[0]) || !validStatusCode(record[1]) || !safeTraceRelativePath(string(record[3:])) {
			return nil, false
		}
		result = append(result, string(record[3:]))
		paths++
		if record[0] == 'R' || record[0] == 'C' || record[1] == 'R' || record[1] == 'C' {
			index++
			if index >= len(fields) || !safeTraceRelativePath(string(fields[index])) {
				return nil, false
			}
			result = append(result, string(fields[index]))
			paths++
		}
		if paths > 100_000 {
			return nil, false
		}
	}
	sort.Strings(result)
	return uniqueSorted(result), true
}

func validStatusCode(value byte) bool {
	return strings.ContainsRune(" MADRCU?!T", rune(value))
}
