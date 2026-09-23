// Package unplannedread measures reads that Corvint failed to prevent: tool
// calls that opened a project file the delivered context packet did not
// already carry. It is an explicitly opt-in, bounded, local, private ledger.
// It is never an input to ranking, evidence, or authority, is read as labels
// only by the operator-invoked slot-weight learning step (decision 0368), and
// stores no file contents.
package unplannedread

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/observations"
	"github.com/Beamfall/corvint/internal/projectpath"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// The bounds are copied from internal/observations so the two private ledgers
// cost the same worst case on disk and per row.
const (
	maxFileBytes = 128 * 1024
	maxRowBytes  = 2048

	ledgerName    = "unplanned-reads.jsonl"
	markerName    = "unplanned-reads.enabled"
	temporaryGlob = ".unplanned-reads.*"
)

var appendProcessLock sync.Mutex

var (
	errPlannedRowPath = errors.New("planned row must carry no path")
	errRowBound       = errors.New("unplanned-read row exceeds bound")
	// A symlinked ledger would copy an outside file's bytes into the ledger.
	errLedgerNotRegular = errors.New("unplanned-read ledger or directory is not regular")
	// A marker, ledger or temporary git would track dirties the worktree and
	// could commit read paths; a repository-committed marker is one such case.
	errLedgerNotIgnored = errors.New("unplanned-read ledger is not safely gitignored")
)

// Event is one classified read. Planned rows carry no path: only the counts
// needed to compute the unplanned share.
type Event struct {
	Timestamp    string `json:"ts"`
	Tool         string `json:"tool"`
	Path         string `json:"path,omitempty"`
	Bytes        int64  `json:"bytes"`
	SizeKnown    bool   `json:"size_known"`
	PacketDigest string `json:"packet"`
	Planned      bool   `json:"planned"`
}

// bashReaders is the closed set of commands the Bash heuristic recognizes.
// The value is the number of leading non-flag arguments to skip before the
// file operand: sed's first non-flag argument is its script, not a path.
var bashReaders = map[string]int{"cat": 0, "head": 0, "tail": 0, "sed": 1}

// Classify decides whether one post-tool payload is a project-relative read
// and whether the packet already planned it. It reads filesystem metadata to resolve
// containment and the worktree path's byte size. The second result is false when the
// call is not a recognized read or its target is not inside the worktree.
func Classify(packetPaths []string, toolName string, toolInput map[string]any, root string) (Event, bool) {
	isPlanned := func(relative string) bool { return planned(packetPaths, relative) }
	return classifyWith(isPlanned, PacketDigest(packetPaths), toolName, toolInput, root, "")
}

// classifyWith resolves a relative target from base, the host's absolute
// working directory, or from root when base is empty.
func classifyWith(isPlanned func(string) bool, digest, toolName string, toolInput map[string]any, root, base string) (Event, bool) {
	target, found := readTarget(toolName, toolInput)
	if !found {
		return Event{}, false
	}
	if base != "" && !filepath.IsAbs(target) {
		// Join would erase '..' before the filesystem can resolve preceding links.
		target = base + string(filepath.Separator) + target
	}
	contained, inside := projectpath.Relative(root, target)
	if !inside {
		return Event{}, false
	}
	relative, stored := storedSpelling(root, contained)
	if !stored {
		return Event{}, false
	}
	size, known := worktreeSize(root, relative)
	// A heuristic Bash operand that names no file was a flag value or script.
	sizeRequired := toolName == "Bash"
	if sizeRequired && !known {
		return Event{}, false
	}
	event := Event{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Tool:         toolName,
		Path:         relative,
		Bytes:        size,
		SizeKnown:    known,
		PacketDigest: digest,
		Planned:      isPlanned(relative),
	}
	if event.Planned {
		event.Path, event.Bytes, event.SizeKnown = "", 0, false
	}
	return event, true
}

// readTarget extracts the single path a read-shaped tool call opened. Read,
// Grep and Glob carry it in a named field. Bash is a deliberately bounded
// heuristic: the command is split on `|`, `;` and `&&` into at most eight
// segments of at most 32 fields, and the first segment whose command basename
// is cat, sed, head or tail contributes its first file operand. A leading `-`
// field is a flag and a purely numeric field is a flag value (`head -n 5`);
// both are skipped.
func readTarget(toolName string, toolInput map[string]any) (string, bool) {
	switch toolName {
	case "Read":
		return stringField(toolInput, "file_path")
	case "Grep", "Glob":
		return stringField(toolInput, "path")
	case "Bash":
		command, ok := stringField(toolInput, "command")
		if !ok {
			return "", false
		}
		return bashReadTarget(command)
	}
	return "", false
}

func stringField(input map[string]any, name string) (string, bool) {
	value, ok := input[name].(string)
	return value, ok && value != ""
}

func bashReadTarget(command string) (string, bool) {
	preceding := ""
	for index, segment := range strings.FieldsFunc(command, isSegmentBreak) {
		if index >= 8 {
			return "", false
		}
		literal := rootLiteralContext(preceding)
		preceding += segment + "\n"
		if !literal {
			continue
		}
		if path, ok := segmentReadTarget(segment); ok {
			return path, !commandChangesDirectory(command)
		}
	}
	return "", false
}

// commandChangesDirectory reports a directory change anywhere in the command: the
// host reports its working directory after the command ran, so a change that
// follows the read still moves the base its relative operand resolves from.
func commandChangesDirectory(command string) bool {
	for _, segment := range strings.FieldsFunc(command, isSegmentBreak) {
		if changesDirectory(segment) {
			return true
		}
	}
	return false
}

// directoryChangers move the base every later relative operand resolves from.
var directoryChangers = map[string]bool{"cd": true, "pushd": true, "popd": true}

// rootLiteralContext reports whether a segment after the preceding segments is
// still a command at the worktree root: an odd quote count means the split fell
// inside a quoted word, a heredoc makes later lines its body, and a directory
// change moves the base of relative operands.
func rootLiteralContext(preceding string) bool {
	if strings.Count(preceding, "'")%2 != 0 {
		return false
	}
	if strings.Count(preceding, `"`)%2 != 0 {
		return false
	}
	if strings.Contains(preceding, "<<") {
		return false
	}
	for _, segment := range strings.Split(preceding, "\n") {
		if changesDirectory(segment) {
			return false
		}
	}
	return true
}

func changesDirectory(segment string) bool {
	fields := strings.Fields(strings.TrimLeft(strings.TrimSpace(segment), "({"))
	return len(fields) > 0 && directoryChangers[fields[0]]
}

// isNumeric admits one leading `+`, so `tail -n +5` reads as a flag value.
func isNumeric(field string) bool {
	digits := strings.TrimPrefix(field, "+")
	return strings.TrimLeft(digits, "0123456789") == "" && digits != ""
}

// shellSyntax marks a field that is a redirection, expansion, glob, escape or
// quote rather than the literal path it spells.
const shellSyntax = "<>$*?[]{}()~\\`'\""

func isSegmentBreak(r rune) bool { return r == '|' || r == ';' || r == '&' || r == '\n' }

// URE-V0-002: a missed read under-counts; an operand the split cannot read
// literally abstains instead of inventing a path.
func segmentReadTarget(segment string) (string, bool) {
	fields := strings.Fields(segment)
	if len(fields) == 0 || len(fields) > 32 {
		return "", false
	}
	skip, recognized := bashReaders[filepath.Base(fields[0])]
	if !recognized {
		return "", false
	}
	if splitInsideQuotes(fields) {
		return "", false
	}
	for _, field := range fields[1:] {
		if skip > 0 && sedScriptFlag(field) {
			return "", false
		}
		if strings.HasPrefix(field, "-") || isNumeric(field) {
			continue
		}
		if skip > 0 {
			skip--
			continue
		}
		return literalOperand(field)
	}
	return "", false
}

// splitInsideQuotes reports a field with an odd count of either quote: the
// whitespace split cut a quoted word, so no field of the segment is a word.
func splitInsideQuotes(fields []string) bool {
	for _, field := range fields {
		if strings.Count(field, "'")%2 != 0 {
			return true
		}
		if strings.Count(field, `"`)%2 != 0 {
			return true
		}
	}
	return false
}

// sedScriptFlag reports a sed flag that supplies the script, after which the
// first non-flag operand is a file, not the script the position rule assumes.
func sedScriptFlag(field string) bool {
	if strings.HasPrefix(field, "--") {
		return strings.HasPrefix(field, "--expression") || strings.HasPrefix(field, "--file")
	}
	if !strings.HasPrefix(field, "-") {
		return false
	}
	return strings.ContainsAny(field[1:], "ef")
}

// literalOperand unwraps one wrapping quote pair and abstains on any remaining
// shell syntax.
func literalOperand(field string) (string, bool) {
	operand := strings.Trim(field, "'\"")
	if strings.ContainsAny(operand, shellSyntax) {
		return "", false
	}
	return operand, operand != ""
}

// planned is true when the packet already carried the path, either exactly or
// as a directory prefix.
func planned(packetPaths []string, relative string) bool {
	for _, candidate := range packetPaths {
		entry := cleanPacketPath(candidate)
		if entry == "" {
			continue
		}
		if relative == entry || strings.HasPrefix(relative, entry+"/") {
			return true
		}
	}
	return false
}

func cleanPacketPath(candidate string) string {
	entry := strings.Trim(filepath.ToSlash(filepath.Clean(candidate)), "/")
	if entry == "." {
		return ""
	}
	return entry
}

// PacketDigest names the packet a read was judged against without retaining
// its paths.
func PacketDigest(packetPaths []string) string {
	sorted := append([]string(nil), packetPaths...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:8])
}

// storedSpelling rewrites each existing component of a contained path to the
// name its directory stores, so a case-insensitive volume cannot split one file
// across spellings. An existing component matched by no stored name, such as a
// Unicode normalization alias, abstains; a verified-absent suffix keeps its spelling.
func storedSpelling(root, relative string) (string, bool) {
	directory, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", false
	}
	components := strings.Split(relative, "/")
	for index, component := range components {
		info, err := os.Lstat(filepath.Join(directory, component))
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return "", false
		}
		name, found := storedName(directory, component, info)
		if !found {
			return "", false
		}
		components[index] = name
		directory = filepath.Join(directory, name)
	}
	return strings.Join(components, "/"), true
}

// storedName prefers the exact entry name, then a case-folded entry naming the same file.
func storedName(directory, component string, info os.FileInfo) (string, bool) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if entry.Name() == component {
			return component, true
		}
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), component) {
			continue
		}
		candidate, err := os.Lstat(filepath.Join(directory, entry.Name()))
		if err == nil && os.SameFile(info, candidate) {
			return entry.Name(), true
		}
	}
	return "", false
}

func worktreeSize(root, relative string) (int64, bool) {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil || info.IsDir() {
		return 0, false
	}
	return info.Size(), true
}

// Enabled reports whether the operator created the opt-in marker.
func Enabled(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".corvint", markerName))
	return err == nil
}

// ledgerIgnored applies the self-observation ledger's conservative ignore
// check to the marker, the ledger and its temporaries.
func ledgerIgnored(root string) bool {
	return observations.CorvintEntriesIgnored(root, markerName, ledgerName, temporaryGlob)
}

// Enable and Disable are the only mutations in this package that a read
// command never performs; they are explicit operator verbs.
func Enable(root string) error {
	if !ledgerIgnored(root) {
		return errLedgerNotIgnored
	}
	directory, err := ledgerDirectory(root)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, markerName), nil, 0o600)
}

func Disable(root string) error {
	if err := os.Remove(filepath.Join(root, ".corvint", markerName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Append writes one row, and only when the opt-in marker exists. Without the
// marker it is a no-op returning nil, so no default install ever gains a file.
func Append(root string, event Event) error {
	if !Enabled(root) {
		return nil
	}
	if event.Planned && event.Path != "" {
		return errPlannedRowPath
	}
	if screened, hit := secretscreen.Screen(event.Path); hit {
		event.Path = screened
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return appendRow(root, encoded)
}

// appendRow writes one encoded row under the ledger bounds. The caller has
// already checked the marker.
func appendRow(root string, encoded []byte) error {
	if len(encoded)+1 > maxRowBytes {
		return errRowBound
	}
	if !ledgerIgnored(root) {
		return errLedgerNotIgnored
	}
	directory, err := ledgerDirectory(root)
	if err != nil {
		return err
	}
	path := filepath.Join(directory, ledgerName)
	appendProcessLock.Lock()
	defer appendProcessLock.Unlock()
	directoryLock, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer directoryLock.Close()
	if err := lockLedgerFile(directoryLock); err != nil {
		return err
	}
	defer unlockLedgerFile(directoryLock)
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errLedgerNotRegular
	}
	var previous []byte
	ledger, err := os.Open(path)
	if err == nil {
		defer ledger.Close()
		info, statErr := ledger.Stat()
		if statErr != nil {
			return statErr
		}
		previous, err = readPreviousRows(ledger, info.Size(), len(encoded)+1)
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return replaceLedger(directory, path, append(append(previous, encoded...), '\n'))
}

// ledgerDirectory creates .corvint and refuses it unless it is a real directory:
// a symlinked .corvint would move the marker, the ledger replacement and the
// temporary sweep outside the worktree.
func ledgerDirectory(root string) (string, error) {
	directory := filepath.Join(root, ".corvint")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	if info, err := os.Lstat(directory); err != nil || !info.IsDir() {
		return "", errLedgerNotRegular
	}
	return directory, nil
}

func readPreviousRows(reader io.ReaderAt, size int64, appendedBytes int) ([]byte, error) {
	readOffset := int64(0)
	readBytes := size
	rotating := size > int64(maxFileBytes-appendedBytes)
	if rotating {
		readBytes = maxFileBytes / 2
		readOffset = size - readBytes
	}
	previous := make([]byte, readBytes)
	if _, err := io.ReadFull(io.NewSectionReader(reader, readOffset, readBytes), previous); err != nil {
		return nil, err
	}
	if rotating && readOffset > 0 {
		if next := bytes.IndexByte(previous, '\n'); next >= 0 {
			previous = append([]byte(nil), previous[next+1:]...)
		}
	}
	return previous, nil
}

// HookPostTool is the orchestrator entry point. It is log-and-drop: it never
// returns an error that could fail a host hook, and the dropped error is
// persisted as an error row the digest reports as LastError.
func HookPostTool(root string, packetPaths []string, payload map[string]any) error {
	if !Enabled(root) {
		return nil
	}
	isPlanned := func(relative string) bool { return planned(packetPaths, relative) }
	hookClassified(root, isPlanned, PacketDigest(packetPaths), payload)
	return nil
}

// hookClassified resolves relative targets from the payload's absolute cwd,
// which the host moves after a `cd`.
func hookClassified(root string, isPlanned func(string) bool, digest string, payload map[string]any) {
	name, _ := payload["tool_name"].(string)
	input, _ := payload["tool_input"].(map[string]any)
	if input == nil {
		return
	}
	cwd, _ := payload["cwd"].(string)
	if !filepath.IsAbs(cwd) {
		cwd = ""
	}
	event, ok := classifyWith(isPlanned, digest, name, input, root, cwd)
	if !ok {
		return
	}
	dropError(root, Append(root, event))
}

// errorRow records one dropped hook error in the ledger itself, so a digest in
// another process sees it (URE-V0-006). It carries a closed reason code, never
// the error text, which could name a path.
type errorRow struct {
	Timestamp string `json:"ts"`
	Kind      string `json:"kind"`
	Reason    string `json:"reason"`
}

const errorKind = "error"

func errorReason(err error) string {
	switch {
	case errors.Is(err, errRowBound):
		return "row-exceeds-bound"
	case errors.Is(err, errPlannedRowPath):
		return "planned-row-path"
	}
	return "write-failed"
}

// dropError appends the error row best-effort: when the ledger itself cannot
// be written, the error row cannot be either, and the failure stays dropped.
func dropError(root string, err error) {
	if err == nil {
		return
	}
	row := errorRow{Timestamp: time.Now().UTC().Format(time.RFC3339), Kind: errorKind, Reason: errorReason(err)}
	encoded, _ := json.Marshal(row)
	_ = appendRow(root, encoded)
}

// packetRow is the host adapter's record of the packet it delivered to one
// session (URE-V0-008). It carries truncated hashes of the packet paths, never
// the paths, so the ledger still stores no packet path list.
type packetRow struct {
	Timestamp    string   `json:"ts"`
	Kind         string   `json:"kind"`
	Session      string   `json:"session"`
	PacketDigest string   `json:"packet"`
	PathHashes   []string `json:"paths_sha256"`
	// Refused marks a packet whose row could not be written: the session's
	// older packet no longer describes what was delivered.
	Refused bool `json:"refused,omitempty"`
}

const packetKind = "packet"

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

// RecordPacket stores the planned set of the packet just delivered to a
// session, so a later post-tool call in that session has a denominator. It is
// marker-gated and log-and-drop like HookPostTool: it never returns an error.
func RecordPacket(root, session string, packetPaths []string) error {
	if !Enabled(root) || session == "" {
		return nil
	}
	hashes := make([]string, 0, len(packetPaths))
	for _, candidate := range packetPaths {
		if entry := cleanPacketPath(candidate); entry != "" {
			hashes = append(hashes, shortHash(entry))
		}
	}
	row := packetRow{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Kind:         packetKind,
		Session:      shortHash(session),
		PacketDigest: PacketDigest(packetPaths),
		PathHashes:   hashes,
	}
	encoded, err := json.Marshal(row)
	if err == nil {
		err = appendRow(root, encoded)
	}
	if err == nil {
		return nil
	}
	dropError(root, err)
	refused, _ := json.Marshal(packetRow{Timestamp: row.Timestamp, Kind: packetKind, Session: row.Session, PacketDigest: row.PacketDigest, Refused: true})
	_ = appendRow(root, refused)
	return nil
}

// RefusePacket records that a session's newest prompt delivered no packet, so
// later reads in that session abstain rather than score against its older
// packet (URE-V0-008). It is marker-gated and log-and-drop like RecordPacket.
func RefusePacket(root, session string) error {
	if !Enabled(root) || session == "" {
		return nil
	}
	refused, _ := json.Marshal(packetRow{Timestamp: time.Now().UTC().Format(time.RFC3339), Kind: packetKind, Session: shortHash(session), PacketDigest: PacketDigest(nil), Refused: true})
	dropError(root, appendRow(root, refused))
	return nil
}

// HookPostToolSession is the host adapter's post-tool entry point. It judges a
// read against the newest packet recorded for the same session; when no such
// packet is retained it abstains and writes nothing, because a read with no
// known packet cannot be called unplanned (URE-V0-009). It never returns an
// error.
func HookPostToolSession(root, session string, payload map[string]any) error {
	if !Enabled(root) || session == "" {
		return nil
	}
	packet, found := newestPacket(root, shortHash(session))
	if !found {
		return nil
	}
	if packet.Refused {
		return nil
	}
	hashes := make(map[string]bool, len(packet.PathHashes))
	for _, hash := range packet.PathHashes {
		hashes[hash] = true
	}
	hookClassified(root, func(relative string) bool { return plannedByHash(hashes, relative) }, packet.PacketDigest, payload)
	return nil
}

// plannedByHash is planned over hashed packet paths: the target or any of its
// ancestor directories must be a packet path.
func plannedByHash(hashes map[string]bool, relative string) bool {
	parts := strings.Split(relative, "/")
	for count := 1; count <= len(parts); count++ {
		if hashes[shortHash(strings.Join(parts[:count], "/"))] {
			return true
		}
	}
	return false
}

func newestPacket(root, sessionHash string) (packetRow, bool) {
	file, err := openLedger(root)
	if err != nil {
		return packetRow{}, false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return packetRow{}, false
	}
	length := info.Size()
	offset := int64(0)
	truncated := length > maxFileBytes
	if truncated {
		offset = length - maxFileBytes
		length = maxFileBytes
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(io.NewSectionReader(file, offset, length), data); err != nil {
		return packetRow{}, false
	}
	if truncated {
		boundary := bytes.IndexByte(data, '\n')
		if boundary < 0 {
			return packetRow{}, false
		}
		data = data[boundary+1:]
	}
	var newest packetRow
	found := false
	for _, line := range bytes.Split(data, []byte("\n")) {
		var row packetRow
		if json.Unmarshal(line, &row) != nil || row.Kind != packetKind || row.Session != sessionHash {
			continue
		}
		newest, found = row, true
	}
	return newest, found
}

// openLedger opens the ledger for reading only when .corvint is a real directory
// and the ledger a regular file, so no digest or packet lookup follows a link
// outside the worktree. A missing entry returns its not-exist error.
func openLedger(root string) (*os.File, error) {
	directory := filepath.Join(root, ".corvint")
	directoryInfo, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !directoryInfo.IsDir() {
		return nil, errLedgerNotRegular
	}
	path := filepath.Join(directory, ledgerName)
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !linkInfo.Mode().IsRegular() {
		return nil, errLedgerNotRegular
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err == nil && os.SameFile(linkInfo, opened) {
		return file, nil
	}
	file.Close()
	return nil, errLedgerNotRegular
}

// PathBytes is one retained path and the bytes read through it.
type PathBytes struct {
	Path  string
	Bytes int64
}

// Digest is the read-only summary of the ledger.
type Digest struct {
	Unplanned   int
	Planned     int
	TotalBytes  int64
	Tools       map[string]int
	TopPaths    []PathBytes
	SkippedRows int
	// LastError is the reason code of the newest retained error row and
	// DroppedErrors counts every retained error row.
	LastError     string
	DroppedErrors int
}

// Ratio is the unplanned share of all recorded reads, rendered to three
// decimals, or "n/a" when nothing was recorded.
func (d Digest) Ratio() string {
	all := d.Unplanned + d.Planned
	if all == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.3f", float64(d.Unplanned)/float64(all))
}

// Read folds the ledger into a digest. It writes nothing.
func Read(root string) (Digest, error) {
	result := Digest{Tools: map[string]int{}}
	file, err := openLedger(root)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	defer file.Close()
	bytesByPath := map[string]int64{}
	reader := bufio.NewReader(io.LimitReader(file, maxFileBytes+1))
	for {
		row, readErr := reader.ReadBytes('\n')
		if trimmed := bytes.TrimSuffix(row, []byte("\n")); len(trimmed) > 0 {
			if len(trimmed) > maxRowBytes {
				result.SkippedRows++
			} else {
				foldRow(&result, bytesByPath, trimmed)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				result.TopPaths = rankPaths(bytesByPath)
				return result, nil
			}
			return result, readErr
		}
	}
}

func foldRow(result *Digest, bytesByPath map[string]int64, row []byte) {
	var probe struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
	}
	if json.Unmarshal(row, &probe) != nil {
		return
	}
	if probe.Kind == errorKind && probe.Reason != "" {
		result.LastError = probe.Reason
		result.DroppedErrors++
		return
	}
	// Packet rows and any unrecognized kind are not reads.
	if probe.Kind != "" {
		return
	}
	var event Event
	if json.Unmarshal(row, &event) != nil {
		return
	}
	if event.Planned {
		result.Planned++
		return
	}
	result.Unplanned++
	result.TotalBytes += event.Bytes
	result.Tools[event.Tool]++
	bytesByPath[event.Path] += event.Bytes
}

func rankPaths(bytesByPath map[string]int64) []PathBytes {
	ranked := make([]PathBytes, 0, len(bytesByPath))
	for path, size := range bytesByPath {
		ranked = append(ranked, PathBytes{Path: path, Bytes: size})
	}
	sort.Slice(ranked, func(a, b int) bool {
		if ranked[a].Bytes != ranked[b].Bytes {
			return ranked[a].Bytes > ranked[b].Bytes
		}
		return ranked[a].Path < ranked[b].Path
	})
	return ranked
}

// Render writes the digest, mirroring the self-observation digest's shape.
func Render(root string, limit int, output io.Writer) error {
	if limit < 1 {
		return fmt.Errorf("limit must be positive")
	}
	data, err := Read(root)
	if err != nil {
		return err
	}
	lines := []string{fmt.Sprintf("UNPLANNED-READS enabled=%t unplanned=%d planned=%d ratio=%s bytes=%d", Enabled(root), data.Unplanned, data.Planned, data.Ratio(), data.TotalBytes)}
	if data.SkippedRows > 0 {
		lines = append(lines, fmt.Sprintf("SKIPPED-ROWS count=%d oversized", data.SkippedRows))
	}
	if data.LastError != "" {
		lines = append(lines, fmt.Sprintf("LAST-ERROR reason=%s count=%d", data.LastError, data.DroppedErrors))
	}
	for _, tool := range sortedKeys(data.Tools) {
		lines = append(lines, fmt.Sprintf("TOOL name=%s count=%d", tool, data.Tools[tool]))
	}
	for _, entry := range data.TopPaths {
		lines = append(lines, fmt.Sprintf("PATH bytes=%d %s", entry.Bytes, entry.Path))
	}
	for _, line := range lines[:min(limit, min(120, len(lines)))] {
		if _, err := fmt.Fprintln(output, line); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys(counts map[string]int) []string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// removeStaleTemporaries clears temporaries left by a writer that died between
// write and rename. The caller holds the directory lock, so every temporary
// present here belongs to a dead process; a stale one would keep the tree dirty.
func removeStaleTemporaries(directory string) {
	stale, _ := filepath.Glob(filepath.Join(directory, temporaryGlob))
	for _, path := range stale {
		os.Remove(path)
	}
}

func replaceLedger(directory, path string, data []byte) error {
	removeStaleTemporaries(directory)
	temporary, err := os.CreateTemp(directory, temporaryGlob)
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	defer temporary.Close()
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	directoryFile, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer directoryFile.Close()
	return directoryFile.Sync()
}

func keepNewest(data []byte, maximum int) []byte {
	if len(data) <= maximum {
		return data
	}
	start := len(data) - maximum
	if next := bytes.IndexByte(data[start:], '\n'); next >= 0 {
		start += next + 1
	}
	return append([]byte(nil), data[start:]...)
}
