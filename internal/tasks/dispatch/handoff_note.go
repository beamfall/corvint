package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// HandoffNoteProfile is the profile of the ATR-V0-104 salvaged note.
const HandoffNoteProfile = "taskman-dispatch-handoff-note/0"

const (
	handoffNoteFile = "handoff-note.json"
	evidenceDirName = "evidence"
	// Bounds of one note (ATR-V0-104).
	maxHandoffNote   = 64 << 10
	maxNoteMessages  = 3
	maxNoteMessage   = 16 << 10
	maxNoteFiles     = 8
	maxNoteFileBytes = 8 << 10
	maxNoteScan      = 256
	noteLogWindow    = 256 << 10
	// redactedLine replaces a line the secret screen matches.
	redactedLine = "[REDACTED]"
	// handoffNoteEnv names the ATR-V0-105 note for the next launch.
	handoffNoteEnv = "CORVINT_DISPATCH_HANDOFF_NOTE"
	// handoffEvidencePrefix is the dispatcher's hand-off evidence reference.
	handoffEvidencePrefix = "dispatch:"
)

// NoteFile is one salvaged evidence file: its name, its size and its last
// bytes, cut when the file was longer.
type NoteFile struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
	Cut   bool   `json:"cut"`
	Text  string `json:"text"`
}

// HandoffNote is the ATR-V0-104 note a context-stopped worker leaves for the
// next attempt on its ticket. It is untrusted agent output.
type HandoffNote struct {
	Profile       string     `json:"profile"`
	Worker        string     `json:"worker"`
	Ticket        string     `json:"ticket"`
	Role          string     `json:"role"`
	At            string     `json:"at"`
	ContextTokens string     `json:"contextTokens"`
	Messages      []string   `json:"messages"`
	Files         []NoteFile `json:"files"`
	RedactedLines int        `json:"redactedLines"`
	Truncated     bool       `json:"truncated"`
	// Commit is the full commit id the checkpoint was taken at, when the
	// note's writer knows it; absent otherwise, so a note without it keeps
	// its exact earlier bytes (ERG-V0-026).
	Commit string `json:"commit,omitempty"`
}

func (n *HandoffNote) encode() []byte {
	raw, _ := json.Marshal(n)
	return append(raw, '\n')
}

// EvidenceDir is a worker's ATR-V0-104 evidence directory.
func EvidenceDir(dir, worker string) string {
	return filepath.Join(dir, "workers", worker, evidenceDirName)
}

// HandoffNotePath is a worker's ATR-V0-104 note.
func HandoffNotePath(dir, worker string) string {
	return filepath.Join(dir, "workers", worker, handoffNoteFile)
}

// screen replaces every line the secret screen matches and counts them.
// Two passes also replace the lines of a secret wrapped across lines
// (ATR-V0-169): the whole text, line breaks kept, catches a PEM or PGP block;
// each run of consecutive lines that do not match alone, joined with their
// surrounding blanks and line breaks removed, catches a token a model broke
// across lines. Each pass replaces every non-empty line a match spanning
// more than one line touches.
func screen(s string) (string, int) {
	s = strings.ToValidUTF8(s, "\ufffd")
	lines := strings.Split(s, "\n")
	alone := make([]bool, len(lines))
	for i, l := range lines {
		alone[i] = obligation.MatchesSecretScreen(l)
	}
	hit := slices.Clone(alone)
	markWrapped(hit, 0, lines, "\n")
	for i := 0; i < len(lines); {
		j := i
		for j < len(lines) && !alone[j] {
			j++
		}
		if j-i > 1 {
			run := make([]string, j-i)
			for k := range run {
				run[k] = strings.Trim(lines[i+k], " \t\r")
			}
			markWrapped(hit, i, run, "")
		}
		i = j + 1
	}
	n := 0
	for i := range lines {
		if hit[i] {
			lines[i], n = redactedLine, n+1
		}
	}
	return strings.Join(lines, "\n"), n
}

// markWrapped screens pieces joined by sep and sets hit[first+i] for every
// non-empty piece i that a match touching more than one non-empty piece
// overlaps.
func markWrapped(hit []bool, first int, pieces []string, sep string) {
	starts := make([]int, len(pieces))
	at := 0
	for i, p := range pieces {
		if i > 0 {
			at += len(sep)
		}
		starts[i] = at
		at += len(p)
	}
	for _, span := range obligation.SecretScreenSpans(strings.Join(pieces, sep)) {
		var touched []int
		for i, p := range pieces {
			if p != "" && starts[i] < span[1] && span[0] < starts[i]+len(p) {
				touched = append(touched, i)
			}
		}
		if len(touched) > 1 {
			for _, i := range touched {
				hit[first+i] = true
			}
		}
	}
}

// boundScreened screens s whole, then keeps at most limit bytes of its tail
// on line boundaries (ATR-V0-104), so a cut never separates a secret from
// the text that lets the screen recognize it. partial marks s read from a
// bounded tail: its first line may be incomplete and is dropped unscreened.
// cut reports that anything was dropped; n counts redacted lines.
func boundScreened(s string, partial bool, limit int) (out string, cut bool, n int) {
	cut = partial
	if partial {
		_, s, _ = strings.Cut(s, "\n")
	}
	s, n = screen(s)
	for len(s) > limit {
		cut = true
		if _, rest, ok := strings.Cut(s, "\n"); ok {
			s = rest
			continue
		}
		// One line longer than the bound keeps its tail, screened again
		// because a fragment can match where the whole line did not.
		s = s[len(s)-limit:]
		for len(s) > 0 && !utf8Start(s[0]) {
			s = s[1:]
		}
		var m int
		s, m = screen(s)
		n += m
	}
	return s, cut, n
}

// workerFile opens rel beneath the state directory dir without following a
// symlink in any component below dir or blocking on a FIFO, and requires a
// regular file or, with directory, a directory (ATR-V0-104).
func workerFile(dir, rel string, directory bool) (*os.File, os.FileInfo, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	f, err := safeopen.InRoot(root, rel, os.O_RDONLY, 0, directory)
	if err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err == nil && fi.IsDir() != directory || err == nil && !directory && !fi.Mode().IsRegular() {
		err = errors.New(rel + " is not a regular file")
	}
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, fi, nil
}

// tailOf reads the last n bytes of the regular file f of size size;
// partial is true when the read starts after the file's first byte.
func tailOf(f *os.File, size, n int64) ([]byte, bool, error) {
	off := max(size-n, 0)
	raw := make([]byte, size-off)
	k, err := f.ReadAt(raw, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	return raw[:k], off > 0, nil
}

// lastTexts is up to the last maxNoteMessages main-session assistant texts
// in the bounded tail of worker's stdout, whole and unscreened.
func lastTexts(dir, worker string) []string {
	f, fi, err := workerFile(dir, "workers/"+worker+"/stdout.log", false)
	if err != nil {
		return nil
	}
	defer f.Close()
	raw, partial, err := tailOf(f, fi.Size(), noteLogWindow)
	if err != nil {
		return nil
	}
	if partial {
		_, after, _ := bytes.Cut(raw, []byte("\n"))
		raw = after
	}
	var out []string
	for _, line := range bytes.Split(raw, []byte("\n")) {
		var v map[string]any
		if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &v) != nil || v["type"] == "result" {
			continue
		}
		if t := textOf(v); t != "" {
			out = append(out, t)
		}
	}
	if len(out) > maxNoteMessages {
		out = out[len(out)-maxNoteMessages:]
	}
	return out
}

// evidenceFiles reads up to maxNoteFiles regular files of worker's evidence
// directory in name order, among at most maxNoteScan entries enumerated;
// more regular files, or more entries, set truncated. The directory and
// each file are opened beneath dir without following a symlink, and each
// file's tail is screened before it is cut to maxNoteFileBytes.
func evidenceFiles(dir, worker string) (files []NoteFile, redacted int, truncated bool) {
	ev, _, err := workerFile(dir, "workers/"+worker+"/"+evidenceDirName, true)
	if err != nil {
		return nil, 0, false
	}
	defer ev.Close()
	var names []string
	scanned := 0
	for scanned < maxNoteScan {
		entries, err := ev.ReadDir(min(64, maxNoteScan-scanned))
		scanned += len(entries)
		for _, e := range entries {
			if e.Type().IsRegular() {
				names = append(names, e.Name())
			}
		}
		if err != nil || len(entries) == 0 {
			break
		}
	}
	if scanned == maxNoteScan {
		if more, _ := ev.ReadDir(1); len(more) > 0 {
			truncated = true
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if len(files) == maxNoteFiles {
			truncated = true
			break
		}
		f, err := safeopen.InDir(ev, name, os.O_RDONLY, 0)
		if err != nil {
			continue
		}
		fi, err := f.Stat()
		var raw []byte
		var partial bool
		if err == nil && fi.Mode().IsRegular() {
			raw, partial, err = tailOf(f, fi.Size(), 2*maxNoteFileBytes)
		} else if err == nil {
			err = errors.New("not a regular file")
		}
		f.Close()
		if err != nil {
			continue
		}
		text, cut, r := boundScreened(string(raw), partial, maxNoteFileBytes)
		files, redacted = append(files, NoteFile{Name: strings.ToValidUTF8(name, "\uFFFD"), Bytes: fi.Size(), Cut: cut, Text: text}), redacted+r
	}
	return files, redacted, truncated
}

// buildHandoffNote assembles worker w's bounded, screened note, naming
// commit when it is not empty.
func (d *Dispatcher) buildHandoffNote(w *Worker, now time.Time, commit string) *HandoffNote {
	n := &HandoffNote{Profile: HandoffNoteProfile, Worker: w.ID, Ticket: w.Ticket, Role: w.Role, At: now.UTC().Format(time.RFC3339), ContextTokens: UsageUnknown, Messages: []string{}, Files: []NoteFile{}, Commit: commit}
	if detail := d.contextHandoffDetail(w); detail != nil {
		n.ContextTokens = detail["contextTokens"]
	}
	for _, m := range lastTexts(d.dir, w.ID) {
		s, _, r := boundScreened(m, false, maxNoteMessage)
		n.Messages, n.RedactedLines = append(n.Messages, s), n.RedactedLines+r
	}
	files, redacted, more := evidenceFiles(d.dir, w.ID)
	if files != nil {
		n.Files = files
	}
	n.RedactedLines += redacted
	n.Truncated = more
	// The oldest texts go first, then the last files, so the note fits.
	for len(n.encode()) > maxHandoffNote {
		n.Truncated = true
		switch {
		case len(n.Messages) > 1:
			n.Messages = n.Messages[1:]
		case len(n.Files) > 0:
			n.Files = n.Files[:len(n.Files)-1]
		default:
			n.Messages = []string{}
		}
	}
	return n
}

// CheckpointCommit is the full commit id attempt a's checkout holds
// (ERG-V0-028): its worktree's HEAD when it records a worktree, otherwise
// its branch in workRoot. Anything resolve does not answer with a full
// commit id gives "", so the note names no commit and the packet's
// checkpoint stays NOT_OBSERVED.
func CheckpointCommit(resolve func(dir, rev string) (string, error), workRoot string, a Attempt) string {
	dir, rev := a.Worktree, "HEAD"
	if dir == "" {
		dir, rev = workRoot, "refs/heads/"+a.Branch
	}
	if resolve == nil || dir == "" || a.Worktree == "" && a.Branch == "" {
		return ""
	}
	c, err := resolve(dir, rev)
	if err != nil {
		return ""
	}
	if _, err := wire.ParseOID("/commit", c); err != nil {
		return ""
	}
	return c
}

// salvageCommit is the checkpoint commit of worker w's attempt in obs: the
// attempt its lease names w as holder of on w's ticket, or "" without one.
func (d *Dispatcher) salvageCommit(w *Worker, obs *Observation) string {
	if obs == nil || d.ResolveCommit == nil {
		return ""
	}
	for _, a := range obs.Attempts {
		if a.Holder == w.ID && a.Ticket == w.Ticket {
			return CheckpointCommit(d.ResolveCommit, d.Config.WorkRoot, a)
		}
	}
	return ""
}

// salvage writes the ATR-V0-104 note of worker w, stopped for CONTEXT,
// naming its attempt's commit in obs when it resolves (ERG-V0-028).
func (d *Dispatcher) salvage(w *Worker, now time.Time, obs *Observation) {
	if ev := d.saveSalvage(w, now, obs); ev != nil {
		d.emit(*ev)
	}
}

// saveSalvage is salvage's write; it returns the event reporting it, so a
// finished event can name the written note before that event (ATR-V0-351).
func (d *Dispatcher) saveSalvage(w *Worker, now time.Time, obs *Observation) *Event {
	if w.KillReason != "CONTEXT" {
		return nil
	}
	n := d.buildHandoffNote(w, now, d.salvageCommit(w, obs))
	path := HandoffNotePath(d.dir, w.ID)
	if err := writeAtomic(path, n.encode()); err != nil {
		return &Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: fmt.Sprintf("could not save the hand-off note of %s, stopped for context: %v", w.ID, err)}
	}
	return &Event{Kind: "context", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("saved the hand-off note of %s worker %s, stopped for context, at %s (%d messages, %d files, %d lines redacted)", w.Role, w.ID, path, len(n.Messages), len(n.Files), n.RedactedLines),
		Detail: map[string]string{"note": path, "messages": strconv.Itoa(len(n.Messages)), "files": strconv.Itoa(len(n.Files)), "redactedLines": strconv.Itoa(n.RedactedLines)}}
}

// SalvageWorker writes worker w's ATR-V0-104 note on demand, for a session
// that still runs or that something other than the dispatcher's hard stop
// ended (ATR-V0-126). It reuses the note's bounds and secret screen; the
// context figure comes from the worker's record when the worker was not
// stopped for context. commit, when not empty, is the full commit id the
// note names (ERG-V0-028). It returns the note's path and the note.
func SalvageWorker(dir string, w *Worker, now time.Time, commit string) (string, *HandoffNote, error) {
	d := &Dispatcher{dir: dir}
	n := d.buildHandoffNote(w, now, commit)
	if n.ContextTokens == UsageUnknown {
		if r, err := ReadContextRecord(dir, w.ID); err == nil {
			if t, ok := r.Estimate(); ok {
				n.ContextTokens = strconv.FormatUint(t, 10)
			}
		}
	}
	path := HandoffNotePath(dir, w.ID)
	if err := writeAtomic(path, n.encode()); err != nil {
		return "", nil, err
	}
	return path, n, nil
}

// ReadHandoffNote reads the note at path, refusing one that is not exactly
// its canonical encoding. Another version is UNSUPPORTED_VERSION
// (CAL-V0-131); a missing note is fs.ErrNotExist.
func ReadHandoffNote(path string) (*HandoffNote, error) {
	raw, err := readRegular(path, maxHandoffNote)
	if err != nil {
		return nil, err
	}
	if err := wire.RawProfileVersion("/profile", raw, HandoffNoteProfile); err != nil {
		return nil, err
	}
	var n HandoffNote
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&n); err != nil {
		return nil, err
	}
	if _, err := time.Parse(time.RFC3339, n.At); err != nil || n.Worker == "" || n.Ticket == "" || n.Messages == nil || n.Files == nil || len(n.Messages) > maxNoteMessages || len(n.Files) > maxNoteFiles || n.RedactedLines < 0 {
		return nil, errors.New("hand-off note is malformed")
	}
	if n.Commit != "" {
		if _, err := wire.ParseOID("/commit", n.Commit); err != nil {
			return nil, errors.New("hand-off note commit is not a full commit id")
		}
	}
	if !bytes.Equal(raw, n.encode()) {
		return nil, errors.New("hand-off note is not in canonical form")
	}
	return &n, nil
}

// readRegular reads at most max bytes of the regular file at path. Its
// open neither follows a final symlink nor blocks on a FIFO, so a FIFO
// planted at path cannot stall the dispatcher (ATR-V0-105).
func readRegular(path string, max int64) ([]byte, error) {
	f, err := openReaderMarker(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil {
		return nil, err
	} else if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	raw, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > max {
		return nil, fmt.Errorf("%s exceeds %d bytes", path, max)
	}
	return raw, nil
}

// handoffNoteFor is the newest readable ATR-V0-105 note of ticket, linked by
// an attempt's dispatcher hand-off evidence; empty without one.
func (d *Dispatcher) handoffNoteFor(obs *Observation, ticket string) string {
	best, bestAt := "", time.Time{}
	for _, a := range obs.Attempts {
		worker, ok := strings.CutPrefix(a.HandoffEvidence, handoffEvidencePrefix)
		if a.Ticket != ticket || !ok || worker == "" || worker == "." || worker == ".." || strings.ContainsAny(worker, `/\`) {
			continue
		}
		path := HandoffNotePath(d.dir, worker)
		n, err := ReadHandoffNote(path)
		if err != nil || n.Ticket != ticket || n.Worker != worker {
			continue
		}
		if at, _ := time.Parse(time.RFC3339, n.At); best == "" || at.After(bestAt) {
			best, bestAt = path, at
		}
	}
	return best
}
