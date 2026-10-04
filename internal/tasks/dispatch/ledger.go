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
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Beamfall/corvint/internal/tasks/wire"
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
	ID              string    `json:"id"`
	Role            string    `json:"role"`
	Host            string    `json:"host"`
	Slot            int       `json:"slot"`
	Key             string    `json:"key"`
	Ticket          string    `json:"ticket,omitempty"`
	Pool            string    `json:"pool,omitempty"`
	Member          string    `json:"member,omitempty"`
	PID             int       `json:"pid"`
	LeaderIdentity  string    `json:"leaderIdentity"`
	Members         []Proc    `json:"members"`
	Started         time.Time `json:"started"`
	LastActive      time.Time `json:"lastActive"`
	LogBytes        int64     `json:"logBytes"`
	ActivityPaths   []string  `json:"activityPaths,omitempty"`
	ActivityMtime   time.Time `json:"activityMtime"`
	State           string    `json:"state"` // RUNNING or KILLING
	KillReason      string    `json:"killReason,omitempty"`
	KillDeadline    time.Time `json:"killDeadline,omitempty"`
	Fingerprint     string    `json:"fingerprint"`
	BaseFingerprint string    `json:"baseFingerprint,omitempty"`
	ProgressDigest  string    `json:"progressDigest,omitempty"`
}

// Backoff is the CAL-V0-057 per-key no-progress record.
type BackoffState struct {
	NoProgress      int       `json:"noProgress"`
	CooldownUntil   time.Time `json:"cooldownUntil"`
	Parked          bool      `json:"parked"`
	Fingerprint     string    `json:"fingerprint"`
	BaseFingerprint string    `json:"baseFingerprint,omitempty"`
	ProgressDigest  string    `json:"progressDigest,omitempty"`
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
	PoolSweeps map[string]*PoolSweepRecord `json:"poolSweeps,omitempty"`
	Profile    string                      `json:"profile"`
	Program    string                      `json:"program"`
	LaunchSeq  uint64                      `json:"launchSeq"`
	EventSeq   uint64                      `json:"eventSeq"`
	Workers    []*Worker                   `json:"workers"`
	Backoff    map[string]*BackoffState    `json:"backoff"`
	Seen       *Seen                       `json:"seen,omitempty"`
	Progress   map[string]*ProgressHistory `json:"progress,omitempty"`
}

const maxProgressPerKey, maxProgressProgram = 256, 8192

// ProgressHistory is lifetime replay protection, including deleted keys.
type ProgressHistory struct {
	Current string   `json:"current"`
	Seen    []string `json:"seen"`
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
	var members map[string]json.RawMessage
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&members); err != nil {
		return nil, fmt.Errorf("dispatch state: %w", err)
	}
	// Detect aliases before struct decoding: encoding/json folds field names,
	// so an uppercase-only member must not fall back to legacy loading.
	for name := range members {
		if strings.EqualFold(name, "progress") || strings.EqualFold(name, "poolSweeps") {
			if bytes.Equal(bytes.TrimSpace(members[name]), []byte("null")) || !validScalarJSON(raw) || !strictProgressJSON(raw) {
				return nil, errors.New("dispatch state: malformed progress JSON")
			}
			break
		}
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
	if err := l.validateProgress(); err != nil {
		return nil, fmt.Errorf("dispatch state: %w", err)
	}
	if err := l.validatePoolSweeps(); err != nil {
		return nil, fmt.Errorf("dispatch state: %w", err)
	}
	if l.Backoff == nil {
		l.Backoff = map[string]*BackoffState{}
	}
	if l.Workers == nil {
		l.Workers = []*Worker{}
	}
	return &l, nil
}

func validProgressDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Token-enabled ledgers require canonical struct fields and unique members.
// Dynamic map keys keep their case-sensitive identities. Container shapes
// and value types are subsequently checked by the ordinary struct decoder.
func strictProgressJSON(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	var value func(int, string) bool
	value = func(depth int, schema string) bool {
		if depth > 32 {
			return false
		}
		token, err := d.Token()
		if err != nil {
			return false
		}
		switch token {
		case json.Delim('{'):
			var fields []string
			switch schema {
			case "ledger":
				fields = []string{"profile", "program", "launchSeq", "eventSeq", "workers", "backoff", "seen", "progress", "poolSweeps"}
			case "sweep-record":
				fields = []string{"workRoot", "program", "queue", "pool", "member", "allocation", "definition", "requestId", "actor", "actorRole", "configDigest", "timeoutSeconds", "phase", "started", "observed", "result", "reason"}
			case "sweep-result":
				fields = []string{"pending", "evidence", "receipt", "receiptSeq", "outcome"}
			case "worker":
				fields = []string{"id", "role", "host", "slot", "key", "ticket", "pool", "member", "pid", "leaderIdentity", "members", "started", "lastActive", "logBytes", "activityPaths", "activityMtime", "state", "killReason", "killDeadline", "fingerprint", "baseFingerprint", "progressDigest"}
			case "backoff-state":
				fields = []string{"noProgress", "cooldownUntil", "parked", "fingerprint", "baseFingerprint", "progressDigest"}
			case "proc":
				fields = []string{"pid", "identity"}
			case "seen":
				fields = []string{"tickets", "claims", "lanes"}
			case "history":
				fields = []string{"current", "seen"}
			}
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				key, ok := k.(string)
				if err != nil || !ok || seen[key] || (fields != nil && !slices.Contains(fields, key)) {
					return false
				}
				seen[key] = true
				child := ""
				switch schema {
				case "ledger":
					if key == "workers" || key == "backoff" || key == "seen" || key == "progress" || key == "poolSweeps" {
						child = key
					}
				case "worker":
					if key == "members" {
						child = key
					}
				case "backoff":
					child = "backoff-state"
				case "progress":
					child = "history"
				case "poolSweeps":
					child = "sweep-record"
				case "sweep-record":
					child = "sweep-scalar"
					if key == "result" {
						child = "sweep-result"
					}
				case "sweep-result":
					child = "sweep-scalar"
				}
				if !value(depth+1, child) {
					return false
				}
			}
			if schema == "sweep-record" {
				for _, key := range fields {
					if key != "reason" && !seen[key] {
						return false
					}
				}
			}
			if schema == "sweep-result" && !seen["pending"] {
				return false
			}
			end, err := d.Token()
			return err == nil && end == json.Delim('}')
		case json.Delim('['):
			child := ""
			if schema == "workers" {
				child = "worker"
			} else if schema == "members" {
				child = "proc"
			}
			for d.More() {
				if !value(depth+1, child) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim(']')
		case json.Delim('}'), json.Delim(']'):
			return false
		default:
			if strings.HasPrefix(schema, "sweep-") || schema == "poolSweeps" {
				return token != nil
			}
			return true
		}
	}
	if !value(0, "ledger") {
		return false
	}
	var extra any
	return d.Decode(&extra) == io.EOF
}

func (l *Ledger) validateProgress() error {
	count := 0
	for key, h := range l.Progress {
		if _, err := wire.ParseTicketID("progress key", key); err != nil || h == nil || len(h.Seen) == 0 || len(h.Seen) > maxProgressPerKey || !validProgressDigest(h.Current) {
			return errors.New("invalid progress history")
		}
		for i, digest := range h.Seen {
			if !validProgressDigest(digest) || (i > 0 && h.Seen[i-1] >= digest) {
				return errors.New("invalid or duplicate progress digest")
			}
		}
		i := sort.SearchStrings(h.Seen, h.Current)
		if i == len(h.Seen) || h.Seen[i] != h.Current {
			return errors.New("current progress digest absent from history")
		}
		count += len(h.Seen)
	}
	if count > maxProgressProgram {
		return errors.New("progress program capacity exceeded")
	}
	check := func(key, base, digest, fp string) error {
		if base == "" && digest == "" && l.Progress[key] == nil {
			return nil
		}
		h := l.Progress[key]
		if h == nil || !validProgressDigest(base) || !validProgressDigest(digest) || progressFingerprint(base, digest) != fp {
			return errors.New("invalid progress accounting baseline")
		}
		i := sort.SearchStrings(h.Seen, digest)
		if i == len(h.Seen) || h.Seen[i] != digest {
			return errors.New("accounting digest absent from history")
		}
		return nil
	}
	for _, w := range l.Workers {
		if w == nil {
			return errors.New("null worker")
		}
		if err := check(w.Key, w.BaseFingerprint, w.ProgressDigest, w.Fingerprint); err != nil {
			return err
		}
	}
	for key, b := range l.Backoff {
		if b == nil {
			return errors.New("null backoff")
		}
		if err := check(key, b.BaseFingerprint, b.ProgressDigest, b.Fingerprint); err != nil {
			return err
		}
	}
	return nil
}

func ledgerBytes(l *Ledger) ([]byte, error) {
	raw, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if len(raw) > maxLedger {
		return nil, errors.New("dispatch full ledger exceeds 16MiB")
	}
	return raw, nil
}
func (l *Ledger) save(dir string) error {
	raw, err := ledgerBytes(l)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "state.json"), raw)
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
// --output-format stream-json --verbose, or json without --verbose, gemini -p
// -o stream-json) or gemini's single pretty-printed -o json object; otherwise
// the raw tail.
func extractText(raw []byte) string {
	var doc map[string]any
	if json.Unmarshal(raw, &doc) == nil {
		if s, ok := doc["response"].(string); ok && s != "" {
			return s
		}
	}
	last, delta := "", ""
	inDelta := false
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
		// gemini streams one assistant reply as consecutive delta chunks; any
		// other event ends that reply.
		if s, ok := v["content"].(string); ok && v["type"] == "message" && v["role"] == "assistant" && v["delta"] == true {
			if !inDelta {
				delta = ""
			}
			inDelta = true
			if delta += s; delta != "" {
				last = delta
			}
			continue
		}
		inDelta = false
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
