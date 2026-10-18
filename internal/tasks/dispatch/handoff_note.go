package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
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
	noteLogWindow    = 256 << 10
	// redactedLine replaces a line the secret screen matches.
	redactedLine = "[REDACTED]"
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
func screen(s string) (string, int) {
	s = strings.ToValidUTF8(s, "�")
	lines := strings.Split(s, "\n")
	n := 0
	for i, l := range lines {
		if obligation.MatchesSecretScreen(l) {
			lines[i], n = redactedLine, n+1
		}
	}
	return strings.Join(lines, "\n"), n
}

// lastTexts is up to the last maxNoteMessages main-session assistant texts
// in the bounded tail of a worker's stdout.
func lastTexts(logDir string) []string {
	raw, err := readTail(filepath.Join(logDir, "stdout.log"), noteLogWindow)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range bytes.Split(raw, []byte("\n")) {
		var v map[string]any
		if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &v) != nil || v["type"] == "result" {
			continue
		}
		if t := textOf(v); t != "" {
			if len(t) > maxNoteMessage {
				t = t[len(t)-maxNoteMessage:]
				for len(t) > 0 && !utf8Start(t[0]) {
					t = t[1:]
				}
			}
			out = append(out, t)
		}
	}
	if len(out) > maxNoteMessages {
		out = out[len(out)-maxNoteMessages:]
	}
	return out
}

// tailRegular reads the last n bytes of the regular file at path, refusing
// a symlink or any other file type.
func tailRegular(path string, n int64) ([]byte, int64, error) {
	li, err := os.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	if !li.Mode().IsRegular() {
		return nil, 0, errors.New("not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !os.SameFile(li, fi) {
		return nil, 0, errors.New("the file changed while it was read")
	}
	off := max(fi.Size()-n, 0)
	raw := make([]byte, fi.Size()-off)
	if _, err := f.ReadAt(raw, off); err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, err
	}
	return raw, fi.Size(), nil
}

// evidenceFiles reads up to maxNoteFiles regular files of a worker's
// evidence directory in name order; more files set truncated.
func evidenceFiles(dir string) ([]NoteFile, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var out []NoteFile
	more := false
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if len(out) == maxNoteFiles {
			more = true
			break
		}
		raw, size, err := tailRegular(filepath.Join(dir, e.Name()), maxNoteFileBytes)
		if err != nil {
			continue
		}
		cut := size > int64(len(raw))
		for cut && len(raw) > 0 && !utf8Start(raw[0]) {
			raw = raw[1:]
		}
		out = append(out, NoteFile{Name: strings.ToValidUTF8(e.Name(), "�"), Bytes: size, Cut: cut, Text: string(raw)})
	}
	return out, more
}

// buildHandoffNote assembles worker w's bounded, screened note.
func (d *Dispatcher) buildHandoffNote(w *Worker, now time.Time) *HandoffNote {
	n := &HandoffNote{Profile: HandoffNoteProfile, Worker: w.ID, Ticket: w.Ticket, Role: w.Role, At: now.UTC().Format(time.RFC3339), ContextTokens: UsageUnknown, Messages: []string{}, Files: []NoteFile{}}
	if detail := d.contextHandoffDetail(w); detail != nil {
		n.ContextTokens = detail["contextTokens"]
	}
	for _, m := range lastTexts(d.workerDir(w.ID)) {
		s, r := screen(m)
		n.Messages, n.RedactedLines = append(n.Messages, s), n.RedactedLines+r
	}
	files, more := evidenceFiles(EvidenceDir(d.dir, w.ID))
	for _, f := range files {
		var r int
		f.Text, r = screen(f.Text)
		n.Files, n.RedactedLines = append(n.Files, f), n.RedactedLines+r
	}
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

// salvage writes the ATR-V0-104 note of worker w, stopped for CONTEXT.
func (d *Dispatcher) salvage(w *Worker, now time.Time) {
	if w.KillReason != "CONTEXT" {
		return
	}
	n := d.buildHandoffNote(w, now)
	path := HandoffNotePath(d.dir, w.ID)
	if err := writeAtomic(path, n.encode()); err != nil {
		d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: fmt.Sprintf("could not save the hand-off note of %s, stopped for context: %v", w.ID, err)})
		return
	}
	d.emit(Event{Kind: "context", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("saved the hand-off note of %s worker %s, stopped for context, at %s (%d messages, %d files, %d lines redacted)", w.Role, w.ID, path, len(n.Messages), len(n.Files), n.RedactedLines),
		Detail: map[string]string{"note": path, "messages": strconv.Itoa(len(n.Messages)), "files": strconv.Itoa(len(n.Files)), "redactedLines": strconv.Itoa(n.RedactedLines)}})
}

// ReadHandoffNote reads the note at path, refusing one that is not exactly
// its canonical encoding. Another version is UNSUPPORTED_VERSION
// (CAL-V0-131); a missing note is fs.ErrNotExist.
func ReadHandoffNote(path string) (*HandoffNote, error) {
	raw, err := readBounded(path, maxHandoffNote)
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
	if !bytes.Equal(raw, n.encode()) {
		return nil, errors.New("hand-off note is not in canonical form")
	}
	return &n, nil
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
