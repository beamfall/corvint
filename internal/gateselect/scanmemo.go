package gateselect

// The scan memo behind AFP-V0-822: what each Go source contributed to the last
// index, pinned by its stat stamp, so an unchanged file is not read again. It
// also holds the gated root-locator literal checks of AFP-V0-823 under the
// same stamps.

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"strings"
)

// ScanMemo persists per-file scan results between indexes. The caller owns
// placement, integrity and the stamp; every field must be set, and any
// failure only rescans.
type ScanMemo struct {
	// Identity names the exact program scanning; a memo written by any other
	// program is not trusted.
	Identity string
	// Load returns the stored memo body, or nil.
	Load func() []byte
	// Store replaces the stored memo body.
	Store func([]byte)
	// Stamp is a file's stat identity, or false when the file cannot be
	// pinned: no change time on this platform, or a change too recent to
	// tell from the bytes read.
	Stamp func(fs.FileInfo) (string, bool)
}

type scanMemoFile struct {
	Root     string                    `json:"root"`
	Module   string                    `json:"module"`
	Identity string                    `json:"identity"`
	Files    map[string]scanMemoEntry  `json:"files"`
	Gated    map[string]gatedMemoEntry `json:"gated,omitempty"`
}

type scanMemoEntry struct {
	Stamp          string     `json:"stamp"`
	Imports        []string   `json:"imports,omitempty"`
	Calls          []string   `json:"calls,omitempty"`
	Untokenized    bool       `json:"untokenized,omitempty"`
	Embeds         bool       `json:"embeds,omitempty"`
	MovesDirectory bool       `json:"movesDirectory,omitempty"`
	Tokens         [][]string `json:"tokens,omitempty"`
}

// gatedMemoEntry is one literal check of carriesLiteral: whether the file,
// pinned by Stamp, holds the quoted Literal.
type gatedMemoEntry struct {
	Stamp   string `json:"stamp"`
	Literal string `json:"literal"`
	Carries bool   `json:"carries,omitempty"`
}

// errRecalledUntokenized stands for the lexer error of a recalled file; the
// index reads only that there was one.
var errRecalledUntokenized = errors.New("recalled: does not tokenize")

// scanRecall is one index's view of the memo; a nil recall scans every file.
type scanRecall struct {
	memo  *ScanMemo
	files map[string]scanMemoEntry
	gated map[string]gatedMemoEntry
	// checked holds this index's literal checks, recalled or read.
	checked map[string]gatedMemoEntry
}

func (memo *ScanMemo) load(root, module string) *scanRecall {
	if memo == nil || memo.Identity == "" || memo.Load == nil || memo.Store == nil || memo.Stamp == nil {
		return nil
	}
	recall := &scanRecall{memo: memo, checked: map[string]gatedMemoEntry{}}
	var stored scanMemoFile
	if body := memo.Load(); body != nil && json.Unmarshal(body, &stored) == nil &&
		stored.Root == root && stored.Module == module && stored.Identity == memo.Identity {
		recall.files, recall.gated = stored.Files, stored.Gated
	}
	return recall
}

// scan returns the memo's result for a file whose stamp it pins unchanged,
// and otherwise reads and scans the file.
func (recall *scanRecall) scan(root, module string, source indexedFile) scannedFile {
	if recall == nil {
		return scanFile(root, module, source)
	}
	stamp, pinned := recall.memo.Stamp(source.expected)
	if !pinned {
		return scanFile(root, module, source)
	}
	if entry, ok := recall.files[source.relative]; ok && entry.Stamp == stamp {
		scanned := scannedFile{relative: source.relative, directory: parentDirectory(source.relative), test: strings.HasSuffix(source.relative, "_test.go"),
			imports: entry.Imports, tokens: entry.Tokens, stamp: stamp, recalled: true}
		scanned.scan.calls, scanned.scan.embeds, scanned.scan.movesDirectory = entry.Calls, entry.Embeds, entry.MovesDirectory
		if entry.Untokenized {
			scanned.scan.err = errRecalledUntokenized
		}
		return scanned
	}
	scanned := scanFile(root, module, source)
	scanned.stamp = stamp
	return scanned
}

// carries reports whether the file holds literal, from the memo when its stamp
// pins the file unchanged since the same check, and otherwise from its bytes
// (AFP-V0-823). A read error is not recorded.
func (recall *scanRecall) carries(root string, source indexedFile, literal []byte) (bool, error) {
	if recall == nil {
		return readCarries(root, source, literal)
	}
	stamp, pinned := recall.memo.Stamp(source.expected)
	if !pinned {
		return readCarries(root, source, literal)
	}
	if entry, ok := recall.gated[source.relative]; ok && entry.Stamp == stamp && entry.Literal == string(literal) {
		recall.checked[source.relative] = entry
		return entry.Carries, nil
	}
	carries, err := readCarries(root, source, literal)
	if err == nil {
		recall.checked[source.relative] = gatedMemoEntry{Stamp: stamp, Literal: string(literal), Carries: carries}
	}
	return carries, err
}

// store writes the memo of this index's results unless every file was
// recalled, none was dropped and every literal check was recalled. It runs
// only after a successful scan and the gated root-locator pass.
func (memo *ScanMemo) store(root, module string, recall *scanRecall, results []scannedFile) {
	if recall == nil {
		return
	}
	files := make(map[string]scanMemoEntry, len(results))
	fresh := false
	for _, scanned := range results {
		if scanned.stamp == "" {
			continue
		}
		fresh = fresh || !scanned.recalled
		files[scanned.relative] = scanMemoEntry{Stamp: scanned.stamp, Imports: scanned.imports, Calls: scanned.scan.calls,
			Untokenized: scanned.scan.err != nil, Embeds: scanned.scan.embeds, MovesDirectory: scanned.scan.movesDirectory, Tokens: scanned.tokens}
	}
	if !fresh && maps.EqualFunc(files, recall.files, func(a, b scanMemoEntry) bool { return a.Stamp == b.Stamp }) &&
		maps.Equal(recall.checked, recall.gated) {
		return
	}
	var gated map[string]gatedMemoEntry
	if len(recall.checked) != 0 {
		gated = recall.checked
	}
	body, err := json.Marshal(scanMemoFile{Root: root, Module: module, Identity: memo.Identity, Files: files, Gated: gated})
	if err == nil {
		memo.Store(body)
	}
}
