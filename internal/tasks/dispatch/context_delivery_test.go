//go:build darwin || linux

package dispatch

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// claudeText is a main-session Claude Code assistant event carrying text.
func claudeText(s string) string {
	raw, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": s}}}, "parent_tool_use_id": nil})
	return string(raw)
}

// fakeSecret is built at run time so the source holds no secret-shaped text.
func fakeSecret() string { return "gh" + "p_" + strings.Repeat("a1b2c3d4", 4) }

// openCtx opens a dispatcher on one claude-code role over t1 with a fixed
// clock; it launches nothing until ticked.
func openCtx(t *testing.T, q *fakeQueue, c *Config) (*Dispatcher, *time.Time) {
	t.Helper()
	c.GlobalCap = 1
	c.Backoff.CooldownSeconds, c.Backoff.ParkAfter = 3600, 10
	c.Roles[0].IdleSeconds, c.Roles[0].WallSeconds = 3600, 7200
	c.Roles[0].UsageFormat = "claude-code"
	q.obs.Tickets = []Ticket{ticket("t1", "P1", 1)}
	d, err := Open("ctx", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	now := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	d.Now = func() time.Time { return now }
	return d, &now
}

func killOnCleanup(t *testing.T, w *Worker) {
	pid := w.PID
	t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL); syscall.Kill(pid, syscall.SIGKILL) })
}

func stderrOf(d *Dispatcher, id string) string {
	b, _ := os.ReadFile(filepath.Join(d.workerDir(id), "stderr.log"))
	return strings.TrimSpace(string(b))
}

// ATR-V0-102 and ATR-V0-104: a host using {contextHookSettings} gets a
// settings file registering the context hook, the placeholder renders to
// its path, and every launch names an existing evidence directory. A
// preparation that fails launches nothing.
func TestATRV0102_LaunchWritesHookSettingsAndPlaceholder(t *testing.T) {
	q := &fakeQueue{}
	c := testConfig(t, `printf '%s|%s|%s\n' "$1" "$CORVINT_DISPATCH_EVIDENCE_DIR" "${CORVINT_DISPATCH_HANDOFF_NOTE-unset}" >&2; exec sleep 300`)
	h := c.Hosts["sh"]
	h.Argv = []string{h.Argv[0], "{contextHookSettings}", "{prompt}"}
	c.Hosts["sh"] = h
	d, _ := openCtx(t, q, c)
	d.HookCommand = []string{"/opt/it's/corvint-tasks", "dispatch", "context-hook"}
	tick(t, d)
	if d.Running() != 1 {
		t.Fatalf("running %d", d.Running())
	}
	w := d.ledger.Workers[0]
	killOnCleanup(t, w)
	tickUntil(t, d, func() bool { return stderrOf(d, w.ID) != "" })
	settings, evidence := ContextHookSettingsPath(d.dir, w.ID), EvidenceDir(d.dir, w.ID)
	if got, want := stderrOf(d, w.ID), settings+"|"+evidence+"|unset"; got != want {
		t.Fatalf("worker saw %q, want %q", got, want)
	}
	if st, err := os.Lstat(evidence); err != nil || !st.IsDir() {
		t.Fatalf("evidence directory %v %v", st, err)
	}
	raw, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct{ Type, Command string }
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &s); err != nil || len(s.Hooks) != 1 || len(s.Hooks["PostToolUse"]) != 1 {
		t.Fatalf("settings %s %v", raw, err)
	}
	entry := s.Hooks["PostToolUse"][0]
	if entry.Matcher != "*" || len(entry.Hooks) != 1 || entry.Hooks[0].Type != "command" || entry.Hooks[0].Command != `'/opt/it'\''s/corvint-tasks' 'dispatch' 'context-hook'` {
		t.Fatalf("settings entry %+v", entry)
	}

	// A preparation failure is proved before spawn: nothing launches.
	q2 := &fakeQueue{}
	d2, _ := openCtx(t, q2, testConfig(t, "exec sleep 300"))
	if err := os.WriteFile(filepath.Join(d2.dir, "workers"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tick(t, d2)
	failed := eventsOf(t, d2, "launch-failed")
	if d2.Running() != 0 || len(failed) != 1 || !strings.Contains(failed[0].Message, "before spawn") {
		t.Fatalf("running %d launch-failed %+v", d2.Running(), failed)
	}
}

// ATR-V0-102: only a claude-code role may run on a host that uses
// {contextHookSettings}; ATR-V0-103 reports each format's delivery mode.
func TestATRV0102_PlaceholderNeedsClaudeCode(t *testing.T) {
	c := testConfig(t, "exit 0")
	h := c.Hosts["sh"]
	h.Argv = []string{h.Argv[0], "--settings", "{contextHookSettings}"}
	c.Hosts["sh"] = h
	for format, ok := range map[string]bool{"claude-code": true, "opencode": false, "codex": false, "": false} {
		c.Roles[0].UsageFormat = format
		raw, _ := json.Marshal(c)
		_, err := DecodeConfig(raw)
		if (err == nil) != ok {
			t.Errorf("usageFormat %q: err %v, want accepted %v", format, err, ok)
		}
	}
	envHost := Host{Argv: []string{"/bin/sh"}, Env: map[string]string{"CLAUDE_SETTINGS": "{contextHookSettings}"}}
	if !hostUsesContextHook(envHost) || hostUsesContextHook(Host{Argv: []string{"/bin/sh", "{prompt}"}}) {
		t.Fatal("placeholder detection")
	}
	dir := t.TempDir()
	for _, tc := range []struct {
		format string
		host   *Host
		mode   string
		ok     bool
	}{{"claude-code", &h, DeliveryHook, true}, {"claude-code", &Host{Argv: []string{"/bin/sh"}}, DeliveryUnknown, true}, {"claude-code", nil, DeliveryUnknown, true}, {"opencode", &h, DeliveryUnsupported, true}, {"codex", &h, "", false}} {
		mode, delivered, ok := ContextDelivery(dir, "w1", tc.format, tc.host)
		if mode != tc.mode || ok != tc.ok || (ok && len(delivered) != 0) {
			t.Errorf("%s: mode %q delivered %v ok %v", tc.format, mode, delivered, ok)
		}
	}
}

// ATR-V0-104: a worker stopped at hardTokens leaves a bounded, screened
// note of its last texts and evidence files, announced by one context
// event; ATR-V0-106 counts the stop.
func TestATRV0104_HardStopSalvagesBoundedScreenedNote(t *testing.T) {
	q := &fakeQueue{}
	secret := fakeSecret()
	script := `E="$CORVINT_DISPATCH_EVIDENCE_DIR"
printf 'step one\n%s\nstep two\n' '` + secret + `' > "$E/a-progress.md"
head -c 20000 /dev/zero | tr '\000' x > "$E/b-big.log"
ln -s /etc/hosts "$E/c-link"
mkdir "$E/d-sub"
for i in 1 2 3 4 5 6 7 8; do echo "f$i" > "$E/f$i.txt"; done
printf '%s\n' '` + claudeText("first") + `' '` + claudeText("second") + `' '` + claudeText("third") + `' '` + claudeText("fourth\n"+secret+"\nend") + `' '` + claudeAssistant(4970, 0) + `'
exec sleep 300`
	c := testConfig(t, script)
	c.Roles[0].Context = &ContextLimits{WarnTokens: 1000, HardTokens: 4000}
	d, _ := openCtx(t, q, c)
	tick(t, d)
	if d.Running() != 1 {
		t.Fatalf("running %d", d.Running())
	}
	w := d.ledger.Workers[0]
	killOnCleanup(t, w)
	q.obs.Attempts = []Attempt{{ID: "a1", Ticket: w.Ticket, Holder: w.ID, Phase: "RUNNING", Generation: "3", Live: true, LeaseExpires: d.Now().Add(time.Hour)}}
	tickUntil(t, d, func() bool { return d.Running() == 0 })
	n, err := ReadHandoffNote(HandoffNotePath(d.dir, w.ID))
	if err != nil {
		t.Fatal(err)
	}
	if n.Worker != w.ID || n.Ticket != w.Ticket || n.Role != "impl" || n.ContextTokens != "5000" {
		t.Fatalf("note header %+v", n)
	}
	if strings.Join(n.Messages, "|") != "second|third|fourth\n"+redactedLine+"\nend" {
		t.Fatalf("messages %q", n.Messages)
	}
	var names []string
	for _, f := range n.Files {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "a-progress.md,b-big.log,f1.txt,f2.txt,f3.txt,f4.txt,f5.txt,f6.txt" || !n.Truncated {
		t.Fatalf("files %v truncated %v", names, n.Truncated)
	}
	if n.Files[0].Text != "step one\n"+redactedLine+"\nstep two\n" || n.RedactedLines != 2 {
		t.Fatalf("screened file %q redacted %d", n.Files[0].Text, n.RedactedLines)
	}
	if big := n.Files[1]; !big.Cut || big.Bytes != 20000 || len(big.Text) != maxNoteFileBytes {
		t.Fatalf("big file cut %v bytes %d text %d", big.Cut, big.Bytes, len(big.Text))
	}
	raw, _ := os.ReadFile(HandoffNotePath(d.dir, w.ID))
	if strings.Contains(string(raw), secret) {
		t.Fatal("the note holds the secret")
	}
	var salvage []Event
	for _, e := range eventsOf(t, d, "context") {
		if e.Detail["note"] != "" {
			salvage = append(salvage, e)
		}
	}
	if len(salvage) != 1 || salvage[0].Detail["note"] != HandoffNotePath(d.dir, w.ID) || salvage[0].Detail["messages"] != "3" || salvage[0].Detail["files"] != "8" || salvage[0].Detail["redactedLines"] != "2" {
		t.Fatalf("salvage events %+v", salvage)
	}
	rec, err := ReadContextExhausted(ContextExhaustedPath(d.dir))
	if err != nil || len(rec.Entries) != 1 || rec.Entries[0].HardStops != 1 || rec.Entries[0].State != ContextCounting || rec.Entries[0].LastWorker != w.ID {
		t.Fatalf("hard-stop record %+v %v", rec, err)
	}

	// The 64 KiB bound drops the oldest texts, then the last files.
	big := &Worker{ID: "w-big", Ticket: w.Ticket, Role: "impl", KillReason: "CONTEXT"}
	ev := EvidenceDir(d.dir, big.ID)
	if err := os.MkdirAll(ev, 0o700); err != nil {
		t.Fatal(err)
	}
	var log strings.Builder
	for i := 0; i < 5; i++ {
		log.WriteString(claudeText(strings.Repeat(string(rune('a'+i)), 20000)) + "\n")
	}
	if err := os.WriteFile(filepath.Join(d.workerDir(big.ID), "stdout.log"), []byte(log.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err := os.WriteFile(filepath.Join(ev, "e"+string(rune('0'+i))), []byte(strings.Repeat("y", 9000)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bn := d.buildHandoffNote(big, d.Now())
	if size := len(bn.encode()); size > maxHandoffNote || !bn.Truncated || len(bn.Messages) == 0 || len(bn.Messages[len(bn.Messages)-1]) != maxNoteMessage || !strings.HasPrefix(bn.Messages[len(bn.Messages)-1], "e") {
		t.Fatalf("bounded note %d bytes, truncated %v, %d messages", size, bn.Truncated, len(bn.Messages))
	}

	// A write failure alerts once and leaves no note.
	blocked := &Worker{ID: "w-blocked", Ticket: w.Ticket, Role: "impl", KillReason: "CONTEXT"}
	if err := os.MkdirAll(HandoffNotePath(d.dir, blocked.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	before := len(eventsOf(t, d, "alert"))
	d.salvage(blocked, d.Now())
	if alerts := eventsOf(t, d, "alert"); len(alerts) != before+1 || !strings.Contains(alerts[len(alerts)-1].Message, "hand-off note") {
		t.Fatalf("alerts %+v", alerts)
	}
}

// ATR-V0-105: the next launch on the ticket is handed the newest readable
// note linked by the attempt's dispatch hand-off evidence, and only that.
func TestATRV0105_NextAttemptReadsSalvagedNote(t *testing.T) {
	q := &fakeQueue{}
	script := `printf '%s\n' "${CORVINT_DISPATCH_HANDOFF_NOTE-unset}" >&2
if [ -n "$CORVINT_DISPATCH_HANDOFF_NOTE" ]; then cat "$CORVINT_DISPATCH_HANDOFF_NOTE" > "$CORVINT_DISPATCH_EVIDENCE_DIR/read-note.json"; exec sleep 300; fi
printf '%s\n' '` + claudeText("left off at step 3") + `' '` + claudeAssistant(4970, 0) + `'
exec sleep 300`
	c := testConfig(t, script)
	c.Roles[0].Context = &ContextLimits{HardTokens: 4000}
	d, now := openCtx(t, q, c)
	tick(t, d)
	first := d.ledger.Workers[0]
	killOnCleanup(t, first)
	q.obs.Attempts = []Attempt{{ID: "a1", Ticket: first.Ticket, Holder: first.ID, Phase: "RUNNING", Generation: "3", Live: true, LeaseExpires: now.Add(time.Hour)}}
	tickUntil(t, d, func() bool { return d.Running() == 0 })
	if stderrOf(d, first.ID) != "unset" {
		t.Fatalf("the first launch saw %q", stderrOf(d, first.ID))
	}
	// The store now records the dispatcher's hand-off evidence (ATR-V0-084).
	q.obs.Attempts[0].HandoffEvidence = "dispatch:" + first.ID
	q.obs.Attempts = append(q.obs.Attempts, Attempt{ID: "a0", Ticket: "ticket:a:q:t9", Phase: "CANCELLED", HandoffEvidence: "dispatch:" + first.ID})
	*now = now.Add(2 * time.Hour)
	tickUntil(t, d, func() bool { return d.Running() == 1 })
	second := d.ledger.Workers[0]
	killOnCleanup(t, second)
	note := HandoffNotePath(d.dir, first.ID)
	read := filepath.Join(EvidenceDir(d.dir, second.ID), "read-note.json")
	tickUntil(t, d, func() bool { _, err := os.Stat(read); return err == nil && stderrOf(d, second.ID) != "" })
	if got := stderrOf(d, second.ID); got != note {
		t.Fatalf("the next launch saw %q, want %q", got, note)
	}
	raw, _ := os.ReadFile(read)
	var n HandoffNote
	if err := json.Unmarshal(raw, &n); err != nil || len(n.Messages) != 1 || n.Messages[0] != "left off at step 3" || n.Worker != first.ID {
		t.Fatalf("the next attempt read %s (%v)", raw, err)
	}

	// Another ticket's attempt naming the worker is never offered the note,
	// nor is a note of another profile version or one that does not read.
	obs := &Observation{Attempts: q.obs.Attempts}
	if got := d.handoffNoteFor(obs, "ticket:a:q:t9"); got != "" {
		t.Fatalf("another ticket was offered %q", got)
	}
	if got := d.handoffNoteFor(obs, first.Ticket); got != note {
		t.Fatalf("offered %q", got)
	}
	newer := strings.Replace(string(raw), HandoffNoteProfile, "taskman-dispatch-handoff-note/1", 1)
	if err := os.WriteFile(note, []byte(newer), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := d.handoffNoteFor(obs, first.Ticket); got != "" {
		t.Fatalf("a newer-version note was offered: %q", got)
	}
	if err := os.WriteFile(note, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := d.handoffNoteFor(obs, first.Ticket); got != "" {
		t.Fatalf("an unreadable note was offered: %q", got)
	}
}

func hardStop(d *Dispatcher, id, role, key string, reason string) {
	d.noteHardStop(&Worker{ID: id, Role: role, Key: key, Ticket: key, KillReason: reason}, d.Now())
}

func exhaustedAlerts(t *testing.T, d *Dispatcher) []Event {
	var out []Event
	for _, e := range eventsOf(t, d, "alert") {
		if e.Detail["code"] == ContextExhausted {
			out = append(out, e)
		}
	}
	return out
}

func streakOf(t *testing.T, d *Dispatcher, key, role string) *HardStopStreak {
	t.Helper()
	rec, err := ReadContextExhausted(ContextExhaustedPath(d.dir))
	if err != nil {
		return nil
	}
	for i := range rec.Entries {
		if rec.Entries[i].Key == key && rec.Entries[i].Role == role {
			return &rec.Entries[i]
		}
	}
	return nil
}

// ATR-V0-106: N consecutive hard stops of one role on one ticket raise
// CONTEXT_EXHAUSTED with one alert; N-1 do not, any other ending resets the
// count, and lane keys are not tracked. ATR-V0-107: the count survives a
// restart.
func TestATRV0106_ExhaustedAfterNHardStopsNotNMinusOne(t *testing.T) {
	q := &fakeQueue{}
	c := testConfig(t, "exit 0")
	c.Roles[0].Context = &ContextLimits{HardTokens: 4000, ExhaustedAfter: 2}
	c.Roles = append(c.Roles, c.Roles[0])
	c.Roles[1].Name, c.Roles[1].Context = "review", &ContextLimits{HardTokens: 4000}
	d, _ := openCtx(t, q, c)
	const k1, k2 = "ticket:a:q:t1", "ticket:a:q:t2"

	hardStop(d, "w1", "impl", k1, "CONTEXT")
	if s := streakOf(t, d, k1, "impl"); s == nil || s.HardStops != 1 || s.State != ContextCounting || len(exhaustedAlerts(t, d)) != 0 {
		t.Fatalf("after N-1: %+v alerts %d", s, len(exhaustedAlerts(t, d)))
	}
	hardStop(d, "w2", "impl", k1, "CONTEXT")
	alerts := exhaustedAlerts(t, d)
	if s := streakOf(t, d, k1, "impl"); s == nil || s.HardStops != 2 || s.State != ContextExhausted || s.LastWorker != "w2" {
		t.Fatalf("after N: %+v", s)
	}
	if len(alerts) != 1 || alerts[0].Detail["role"] != "impl" || alerts[0].Detail["hardStops"] != "2" || alerts[0].Detail["exhaustedAfter"] != "2" || alerts[0].Ticket != k1 {
		t.Fatalf("alerts %+v", alerts)
	}
	hardStop(d, "w3", "impl", k1, "CONTEXT")
	if s := streakOf(t, d, k1, "impl"); s.HardStops != 3 || s.State != ContextExhausted || len(exhaustedAlerts(t, d)) != 1 {
		t.Fatalf("a later stop re-alerted or lost the state: %+v", s)
	}

	// The default N is 3, per role and per ticket; another ending resets.
	hardStop(d, "r1", "review", k1, "CONTEXT")
	hardStop(d, "r2", "review", k1, "CONTEXT")
	hardStop(d, "r3", "review", k2, "CONTEXT")
	if s := streakOf(t, d, k1, "review"); s.HardStops != 2 || s.State != ContextCounting || len(exhaustedAlerts(t, d)) != 1 {
		t.Fatalf("review after 2 of 3: %+v", s)
	}
	hardStop(d, "r4", "review", k1, "")
	if s := streakOf(t, d, k1, "review"); s != nil {
		t.Fatalf("an ordinary ending did not reset: %+v", s)
	}
	hardStop(d, "l1", "impl", "lane:pool/m1", "CONTEXT")
	if s := streakOf(t, d, "lane:pool/m1", "impl"); s != nil {
		t.Fatalf("a lane key was tracked: %+v", s)
	}

	// A restarted dispatcher continues the count from the record.
	d.Close()
	d2, err := Open("ctx", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d2.Close() })
	d2.Now = d.Now
	hardStop(d2, "r5", "review", k2, "CONTEXT")
	if s := streakOf(t, d2, k2, "review"); s == nil || s.HardStops != 2 {
		t.Fatalf("after restart: %+v", s)
	}
}

// ATR-V0-106: exhaustedAfter is 1..100 and needs hardTokens.
func TestATRV0106_ExhaustedAfterValidation(t *testing.T) {
	for raw, ok := range map[string]bool{
		`{"hardTokens":4000,"exhaustedAfter":5}`:   true,
		`{"hardTokens":4000}`:                      true,
		`{"hardTokens":4000,"exhaustedAfter":100}`: true,
		`{"hardTokens":4000,"exhaustedAfter":101}`: false,
		`{"hardTokens":4000,"exhaustedAfter":0}`:   false,
		`{"warnTokens":1000,"exhaustedAfter":3}`:   false,
		`{"exhaustedAfter":3}`:                     false,
	} {
		var c ContextLimits
		err := json.Unmarshal([]byte(raw), &c)
		if err == nil {
			r := &Role{UsageFormat: "claude-code", Context: &c}
			err = r.validateContext(Heal{Handoff: true})
		}
		if (err == nil) != ok {
			t.Errorf("%s: err %v, want accepted %v", raw, err, ok)
		}
	}
	if n := (&ContextLimits{HardTokens: 1}).exhaustedAfter(); n != 3 {
		t.Fatalf("default exhaustedAfter %d", n)
	}
}

// ATR-V0-107: a newer-version record is kept and disables counting after
// one alert; a damaged record is replaced after one alert.
func TestATRV0107_RecordVersions(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Context = &ContextLimits{HardTokens: 4000}
	d, _ := openCtx(t, &fakeQueue{}, c)
	path := ContextExhaustedPath(d.dir)
	newer := []byte(`{"profile":"taskman-dispatch-context-exhausted/1","program":"ctx","entries":[]}` + "\n")
	if err := os.WriteFile(path, newer, 0o600); err != nil {
		t.Fatal(err)
	}
	hardStop(d, "w1", "impl", "ticket:a:q:t1", "CONTEXT")
	hardStop(d, "w2", "impl", "ticket:a:q:t1", "CONTEXT")
	if raw, _ := os.ReadFile(path); string(raw) != string(newer) {
		t.Fatalf("the newer record was overwritten: %s", raw)
	}
	if alerts := eventsOf(t, d, "alert"); len(alerts) != 1 || !strings.Contains(alerts[0].Message, "newer version") {
		t.Fatalf("alerts %+v", alerts)
	}

	d2, _ := openCtx(t, &fakeQueue{}, testConfig(t, "exit 0"))
	d2.Config.Roles[0].Context = &ContextLimits{HardTokens: 4000}
	path = ContextExhaustedPath(d2.dir)
	if err := os.WriteFile(path, []byte(`{"profile":"taskman-dispatch-context-exhausted/0","program":`), 0o600); err != nil {
		t.Fatal(err)
	}
	hardStop(d2, "w1", "impl", "ticket:a:q:t1", "CONTEXT")
	hardStop(d2, "w2", "impl", "ticket:a:q:t1", "CONTEXT")
	if s := streakOf(t, d2, "ticket:a:q:t1", "impl"); s == nil || s.HardStops != 2 {
		t.Fatalf("the damaged record was not replaced: %+v", s)
	}
	if alerts := eventsOf(t, d2, "alert"); len(alerts) != 1 || !strings.Contains(alerts[0].Message, "is replaced") {
		t.Fatalf("alerts %+v", alerts)
	}
}
