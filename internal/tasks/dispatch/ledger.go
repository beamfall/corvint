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
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	Tier            int       `json:"tier,omitempty"`
	Model           string    `json:"model,omitempty"`
}

// EscalationState is the CAL-V0-057 ladder record of one ticket: the
// trailing count of finished sessions without progress, and the tier each
// escalating role last launched at.
type EscalationState struct {
	Streak int            `json:"streak"`
	Tiers  map[string]int `json:"tiers,omitempty"`
}

// Infrastructure retry episode states (ESC-V0-007). WAITING has a reserved
// retry ordinal and cooldown deadline; RESERVED has also written its launch
// identity before spawn; RUNNING is that launch's recorded worker; IDLE keeps
// the debt with no retry pending; RECOVERED followed checked work progress.
// EXHAUSTED, DISABLED, NATIVE_EXHAUSTED and UNKNOWN hold the ticket.
const (
	InfraWaiting         = "WAITING"
	InfraReserved        = "RESERVED"
	InfraRunning         = "RUNNING"
	InfraIdle            = "IDLE"
	InfraRecovered       = "RECOVERED"
	InfraExhausted       = "EXHAUSTED"
	InfraDisabled        = "DISABLED"
	InfraNativeExhausted = "NATIVE_EXHAUSTED"
	InfraUnknown         = "UNKNOWN"
)

// InfraEpisode is one ticket's ESC-V0-007 infrastructure retry episode at
// one acceptance revision, shared across roles and request IDs. Sessions
// counts ended infrastructure sessions, each once; Charged counts reserved
// retry ordinals; Limit is the maxRetries the episode started with, which a
// configuration reload can only narrow. Nothing here resets automatically.
type InfraEpisode struct {
	AcceptanceRevision string    `json:"acceptanceRevision"`
	State              string    `json:"state"`
	Sessions           int       `json:"sessions"`
	Charged            int       `json:"charged"`
	Limit              int       `json:"limit"`
	CooldownUntil      time.Time `json:"cooldownUntil"`
	Launch             string    `json:"launch,omitempty"`
}

// Holds reports whether the episode keeps its ticket from launching at now.
func (e *InfraEpisode) Holds(now time.Time) bool {
	switch e.State {
	case InfraExhausted, InfraDisabled, InfraNativeExhausted, InfraUnknown:
		return true
	case InfraWaiting:
		return e.CooldownUntil.After(now)
	}
	return false
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
	// Escalations keeps each ESC-V0-006 held ticket's request IDs, apart from
	// its plan reason, so status shows a hold behind another blocker.
	Escalations map[string][]string `json:"escalations,omitempty"`
	// Loops keeps each CAL-V0-102 held ticket's loop hold, so status shows
	// it and diff raises one blocked escalation event per episode.
	Loops map[string]LoopHold `json:"loops,omitempty"`
	// Requests keeps each ticket's current OPEN escalation requests, and
	// RequestsUnknown the sorted tickets whose escalation material could not
	// be validated, so status shows kinds and ages without reading the
	// native store (ESC-V0-009).
	Requests        map[string][]OpenRequest `json:"requests,omitempty"`
	RequestsUnknown []string                 `json:"requestsUnknown,omitempty"`
}

// OpenRequest is one current OPEN escalation request: its kind and the
// RecordedAt of its audited OPEN event.
type OpenRequest struct {
	RequestID  string `json:"requestId"`
	Kind       string `json:"kind"`
	RecordedAt string `json:"recordedAt"`
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
	Pressure   *PressureRecord             `json:"pressure,omitempty"`
	// Escalation is present only while the configuration has a ladder.
	Escalation map[string]*EscalationState `json:"escalation,omitempty"`
	// InfraRetry is present only once an ESC-V0-007 episode exists.
	InfraRetry map[string]*InfraEpisode `json:"infraRetry,omitempty"`
	// Config is present only after this run's configuration file changed
	// (CAL-V0-127).
	Config *ConfigRecord `json:"config,omitempty"`
}

const maxPressureHeld, maxPressureProblems, maxPressureProblem = 8192, 8, 200

// PressureRecord is the CAL-V0-068 derived pressure state: hysteresis, the
// newest bounded sample and the work keys the last roster held. It exists
// only while pressure is configured and is never a native-store input.
type PressureRecord struct {
	State  PressureState  `json:"state"`
	Sample PressureSample `json:"sample"`
	Held   []HeldLaunch   `json:"held"`
}

// HeldLaunch is one roster candidate held by the pressure budget.
type HeldLaunch struct {
	Role   string `json:"role"`
	Key    string `json:"key"`
	Ticket string `json:"ticket,omitempty"`
}

func (r *PressureRecord) validate() error {
	st := r.State
	if st.Level < 0 || st.Level > 2 || st.PendingLevel < 0 || st.PendingLevel > 2 || st.PendingTicks < 0 || st.PendingTicks >= 3600 || !ValidPressureReason(st.Level, st.Reason) {
		return errors.New("invalid pressure state")
	}
	if m := r.Sample.MemoryPressureLevel; (r.Sample.MemoryPressureKnown && !validMemoryPressureLevel(m)) || (!r.Sample.MemoryPressureKnown && m != 0) {
		return errors.New("invalid pressure memory level")
	}
	if _, ok := r.Sample.CPUUtilizationFraction(); ok != r.Sample.CPUUtilizationKnown || (!ok && r.Sample.CPUUtilization != 0) {
		return errors.New("invalid pressure cpu utilization")
	}
	if len(r.Held) > maxPressureHeld || len(r.Sample.Problems) > maxPressureProblems || len(r.Sample.Source) > 256 {
		return errors.New("pressure record exceeds bounds")
	}
	for _, p := range r.Sample.Problems {
		if len(p) > maxPressureProblem {
			return errors.New("pressure record exceeds bounds")
		}
	}
	keys := map[string]bool{}
	for _, h := range r.Held {
		if !ValidName(h.Role) || h.Key == "" || len(h.Key) > 512 || len(h.Ticket) > 512 || keys[h.Key] {
			return errors.New("invalid or duplicate pressure held launch")
		}
		keys[h.Key] = true
	}
	return nil
}

// boundPressureSample keeps a sample's diagnostics inside the ledger bounds.
func boundPressureSample(s PressureSample) PressureSample {
	if !finiteNonnegative(s.LoadAverage) {
		s.LoadAverage, s.LoadKnown = 0, false
	}
	if _, ok := s.MemoryPressure(); !ok {
		// An invalid level stays UNKNOWN: a sample carrying one never
		// falls back to swap (V1-0862).
		if s.MemoryPressureKnown || s.MemoryPressureLevel != 0 {
			s.Problems = append([]string{"memory: invalid kernel memory pressure level"}, s.Problems...)
			s.SwapKnown = false
		}
		s.MemoryPressureLevel, s.MemoryPressureKnown = 0, false
	}
	if _, ok := s.CPUUtilizationFraction(); !ok {
		s.CPUUtilization, s.CPUUtilizationKnown = 0, false
	}
	s.Source = boundUTF8(s.Source, 256)
	var problems []string
	for _, p := range s.Problems {
		if len(problems) == maxPressureProblems {
			break
		}
		problems = append(problems, boundUTF8(p, maxPressureProblem))
	}
	s.Problems = problems
	return s
}

// boundUTF8 replaces invalid UTF-8 and truncates s to at most n bytes at a
// rune boundary.
func boundUTF8(s string, n int) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

const maxProgressPerKey, maxProgressProgram = 256, 8192

// ProgressHistory is lifetime replay protection, including deleted keys.
type ProgressHistory struct {
	Current string   `json:"current"`
	Seen    []string `json:"seen"`
}

// ProgramDir is the dispatcher's state directory for one program.
func ProgramDir(c *Config, program string) string { return filepath.Join(c.StateDir, program) }

// ledgerFormat is the CAL-V0-132 adjacent-build refusal: a ledger whose
// profile is another taskman-dispatch-state version, or which carries a
// top-level member that no spelling of a known member matches, was written
// by a build with another format. It is refused as UNSUPPORTED_VERSION and
// never read or migrated; case-folded aliases keep their malformed refusal.
func ledgerFormat(members map[string]json.RawMessage) error {
	var profile string
	if raw, ok := members["profile"]; ok && json.Unmarshal(raw, &profile) == nil && profile != StateProfile {
		if err := wire.CheckProfile("/profile", profile, StateProfile); wire.CodeOf(err) == wire.CodeUnsupportedVersion {
			return err
		}
	}
	known := reflect.TypeFor[Ledger]()
	for name := range members {
		found := false
		for i := 0; i < known.NumField() && !found; i++ {
			tag, _, _ := strings.Cut(known.Field(i).Tag.Get("json"), ",")
			found = strings.EqualFold(name, tag)
		}
		if !found {
			return wire.Errorf(wire.CodeUnsupportedVersion, "/"+name, "dispatch state member is not known to this build")
		}
	}
	return nil
}

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
	if err := ledgerFormat(members); err != nil {
		return nil, err
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
	// ESC-V0-007: an infrastructure retry episode always needs the strict
	// reader, whether or not the ledger carries progress.
	for name := range members {
		if strings.EqualFold(name, "infraRetry") && (!validScalarJSON(raw) || !strictProgressJSON(raw)) {
			return nil, errors.New("dispatch state: malformed infraRetry JSON")
		}
	}
	// CAL-V0-103: a recorded loop hold is closed whether or not the ledger
	// carries progress, so its presence alone requires the strict reader.
	if seenCarries(raw, "loops") && (!validScalarJSON(raw) || !strictProgressJSON(raw)) {
		return nil, errors.New("dispatch state: malformed loops JSON")
	}
	// ESC-V0-009: recorded open requests are likewise closed on their own.
	if seenCarries(raw, "requests", "requestsUnknown") && (!validScalarJSON(raw) || !strictProgressJSON(raw)) {
		return nil, errors.New("dispatch state: malformed requests JSON")
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
	if err := l.validateEscalation(); err != nil {
		return nil, fmt.Errorf("dispatch state: %w", err)
	}
	if err := l.validateSeenEscalations(); err != nil {
		return nil, fmt.Errorf("dispatch state: %w", err)
	}
	if err := l.validateInfraRetry(); err != nil {
		return nil, fmt.Errorf("dispatch state: %w", err)
	}
	if err := l.validateSeenLoops(); err != nil {
		return nil, fmt.Errorf("dispatch state: %w", err)
	}
	if err := l.validateSeenRequests(); err != nil {
		return nil, fmt.Errorf("dispatch state: %w", err)
	}
	if l.Pressure != nil {
		if err := l.Pressure.validate(); err != nil {
			return nil, fmt.Errorf("dispatch state: %w", err)
		}
	}
	if l.Config != nil {
		if err := l.Config.validate(); err != nil {
			return nil, fmt.Errorf("dispatch state: %w", err)
		}
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
				fields = []string{"profile", "program", "launchSeq", "eventSeq", "workers", "backoff", "seen", "progress", "poolSweeps", "pressure", "escalation", "infraRetry", "config"}
			case "sweep-record":
				fields = []string{"workRoot", "program", "queue", "pool", "member", "allocation", "definition", "requestId", "actor", "actorRole", "configDigest", "timeoutSeconds", "phase", "started", "observed", "result", "reason"}
			case "sweep-result":
				fields = []string{"pending", "evidence", "receipt", "receiptSeq", "outcome"}
			case "worker":
				fields = []string{"id", "role", "host", "slot", "key", "ticket", "pool", "member", "pid", "leaderIdentity", "members", "started", "lastActive", "logBytes", "activityPaths", "activityMtime", "state", "killReason", "killDeadline", "fingerprint", "baseFingerprint", "progressDigest", "tier", "model"}
			case "backoff-state":
				fields = []string{"noProgress", "cooldownUntil", "parked", "fingerprint", "baseFingerprint", "progressDigest"}
			case "proc":
				fields = []string{"pid", "identity"}
			case "seen":
				fields = []string{"tickets", "claims", "lanes", "escalations", "loops", "requests", "requestsUnknown"}
			case "open-request":
				fields = []string{"requestId", "kind", "recordedAt"}
			case "loop-hold":
				fields = []string{"signal", "acceptanceRevision", "generations", "pending"}
			case "history":
				fields = []string{"current", "seen"}
			case "escalation-state":
				fields = []string{"streak", "tiers"}
			case "infra-episode":
				fields = []string{"acceptanceRevision", "state", "sessions", "charged", "limit", "cooldownUntil", "launch"}
			case "config":
				fields = []string{"appliedSha256", "appliedAt", "refused"}
			case "config-refusal":
				fields = []string{"sha256", "at", "reason"}
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
					if key == "workers" || key == "backoff" || key == "seen" || key == "progress" || key == "poolSweeps" || key == "escalation" || key == "infraRetry" || key == "config" {
						child = key
					}
				case "config":
					if key == "refused" {
						child = "config-refusal"
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
				case "escalation":
					child = "escalation-state"
				case "infraRetry":
					child = "infra-episode"
				case "infra-episode":
					child = "infra-scalar"
				case "seen":
					if key == "loops" || key == "requests" {
						child = key
					}
				case "requests":
					child = "request-list"
				case "loops":
					child = "loop-hold"
				case "loop-hold":
					if key == "pending" {
						child = "loop-pending"
					}
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
			// Every episode member but launch is written, so an omitted
			// one cannot decode as zero debt or an elapsed deadline.
			if schema == "infra-episode" {
				for _, key := range fields {
					if key != "launch" && !seen[key] {
						return false
					}
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim('}')
		case json.Delim('['):
			child := ""
			if schema == "workers" {
				child = "worker"
			} else if schema == "members" {
				child = "proc"
			} else if schema == "request-list" {
				child = "open-request"
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
			switch schema {
			case "loops", "loop-hold", "infraRetry", "infra-episode", "requests", "open-request":
				return false // these maps and their records are objects
			case "infra-scalar":
				return token != nil // a null would decode as zero
			case "loop-pending":
				return token == true // written only as true
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

// seenCarries reports whether any member named like "seen", including a
// duplicate, holds a member named like one of names, folding case as the
// struct decoder does. Malformed JSON reports true, so the strict reader
// decides.
func seenCarries(raw []byte, names ...string) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	skip := func() bool {
		depth := 0
		for {
			t, err := d.Token()
			if err != nil {
				return false
			}
			switch t {
			case json.Delim('{'), json.Delim('['):
				depth++
			case json.Delim('}'), json.Delim(']'):
				depth--
			}
			if depth == 0 {
				return true
			}
		}
	}
	if t, err := d.Token(); err != nil || t != json.Delim('{') {
		return true
	}
	for d.More() {
		k, err := d.Token()
		if err != nil {
			return true
		}
		if name, _ := k.(string); !strings.EqualFold(name, "seen") {
			if !skip() {
				return true
			}
			continue
		}
		t, err := d.Token()
		if err != nil {
			return true
		}
		if t != json.Delim('{') {
			continue
		}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return true
			}
			if name, _ := k.(string); slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(name, n) }) {
				return true
			}
			if !skip() {
				return true
			}
		}
		if _, err := d.Token(); err != nil {
			return true
		}
	}
	return false
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

// validateEscalation bounds the ladder records: ticket keys, a
// non-negative streak, and role tiers 1..8.
func (l *Ledger) validateEscalation() error {
	for key, e := range l.Escalation {
		if _, err := wire.ParseTicketID("escalation key", key); err != nil || e == nil || e.Streak < 0 || len(e.Tiers) > 32 {
			return errors.New("invalid escalation record")
		}
		for role, tier := range e.Tiers {
			if !ValidName(role) || tier < 1 || tier > 8 {
				return errors.New("invalid escalation tier")
			}
		}
	}
	for _, w := range l.Workers {
		if w != nil && (w.Tier < 0 || w.Tier > 8 || w.Tier > 0 && w.Model == "") {
			return errors.New("invalid worker tier")
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
	return syncedRename(tmp.Name(), path)
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
var EventKinds = []string{"started", "stopped", "adopted", "launched", "launch-failed", "finished", "killing", "killed", "handoff", "handoff-refused", "reaped", "state", "claim", "release", "lane", "cooldown", "parked", "unparked", "alert", "needs-owner", "throttled", "escalated", "config"}

// appendEvent writes one event line, rotating the log once at 16 MiB.
// Tests replace it to inject partial writes and close failures.
var appendEvent = appendEventLog

// appendEventLog first ends a trailing unterminated fragment, which an
// append that failed part-way can leave, so the new line parses on its own
// (CAL-V0-103); a well-formed log receives exactly the line.
func appendEventLog(dir string, e Event) error {
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
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	line := append(raw, '\n')
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	if st.Size() > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, st.Size()-1); err != nil {
			f.Close()
			return err
		}
		if last[0] != '\n' {
			line = append([]byte{'\n'}, line...)
		}
	}
	if _, err := f.Write(line); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// loopEventRecorded reports whether the readable tail of the current or
// rotated event log already holds the CAL-V0-103 needs-owner event of this
// loop episode: the ticket, signal, acceptance revision and newest counted
// generation.
func loopEventRecorded(dir, ticketID string, h LoopHold) bool {
	newest := func(gens string) string { return gens[strings.LastIndexByte(gens, ',')+1:] }
	want := newest(strings.Join(h.Generations, ","))
	for _, name := range []string{"events.jsonl", "events.jsonl.1"} {
		raw, err := readTail(filepath.Join(dir, name), 1<<20)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			var e Event
			if json.Unmarshal([]byte(line), &e) != nil || e.Profile != EventProfile || e.Kind != "needs-owner" || e.Ticket != ticketID {
				continue
			}
			if e.Detail["code"] == "LOOP_DETECTED" && e.Detail["signal"] == h.Signal && e.Detail["acceptanceRevision"] == h.AcceptanceRevision && newest(e.Detail["generations"]) == want {
				return true
			}
		}
	}
	return false
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

// syncedRename publishes tmp at path and then fsyncs the parent directory:
// a ledger write counts as durable only once the rename itself is durable
// (SERVICE500-003), so a launch intent is never resolved by a rename a power
// loss could still undo.
func syncedRename(tmp, path string) error {
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

// syncDir fsyncs a directory; tests replace it to inject a failed sync
// after a successful rename.
var syncDir = syncDirectory

func syncDirectory(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// validateSeenEscalations admits only what diff records for ESC-V0-006: per
// held ticket, 1 to 16 strictly sorted request identifiers.
func (l *Ledger) validateSeenEscalations() error {
	if l.Seen == nil {
		return nil
	}
	for key, ids := range l.Seen.Escalations {
		if _, err := wire.ParseTicketID("escalations key", key); err != nil || len(ids) == 0 || len(ids) > wire.EscalationMaxCurrentOpen {
			return errors.New("invalid escalation hold")
		}
		for i, id := range ids {
			if _, err := wire.ParseIdentifier("escalation request", id); err != nil || (i > 0 && ids[i-1] >= id) {
				return errors.New("invalid escalation hold")
			}
		}
	}
	return nil
}

// validateSeenLoops admits only what diff records for CAL-V0-102: per held
// ticket, a known signal and 1 to MaxLoopGenerations generations.
func (l *Ledger) validateSeenLoops() error {
	if l.Seen == nil {
		return nil
	}
	for key, h := range l.Seen.Loops {
		if _, err := wire.ParseTicketID("loops key", key); err != nil || (h.Signal != "NO_PROGRESS" && h.Signal != "ALTERNATING_RETURNS") || len(h.Generations) == 0 || len(h.Generations) > MaxLoopGenerations {
			return errors.New("invalid loop hold")
		}
		if _, err := wire.ParseSize("loop acceptance revision", h.AcceptanceRevision); err != nil {
			return errors.New("invalid loop hold")
		}
		for _, g := range h.Generations {
			if _, err := wire.ParseSize("loop generation", g); err != nil {
				return errors.New("invalid loop hold")
			}
		}
	}
	return nil
}

// validateSeenRequests admits only what diff records for ESC-V0-009: per
// ticket, 1 to 16 strictly sorted requests of a known kind with a canonical
// OPEN time, and strictly sorted unknown tickets that name no request.
func (l *Ledger) validateSeenRequests() error {
	if l.Seen == nil {
		return nil
	}
	bad := errors.New("invalid escalation requests")
	for key, rs := range l.Seen.Requests {
		if _, err := wire.ParseTicketID("requests key", key); err != nil || len(rs) == 0 || len(rs) > wire.EscalationMaxCurrentOpen {
			return bad
		}
		for i, r := range rs {
			if _, err := wire.ParseIdentifier("escalation request", r.RequestID); err != nil || (i > 0 && rs[i-1].RequestID >= r.RequestID) {
				return bad
			}
			if _, err := wire.ParseTimestamp("escalation recordedAt", r.RecordedAt); err != nil {
				return bad
			}
			switch r.Kind {
			case "decision", "scope", "blocked", "infrastructure":
			default:
				return bad
			}
		}
	}
	for i, key := range l.Seen.RequestsUnknown {
		if _, err := wire.ParseTicketID("requestsUnknown", key); err != nil || (i > 0 && l.Seen.RequestsUnknown[i-1] >= key) {
			return bad
		}
		if _, ok := l.Seen.Requests[key]; ok {
			return bad
		}
	}
	return nil
}

// MaxLoopGenerations bounds the generations one recorded loop hold names.
const MaxLoopGenerations = 1024

// maxInfraEpisodes bounds the ESC-V0-007 episodes, like progress histories.
const maxInfraEpisodes = 8192

// validateInfraRetry admits only episodes the dispatcher writes: canonical
// ticket keys and acceptance revisions, a known state, bounded counts, a
// launch identity exactly while one is reserved or running, and a charged
// retry with its deadline while one waits, is reserved or runs.
func (l *Ledger) validateInfraRetry() error {
	if l.InfraRetry != nil && len(l.InfraRetry) == 0 || len(l.InfraRetry) > maxInfraEpisodes {
		return errors.New("invalid infraRetry episodes")
	}
	for key, e := range l.InfraRetry {
		if _, err := wire.ParseTicketID("infraRetry key", key); err != nil || e == nil {
			return errors.New("invalid infraRetry episode")
		}
		if _, err := wire.ParseCount("infraRetry acceptance revision", e.AcceptanceRevision); err != nil {
			return errors.New("invalid infraRetry episode")
		}
		if e.Sessions < 1 || e.Sessions > 1024 || e.Charged < 0 || e.Charged > 10 || e.Charged > e.Sessions || e.Limit < 0 || e.Limit > 10 {
			return errors.New("invalid infraRetry counts")
		}
		switch e.State {
		case InfraWaiting, InfraReserved, InfraRunning:
			if e.Charged < 1 || e.CooldownUntil.IsZero() || e.State == InfraWaiting && e.Charged > e.Limit {
				return errors.New("invalid infraRetry retry")
			}
		}
		switch e.State {
		case InfraReserved, InfraRunning:
			if e.Launch == "" {
				return errors.New("invalid infraRetry launch")
			}
		case InfraWaiting:
		case InfraIdle, InfraRecovered, InfraExhausted, InfraDisabled, InfraNativeExhausted, InfraUnknown:
			if e.Launch != "" {
				return errors.New("invalid infraRetry launch")
			}
		default:
			return errors.New("invalid infraRetry state")
		}
		if len(e.Launch) > 128 {
			return errors.New("invalid infraRetry launch")
		}
	}
	return nil
}
