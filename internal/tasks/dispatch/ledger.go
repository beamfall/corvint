package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	maxLedger      = 16 << 20
	maxEventsBytes = 16 << 20
	maxSummary     = 2000
)

// Proc is one identity-verified process.
type Proc struct {
	PID      int    `json:"pid"`
	Identity string `json:"identity"`
}

// Worker is one launched host process and its supervised tree.
type Worker struct {
	ID             string    `json:"id"`
	Role           string    `json:"role"`
	Host           string    `json:"host"`
	Slot           int       `json:"slot"`
	Key            string    `json:"key"`
	Ticket         string    `json:"ticket,omitempty"`
	Pool           string    `json:"pool,omitempty"`
	Member         string    `json:"member,omitempty"`
	PID            int       `json:"pid"`
	LeaderIdentity string    `json:"leaderIdentity"`
	Members        []Proc    `json:"members"`
	Started        time.Time `json:"started"`
	LastActive     time.Time `json:"lastActive"`
	LogBytes       int64     `json:"logBytes"`
	ActivityPaths  []string  `json:"activityPaths,omitempty"`
	ActivityMtime  time.Time `json:"activityMtime"`
	State          string    `json:"state"` // RUNNING or KILLING
	KillReason     string    `json:"killReason,omitempty"`
	KillDeadline   time.Time `json:"killDeadline,omitempty"`
	Fingerprint    string    `json:"fingerprint"`
}

// Backoff is the CAL-V0-057 per-key no-progress record.
type BackoffState struct {
	NoProgress    int       `json:"noProgress"`
	CooldownUntil time.Time `json:"cooldownUntil"`
	Parked        bool      `json:"parked"`
	Fingerprint   string    `json:"fingerprint"`
}

// Seen is the previous observation, kept to emit change events.
type Seen struct {
	Tickets map[string]string `json:"tickets"`
	Claims  map[string]string `json:"claims"`
	Lanes   map[string]string `json:"lanes"`
}

// Ledger is the dispatcher's private taskman-dispatch-state/0 file. It is
// never an input to the native store.
type Ledger struct {
	Profile   string                   `json:"profile"`
	Program   string                   `json:"program"`
	LaunchSeq uint64                   `json:"launchSeq"`
	EventSeq  uint64                   `json:"eventSeq"`
	Workers   []*Worker                `json:"workers"`
	Backoff   map[string]*BackoffState `json:"backoff"`
	Seen      *Seen                    `json:"seen,omitempty"`
}

// ProgramDir is the dispatcher's state directory for one program.
func ProgramDir(c *Config, program string) string { return filepath.Join(c.StateDir, program) }

// LoadLedger reads the ledger; a missing ledger is a fresh one.
func LoadLedger(dir, program string) (*Ledger, error) {
	raw, err := readBounded(filepath.Join(dir, "state.json"), maxLedger)
	if errors.Is(err, fs.ErrNotExist) {
		return &Ledger{Profile: StateProfile, Program: program, Workers: []*Worker{}, Backoff: map[string]*BackoffState{}}, nil
	}
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var l Ledger
	if err := d.Decode(&l); err != nil {
		return nil, fmt.Errorf("dispatch state: %w", err)
	}
	if l.Profile != StateProfile || l.Program != program {
		return nil, fmt.Errorf("dispatch state belongs to profile %q program %q", l.Profile, l.Program)
	}
	if l.Backoff == nil {
		l.Backoff = map[string]*BackoffState{}
	}
	if l.Workers == nil {
		l.Workers = []*Worker{}
	}
	return &l, nil
}

func (l *Ledger) save(dir string) error {
	raw, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "state.json"), append(raw, '\n'))
}

func writeAtomic(path string, raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func readBounded(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > max {
		return nil, fmt.Errorf("%s exceeds %d bytes", path, max)
	}
	return raw, nil
}

// Event is one taskman-dispatch-event/0 line. Message is plain language.
type Event struct {
	Profile string            `json:"profile"`
	Seq     uint64            `json:"seq"`
	At      string            `json:"at"`
	Program string            `json:"program"`
	Kind    string            `json:"kind"`
	Ticket  string            `json:"ticket,omitempty"`
	Role    string            `json:"role,omitempty"`
	Worker  string            `json:"worker,omitempty"`
	Message string            `json:"message"`
	Detail  map[string]string `json:"detail,omitempty"`
}

// EventKinds is the closed CAL-V0-058 event vocabulary.
var EventKinds = []string{"started", "stopped", "adopted", "launched", "launch-failed", "finished", "killing", "killed", "handoff", "handoff-refused", "reaped", "state", "claim", "release", "lane", "cooldown", "parked", "unparked", "alert", "needs-owner"}

// appendEvent writes one event line, rotating the log once at 16 MiB.
func appendEvent(dir string, e Event) error {
	path := filepath.Join(dir, "events.jsonl")
	if st, err := os.Stat(path); err == nil && st.Size() > maxEventsBytes {
		if err := os.Rename(path, path+".1"); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(raw, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ReadEvents returns the last n events of the current log.
func ReadEvents(dir string, n int) ([]Event, error) {
	raw, err := readTail(filepath.Join(dir, "events.jsonl"), 1<<20)
	if errors.Is(err, fs.ErrNotExist) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	out := []Event{}
	for _, line := range lines {
		var e Event
		if json.Unmarshal([]byte(line), &e) == nil && e.Profile == EventProfile {
			out = append(out, e)
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out, nil
}

func readTail(path string, n int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	off := st.Size() - n
	if off < 0 {
		off = 0
	}
	raw := make([]byte, st.Size()-off)
	_, err = f.ReadAt(raw, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if off > 0 {
		if i := bytes.IndexByte(raw, '\n'); i >= 0 {
			raw = raw[i+1:]
		}
	}
	return raw, nil
}

// Summary is the sanitized last part of a worker's stdout.
func Summary(logDir string) string {
	raw, err := readTail(filepath.Join(logDir, "stdout.log"), 64<<10)
	if err != nil {
		return ""
	}
	text := extractText(raw)
	text = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) || r == unicode.ReplacementChar {
			return -1
		}
		return r
	}, strings.TrimSpace(text))
	if len(text) > maxSummary {
		text = text[len(text)-maxSummary:]
		for len(text) > 0 && !utf8Start(text[0]) {
			text = text[1:]
		}
	}
	return text
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// extractText returns the agent's final text when stdout is a JSONL event
// stream (opencode --format json, codex exec --json, claude -p
// --output-format stream-json --verbose, or json without --verbose); otherwise
// the raw tail.
func extractText(raw []byte) string {
	last := ""
	jsonl := false
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var v map[string]any
		if json.Unmarshal(line, &v) != nil {
			return string(raw)
		}
		jsonl = true
		if t := textOf(v); t != "" {
			last = t
		}
	}
	if jsonl && last != "" {
		return last
	}
	return string(raw)
}

// textOf finds a text payload in the known host event shapes: opencode
// {"type":"text","part":{"text":...}}, codex {"item":{"type":"agent_message","text":...}},
// and claude {"type":"result","result":...} or the last text block of a top-level
// {"type":"assistant","message":{"content":[{"type":"text","text":...}]}}.
func textOf(v map[string]any) string {
	switch v["type"] {
	case "result":
		if s, ok := v["result"].(string); ok {
			return s
		}
	case "assistant":
		if v["parent_tool_use_id"] != nil {
			return "" // a subagent's message, not the worker's own
		}
		msg, _ := v["message"].(map[string]any)
		blocks, _ := msg["content"].([]any)
		last := ""
		for _, b := range blocks {
			if c, ok := b.(map[string]any); ok && c["type"] == "text" {
				if s, ok := c["text"].(string); ok && s != "" {
					last = s
				}
			}
		}
		return last
	}
	if part, ok := v["part"].(map[string]any); ok && v["type"] == "text" {
		if s, ok := part["text"].(string); ok {
			return s
		}
	}
	if item, ok := v["item"].(map[string]any); ok && item["type"] == "agent_message" {
		if s, ok := item["text"].(string); ok {
			return s
		}
	}
	return ""
}
