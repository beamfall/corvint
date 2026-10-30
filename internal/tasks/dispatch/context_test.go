//go:build darwin || linux

package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// claudeAssistant is a main-session Claude Code assistant event whose usage
// sums to tokens: input 10, cache creation 20, cache read and output.
func claudeAssistant(cacheRead, output uint64) string {
	return `{"type":"assistant","message":{"id":"m","type":"message","role":"assistant","content":[],"usage":{"input_tokens":10,"cache_creation_input_tokens":20,"cache_read_input_tokens":` + itoa(cacheRead) + `,"output_tokens":` + itoa(output) + `,"service_tier":"standard"}},"parent_tool_use_id":null,"session_id":"s"}`
}

func itoa(n uint64) string { b, _ := json.Marshal(n); return string(b) }

// contextOf reads out as one worker's whole stdout.log and returns the last
// context observation: seen, known and the estimate.
func contextOf(t *testing.T, format, out string) (bool, bool, uint64) {
	t.Helper()
	u := usageOf(t, format, out)
	return u.ctxSeen, u.ctxKnown, u.ctxTokens
}

func TestATRV0031_ContextEstimateFromRecognisedLinesOnly(t *testing.T) {
	main := claudeAssistant(30, 5) // 65
	sub := strings.Replace(claudeAssistant(900000, 5), `"parent_tool_use_id":null`, `"parent_tool_use_id":"toolu_1"`, 1)
	type want struct {
		seen, known bool
		tokens      uint64
	}
	cases := []struct {
		name, format, out string
		want
	}{
		{"claude main assistant", "claude-code", main + "\n", want{true, true, 65}},
		{"claude latest call replaces, never sums", "claude-code", main + "\n" + claudeAssistant(1000, 7) + "\n", want{true, true, 1037}},
		{"claude subagent message leaves it", "claude-code", main + "\n" + sub + "\n", want{true, true, 65}},
		{"claude subagent alone is no observation", "claude-code", sub + "\n", want{}},
		{"claude result total is not context", "claude-code", claudeResult + "\n", want{}},
		{"claude other events", "claude-code", `{"type":"system","subtype":"init"}` + "\n" + `{"type":"user","message":{"role":"user"},"parent_tool_use_id":null}` + "\n", want{}},
		{"claude without parent_tool_use_id", "claude-code", main + "\n" + strings.Replace(main, `,"parent_tool_use_id":null`, "", 1) + "\n", want{true, false, 0}},
		{"claude null counter", "claude-code", main + "\n" + strings.Replace(main, `"output_tokens":5`, `"output_tokens":null`, 1) + "\n", want{true, false, 0}},
		{"claude null cache counter", "claude-code", strings.Replace(main, `"cache_read_input_tokens":30`, `"cache_read_input_tokens":null`, 1) + "\n", want{true, false, 0}},
		{"claude missing output counter", "claude-code", strings.Replace(main, `,"output_tokens":5`, "", 1) + "\n", want{true, false, 0}},
		{"claude optional cache counters absent", "claude-code", strings.Replace(strings.Replace(main, `"cache_creation_input_tokens":20,`, "", 1), `"cache_read_input_tokens":30,`, "", 1) + "\n", want{true, true, 15}},
		{"claude fractional counter", "claude-code", strings.Replace(main, `"output_tokens":5`, `"output_tokens":5.5`, 1) + "\n", want{true, false, 0}},
		{"claude repeated counter", "claude-code", strings.Replace(main, `"output_tokens":5`, `"output_tokens":5,"output_tokens":0`, 1) + "\n", want{true, false, 0}},
		{"claude case-folded usage", "claude-code", strings.Replace(main, `"content":[]`, `"content":[],"USAGE":{}`, 1) + "\n", want{true, false, 0}},
		{"claude case-folded counter", "claude-code", strings.Replace(main, `"input_tokens":10`, `"input_tokens":10,"Output_Tokens":0`, 1) + "\n", want{true, false, 0}},
		{"claude case-folded top member", "claude-code", strings.Replace(main, `"session_id":"s"`, `"session_id":"s","Message":{}`, 1) + "\n", want{true, false, 0}},
		{"claude repeated type hides an assistant", "claude-code", main + "\n" + `{"type":"assistant","type":"user"}` + "\n", want{true, false, 0}},
		{"claude overflowing sum", "claude-code", strings.Replace(main, `"output_tokens":5`, `"output_tokens":18446744073709551615`, 1) + "\n", want{true, false, 0}},
		{"claude torn assistant line", "claude-code", main + "\n" + main[:40] + "\n", want{true, false, 0}},
		{"claude plain text leaves it", "claude-code", main + "\nplain text\n", want{true, true, 65}},
		{"opencode step", "opencode", openCodeStart + "\n" + openCodeStep("tool-calls") + "\n", want{true, true, 8}},
		{"opencode latest step replaces", "opencode", openCodeStep("tool-calls") + "\n" + strings.Replace(openCodeStep("stop"), `"input":5`, `"input":500`, 1) + "\n", want{true, true, 503}},
		{"opencode malformed step", "opencode", openCodeStep("stop") + "\n" + `{"type":"step_finish","part":{}}` + "\n", want{true, false, 0}},
		{"opencode other events", "opencode", openCodeStart + "\n", want{}},
		{"codex never reports context", "codex", codexTurn + "\n" + codexTurn + "\n", want{}},
	}
	for _, tc := range cases {
		seen, known, tokens := contextOf(t, tc.format, tc.out)
		if seen != tc.seen || known != tc.known || tokens != tc.tokens && tc.known {
			t.Errorf("%s: seen %v known %v tokens %d, want %+v", tc.name, seen, known, tokens, tc.want)
		}
	}
	// A line longer than the read bound may be a context line: UNKNOWN.
	long := `{"type":"assistant","pad":"` + strings.Repeat("x", maxUsageLine) + `"}`
	if seen, known, _ := contextOf(t, "claude-code", main+"\n"+long+"\n"); !seen || known {
		t.Errorf("an oversize line kept the estimate: seen %v known %v", seen, known)
	}
}

func TestATRV0030_ContextConfigIsExactAndBounded(t *testing.T) {
	base := func() *Config {
		c := testConfig(t, "exit 0")
		c.Roles[0].UsageFormat = "claude-code"
		c.Roles[0].Context = &ContextLimits{WarnTokens: 100, HandoffTokens: 200, HardTokens: 300}
		return c
	}
	raw, _ := json.Marshal(base())
	c, err := DecodeConfig(raw)
	if err != nil {
		t.Fatalf("valid context refused: %v", err)
	}
	if got := *c.Roles[0].Context; got != (ContextLimits{WarnTokens: 100, HandoffTokens: 200, HardTokens: 300}) {
		t.Fatalf("decoded %+v", got)
	}
	const member = `"context":{"warnTokens":100,"handoffTokens":200,"hardTokens":300}`
	if !strings.Contains(string(raw), member) {
		t.Fatalf("encoding %s", raw)
	}
	for name, ctx := range map[string]string{
		"only warn":        `{"warnTokens":1}`,
		"only hard":        `{"hardTokens":1000000000}`,
		"warn and hard":    `{"warnTokens":5,"hardTokens":6}`,
		"handoff and hard": `{"handoffTokens":5,"hardTokens":600}`,
	} {
		if _, err := DecodeConfig([]byte(strings.Replace(string(raw), member, `"context":`+ctx, 1))); err != nil {
			t.Errorf("%s refused: %v", name, err)
		}
	}
	for name, ctx := range map[string]string{
		"empty":          `{}`,
		"null":           `null`,
		"array":          `[]`,
		"unknown member": `{"warnTokens":1,"softTokens":2}`,
		"repeated":       `{"warnTokens":1,"warnTokens":2}`,
		"case-folded":    `{"WarnTokens":1}`,
		"null member":    `{"warnTokens":null}`,
		"zero":           `{"warnTokens":0}`,
		"negative":       `{"warnTokens":-1}`,
		"fraction":       `{"warnTokens":1.5}`,
		"string":         `{"warnTokens":"1"}`,
		"over the bound": `{"warnTokens":1000000001}`,
		"equal":          `{"warnTokens":5,"handoffTokens":5}`,
		"descending":     `{"handoffTokens":6,"hardTokens":5}`,
	} {
		if _, err := DecodeConfig([]byte(strings.Replace(string(raw), member, `"context":`+ctx, 1))); err == nil {
			t.Errorf("%s context accepted", name)
		}
	}
	for name, mutate := range map[string]func(*Config){
		"codex format":      func(c *Config) { c.Roles[0].UsageFormat = "codex" },
		"no usage format":   func(c *Config) { c.Roles[0].UsageFormat = "" },
		"hard without heal": func(c *Config) { c.Heal.Handoff = false },
	} {
		c := base()
		mutate(c)
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	c = base()
	c.Heal.Handoff = false
	c.Roles[0].Context.HardTokens = 0
	raw, _ = json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Errorf("warn and handoff without heal.handoff refused: %v", err)
	}
}

func TestATRV0033_CrossingsRecordedOncePerSession(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].UsageFormat = "claude-code"
	c.Roles[0].Context = &ContextLimits{WarnTokens: 100, HandoffTokens: 200, HardTokens: 300}
	d, err := Open("ctx", c, &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	w := &Worker{ID: "w1", Role: "impl", Ticket: "ticket:a:q:t1", Usage: &WorkerUsage{Format: "claude-code"}}
	if err := os.MkdirAll(d.workerDir(w.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	step := func(tokens uint64, known, observe bool) bool {
		t.Helper()
		if observe {
			w.Usage.observeContext(tokens, known)
		}
		now = now.Add(time.Second)
		return d.noteContext(w, now)
	}
	levels := func() []string {
		var out []string
		for _, e := range eventsOf(t, d, "context") {
			out = append(out, e.Detail["level"]+"@"+e.Detail["tokens"])
		}
		return out
	}
	if step(0, false, false) {
		t.Fatal("no observation stopped the worker")
	}
	if rec, err := ReadContextRecord(d.dir, w.ID); err != nil || rec.Tokens != UsageUnknown || rec.ObservedAt != UsageUnknown || rec.HardTokens != "300" {
		t.Fatalf("first record %+v %v", rec, err)
	}
	step(50, true, true)
	step(150, true, true)
	step(0, false, false) // the same estimate on a later tick
	step(90, true, true)  // a smaller estimate does not re-arm WARN
	step(120, true, true)
	step(0, false, true) // UNKNOWN crosses nothing
	if got := strings.Join(levels(), ","); got != "WARN@150" {
		t.Fatalf("crossings %s", got)
	}
	rec, err := ReadContextRecord(d.dir, w.ID)
	if err != nil || rec.Tokens != UsageUnknown || rec.Level != ContextWarn || len(rec.Crossings) != 1 || rec.ObservedAt != now.Format(time.RFC3339) {
		t.Fatalf("record %+v %v", rec, err)
	}
	step(250, true, true)
	// A restart reloads the record: nothing already crossed is reported.
	d.contexts = nil
	step(260, true, true)
	if got := strings.Join(levels(), ","); got != "WARN@150,HANDOFF@250" {
		t.Fatalf("crossings after restart %s", got)
	}

	// A record that cannot be written alerts once, records no crossing and
	// emits no event; the hard decision still follows the estimate.
	dir := d.workerDir(w.ID)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	alerts := len(eventsOf(t, d, "alert"))
	if !step(350, true, true) || !step(0, false, false) {
		t.Fatal("the hard estimate did not decide a stop while the record was unwritable")
	}
	os.Chmod(dir, 0o700)
	if n := len(eventsOf(t, d, "alert")) - alerts; n != 1 {
		t.Fatalf("%d alerts for one write error", n)
	}
	if got := strings.Join(levels(), ","); got != "WARN@150,HANDOFF@250" {
		t.Fatalf("a crossing was reported before its record was written: %s", got)
	}
	step(0, false, false)
	if got := strings.Join(levels(), ","); got != "WARN@150,HANDOFF@250,HARD@350" {
		t.Fatalf("crossings after the write recovered %s", got)
	}
	if rec, _ := ReadContextRecord(d.dir, w.ID); rec == nil || rec.Level != ContextHard || len(rec.Crossings) != 3 {
		t.Fatalf("record %+v", rec)
	}

	// A damaged record alerts once and is replaced by a fresh one.
	if err := os.WriteFile(ContextRecordPath(d.dir, w.ID), []byte(`{"profile":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d.contexts = nil
	alerts = len(eventsOf(t, d, "alert"))
	step(10, true, true)
	if n := len(eventsOf(t, d, "alert")) - alerts; n != 1 {
		t.Fatalf("%d alerts for a damaged record", n)
	}
	if rec, err := ReadContextRecord(d.dir, w.ID); err != nil || rec.Tokens != "10" || rec.Level != ContextNone {
		t.Fatalf("replaced record %+v %v", rec, err)
	}

	// The record reader refuses anything but the canonical closed form.
	good, _ := os.ReadFile(ContextRecordPath(d.dir, w.ID))
	for name, raw := range map[string]string{
		"extra member":   strings.Replace(string(good), `"crossings"`, `"x":1,"crossings"`, 1),
		"no newline":     strings.TrimSuffix(string(good), "\n"),
		"other worker":   strings.Replace(string(good), `"worker":"w1"`, `"worker":"w2"`, 1),
		"zero-padded":    strings.Replace(string(good), `"tokens":"10"`, `"tokens":"010"`, 1),
		"level mismatch": strings.Replace(string(good), `"level":"NONE"`, `"level":"WARN"`, 1),
	} {
		os.WriteFile(ContextRecordPath(d.dir, w.ID), []byte(raw), 0o600)
		if _, err := ReadContextRecord(d.dir, w.ID); err == nil {
			t.Errorf("%s record accepted", name)
		}
	}
	if _, err := ReadContextRecord(d.dir, "absent"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("absent record: %v", err)
	}
}

// contextRig launches one real worker on t1 that prints out to standard
// output and then sleeps, under a fixed clock, holding live attempt a1.
func contextRig(t *testing.T, q *fakeQueue, out string, configure func(*Config)) (*Dispatcher, *Worker, *time.Time) {
	t.Helper()
	c := testConfig(t, "printf '%s\\n' '"+out+"'\nexec sleep 300")
	c.GlobalCap = 1
	c.Backoff.CooldownSeconds = 3600
	c.Roles[0].IdleSeconds, c.Roles[0].WallSeconds = 3600, 7200
	c.Roles[0].UsageFormat = "claude-code"
	if configure != nil {
		configure(c)
	}
	q.obs.Tickets = []Ticket{ticket("t1", "P1", 1)}
	d, err := Open("ctx", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	d.Now = func() time.Time { return now }
	tick(t, d)
	if d.Running() != 1 {
		t.Fatalf("running %d, want one launched worker", d.Running())
	}
	w := d.ledger.Workers[0]
	pid := w.PID
	t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL); syscall.Kill(pid, syscall.SIGKILL) })
	q.obs.Attempts = []Attempt{{ID: "a1", Ticket: w.Ticket, Holder: w.ID, Phase: "RUNNING", Generation: "3", Live: true, LeaseExpires: now.Add(time.Hour)}}
	return d, w, &now
}

// tickUntil ticks until done or about thirty seconds pass (the host may be
// loaded).
func tickUntil(t *testing.T, d *Dispatcher, done func() bool) {
	t.Helper()
	for i := 0; i < 600 && !done(); i++ {
		tick(t, d)
		time.Sleep(50 * time.Millisecond)
	}
	if !done() {
		t.Fatal("condition not reached")
	}
}

func ctxObserved(d *Dispatcher, w *Worker) func() bool {
	return func() bool { st := d.contexts[w.ID]; return st != nil && st.rec.ObservedAt != UsageUnknown }
}

func TestATRV0034_WorkerGetsContextFile(t *testing.T) {
	q := &fakeQueue{}
	c := testConfig(t, `printf '%s\n' "${CORVINT_DISPATCH_CONTEXT_FILE-unset}" >&2; exec sleep 300`)
	c.GlobalCap = 2
	c.Roles[0].IdleSeconds, c.Roles[0].WallSeconds = 3600, 7200
	c.Roles[0].UsageFormat = "claude-code"
	c.Roles[0].Match = &Match{Labels: []string{"ctx"}}
	c.Roles = append(c.Roles, c.Roles[0])
	c.Roles[1].Name, c.Roles[1].UsageFormat, c.Roles[1].Match = "plain", "codex", &Match{Labels: []string{"plain"}}
	q.obs.Tickets = []Ticket{ticket("t1", "P1", 1, "ctx"), ticket("t2", "P1", 2, "plain")}
	d, err := Open("ctx", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	tick(t, d)
	if d.Running() != 2 {
		t.Fatalf("running %d", d.Running())
	}
	for _, w := range d.ledger.Workers {
		pid := w.PID
		t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL); syscall.Kill(pid, syscall.SIGKILL) })
	}
	stderr := func(w *Worker) string {
		b, _ := os.ReadFile(filepath.Join(d.workerDir(w.ID), "stderr.log"))
		return strings.TrimSpace(string(b))
	}
	tickUntil(t, d, func() bool { return stderr(d.ledger.Workers[0]) != "" && stderr(d.ledger.Workers[1]) != "" })
	// tickUntil may return without ticking: a fast worker writes stderr
	// before any later tick, and the launching tick supervised before it
	// launched. The record is promised only after a supervising tick of the
	// running worker (ATR-V0-034), so take one.
	tick(t, d)
	for _, w := range d.ledger.Workers {
		want := "unset"
		if w.Role == "impl" {
			want = ContextRecordPath(d.dir, w.ID)
			if !filepath.IsAbs(want) {
				t.Fatalf("context file %s is not absolute", want)
			}
			if _, err := ReadContextRecord(d.dir, w.ID); err != nil {
				t.Fatalf("no record after a supervising tick: %v", err)
			}
		} else if _, err := os.Stat(ContextRecordPath(d.dir, w.ID)); err == nil {
			t.Fatal("a codex worker has a context record")
		}
		if got := stderr(w); got != want {
			t.Errorf("%s worker saw CORVINT_DISPATCH_CONTEXT_FILE=%q, want %q", w.Role, got, want)
		}
	}
}

func TestATRV0035_HardStopHandsOffWithReason(t *testing.T) {
	q := &fakeQueue{}
	d, w, _ := contextRig(t, q, claudeAssistant(4970, 0), func(c *Config) {
		c.Roles[0].Context = &ContextLimits{WarnTokens: 1000, HardTokens: 4000}
	})
	pid := w.PID
	tickUntil(t, d, func() bool { return d.Running() == 0 })
	if !gone(pid) {
		t.Fatal("the worker survived its hard context threshold")
	}
	killing := eventsOf(t, d, "killing")
	if len(killing) != 1 || killing[0].Detail["reason"] != "CONTEXT" || !strings.Contains(killing[0].Message, "hardTokens") {
		t.Fatalf("killing events %+v", killing)
	}
	handoff := eventsOf(t, d, "handoff")
	if len(handoff) != 1 || handoff[0].Detail["attempt"] != "a1" || handoff[0].Detail["reason"] != "CONTEXT" || handoff[0].Detail["contextTokens"] != "5000" || handoff[0].Detail["hardTokens"] != "4000" {
		t.Fatalf("handoff events %+v", handoff)
	}
	if len(q.released) != 1 || q.released[0] != "a1" || q.evidence[0] != "dispatch:"+w.ID {
		t.Fatalf("released %v evidence %v", q.released, q.evidence)
	}
	finished := eventsOf(t, d, "finished")
	if len(finished) != 1 || finished[0].Detail["killReason"] != "CONTEXT" {
		t.Fatalf("finished events %+v", finished)
	}
	var levels []string
	for _, e := range eventsOf(t, d, "context") {
		if e.Detail["note"] == "" { // ATR-V0-104's salvage event has no level
			levels = append(levels, e.Detail["level"])
		}
	}
	if strings.Join(levels, ",") != "WARN,HARD" {
		t.Fatalf("context events %v", levels)
	}
}

func TestATRV0035_UnknownEstimateNeverStops(t *testing.T) {
	// A null counter, then a subagent's large message: the estimate stays
	// UNKNOWN and a hard threshold of one token never stops the worker.
	sub := strings.Replace(claudeAssistant(900000, 5), `"parent_tool_use_id":null`, `"parent_tool_use_id":"toolu_1"`, 1)
	out := strings.Replace(claudeAssistant(30, 5), `"output_tokens":5`, `"output_tokens":null`, 1) + "\n" + sub
	q := &fakeQueue{}
	d, w, now := contextRig(t, q, out, func(c *Config) { c.Roles[0].Context = &ContextLimits{HardTokens: 1} })
	tickUntil(t, d, ctxObserved(d, w))
	for i := 0; i < 3; i++ {
		*now = now.Add(time.Second)
		tick(t, d)
	}
	if d.Running() != 1 || w.State != "RUNNING" || len(eventsOf(t, d, "killing")) != 0 || len(eventsOf(t, d, "context")) != 0 {
		t.Fatalf("an unknown estimate acted: running %d state %s", d.Running(), w.State)
	}
	if rec, err := ReadContextRecord(d.dir, w.ID); err != nil || rec.Tokens != UsageUnknown || rec.Level != ContextNone {
		t.Fatalf("record %+v %v", rec, err)
	}
}

func TestATRV0036_ThresholdsHotReload(t *testing.T) {
	q := &fakeQueue{}
	c := testConfig(t, "printf '%s\\n' '"+claudeAssistant(4970, 0)+"'\nexec sleep 300")
	c.GlobalCap = 1
	c.Backoff.CooldownSeconds = 3600
	c.Roles[0].IdleSeconds, c.Roles[0].WallSeconds = 3600, 7200
	c.Roles[0].UsageFormat = "claude-code"
	c.Roles[0].Context = &ContextLimits{WarnTokens: 10000}
	q.obs.Tickets = []Ticket{ticket("t1", "P1", 1)}
	f := newReloadFixture(t, c, q)
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	f.d.Now = func() time.Time { return now }
	f.tick()
	w := f.d.ledger.Workers[0]
	pid := w.PID
	t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL); syscall.Kill(pid, syscall.SIGKILL) })
	q.obs.Attempts = []Attempt{{ID: "a1", Ticket: w.Ticket, Holder: w.ID, Phase: "RUNNING", Generation: "3", Live: true, LeaseExpires: now.Add(time.Hour)}}
	tickUntil(t, f.d, ctxObserved(f.d, w))
	if n := len(f.events("context")); n != 0 {
		t.Fatalf("%d crossings below the threshold", n)
	}
	reload := func(ctx *ContextLimits) {
		t.Helper()
		next := cloneConfig(t, c)
		next.Roles[0].Context = ctx
		f.write(next)
		now = now.Add(time.Second)
		f.tick()
	}
	// Lowered thresholds apply to the running worker at the next tick.
	reload(&ContextLimits{WarnTokens: 1000, HandoffTokens: 2000})
	if got := f.events("context"); len(got) != 2 || got[0].Detail["level"] != ContextWarn || got[1].Detail["level"] != ContextHandoff || w.State != "RUNNING" {
		t.Fatalf("context events %+v state %s", got, w.State)
	}
	// Removing them keeps the recorded crossings and shows no thresholds.
	reload(nil)
	rec, err := ReadContextRecord(f.d.dir, w.ID)
	if err != nil || rec.Level != ContextHandoff || len(rec.Crossings) != 2 || rec.WarnTokens != ContextNone || rec.HardTokens != ContextNone {
		t.Fatalf("record after removal %+v %v", rec, err)
	}
	// A reload setting hardTokens under the estimate stops the worker (the
	// CAL-V0-127 exception) and reports only the new level.
	reload(&ContextLimits{WarnTokens: 1000, HandoffTokens: 2000, HardTokens: 4000})
	tickUntil(t, f.d, func() bool { return f.d.Running() == 0 })
	var levels []string
	for _, e := range f.events("context") {
		if e.Detail["note"] == "" { // ATR-V0-104's salvage event has no level
			levels = append(levels, e.Detail["level"])
		}
	}
	if strings.Join(levels, ",") != "WARN,HANDOFF,HARD" {
		t.Fatalf("context events %v", levels)
	}
	if h := f.events("handoff"); len(h) != 1 || h[0].Detail["reason"] != "CONTEXT" || h[0].Detail["hardTokens"] != "4000" {
		t.Fatalf("handoff %+v", h)
	}
}

// contextDispatcher opens a dispatcher whose impl role reads claude-code
// context under limits, with worker w1's directory created.
func contextDispatcher(t *testing.T, limits *ContextLimits) (*Dispatcher, *Worker) {
	t.Helper()
	c := testConfig(t, "exit 0")
	c.Roles[0].UsageFormat = "claude-code"
	c.Roles[0].Context = limits
	d, err := Open("ctx", c, &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	w := &Worker{ID: "w1", Role: "impl", Ticket: "ticket:a:q:t1", Usage: &WorkerUsage{Format: "claude-code"}}
	if err := os.MkdirAll(d.workerDir(w.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	return d, w
}

// ATR-V0-037: a line whose repeated or case-folded member hides an escaped
// step_finish type, and output the read never reached (a CAL-V0-143 cut or
// a file cut by something else), leave the estimate UNKNOWN, so an earlier
// high estimate stops nothing.
func TestATRV0037_HiddenOrLostContextLinesAreUnknown(t *testing.T) {
	high := strings.Replace(openCodeStep("tool-calls"), `"input":5`, `"input":5000`, 1)
	hidden := strings.Replace(openCodeStep("stop"), `"type":"step_finish"`, `"type":"step\u005ffinish","sessionID":"ses_1"`, 1)
	if strings.Contains(hidden, `"step_finish"`) {
		t.Fatal("the hidden type is not escaped")
	}
	if seen, known, _ := contextOf(t, "opencode", high+"\n"+hidden+"\n"); !seen || known {
		t.Errorf("a hidden step_finish kept the estimate: seen %v known %v", seen, known)
	}
	if seen, known, n := contextOf(t, "opencode", high+"\n"+openCodeStart+"\n"); !seen || !known || n != 5003 {
		t.Errorf("an ordinary event changed the estimate: seen %v known %v tokens %d", seen, known, n)
	}

	dir := t.TempDir()
	log := filepath.Join(dir, "stdout.log")
	main := claudeAssistant(4970, 0)
	if err := os.WriteFile(log, []byte(main+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	u := &WorkerUsage{Format: "claude-code"}
	readUsage(dir, u)
	if !u.ctxKnown || u.ctxTokens != 5000 {
		t.Fatalf("estimate %v %d", u.ctxKnown, u.ctxTokens)
	}
	u.cut()
	if !u.ctxSeen || u.ctxKnown {
		t.Errorf("a cut kept the estimate: seen %v known %v", u.ctxSeen, u.ctxKnown)
	}
	u = &WorkerUsage{Format: "claude-code"}
	readUsage(dir, u)
	if err := os.WriteFile(log, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	readUsage(dir, u)
	if !u.ctxSeen || u.ctxKnown {
		t.Errorf("a shrunk log kept the estimate: seen %v known %v", u.ctxSeen, u.ctxKnown)
	}

	// The supervising decision: the known estimate stops, the cut one not.
	d, w := contextDispatcher(t, &ContextLimits{HardTokens: 4000})
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	w.Usage.observeContext(5000, true)
	w.Usage.cut()
	if d.noteContext(w, now) {
		t.Fatal("an estimate whose newer output was lost stopped the worker")
	}
	w.Usage.observeContext(5000, true)
	if !d.noteContext(w, now.Add(time.Second)) {
		t.Fatal("a known estimate over hardTokens did not stop the worker")
	}
}

// ATR-V0-038: after a restart, a reloaded record's estimate may predate an
// observation whose record write failed while the ledger's usage offset
// moved past it, so it crosses nothing and stops nothing until a line read
// by the restarted run replaces it; it is still shown.
func TestATRV0038_ReloadedEstimateActsOnlyAfterAFreshLine(t *testing.T) {
	d, w := contextDispatcher(t, &ContextLimits{WarnTokens: 100})
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	step := func(tokens uint64, observe bool) bool {
		t.Helper()
		if observe {
			w.Usage.observeContext(tokens, true)
		}
		now = now.Add(time.Second)
		return d.noteContext(w, now)
	}
	step(250, true)
	// A compacted, smaller estimate cannot be written; then a restart with
	// a hardTokens below the stale 250.
	dir := d.workerDir(w.ID)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	step(50, true)
	os.Chmod(dir, 0o700)
	d.contexts = nil
	d.Config.Roles[0].Context = &ContextLimits{WarnTokens: 100, HardTokens: 200}
	if step(0, false) || step(0, false) {
		t.Fatal("a reloaded estimate stopped the worker")
	}
	rec, err := ReadContextRecord(d.dir, w.ID)
	if err != nil || rec.Tokens != "250" || rec.crossed(ContextHard) != nil || rec.HardTokens != "200" {
		t.Fatalf("record after restart %+v %v", rec, err)
	}
	if !step(260, true) {
		t.Fatal("a fresh estimate over hardTokens did not stop the worker")
	}
	var levels []string
	for _, e := range eventsOf(t, d, "context") {
		if e.Detail["note"] == "" { // ATR-V0-104's salvage event has no level
			levels = append(levels, e.Detail["level"]+"@"+e.Detail["tokens"])
		}
	}
	if got := strings.Join(levels, ","); got != "WARN@250,HARD@260" {
		t.Fatalf("crossings %s", got)
	}
}

// writeContextRecord writes a record for w1 with the estimate and hard
// threshold, as a context-stopped worker leaves it.
func writeContextRecord(t *testing.T, d *Dispatcher, tokens, hard string) {
	t.Helper()
	rec := freshContextRecord("w1")
	rec.Tokens, rec.ObservedAt, rec.HardTokens = tokens, "2026-10-17T09:00:00Z", hard
	if err := os.MkdirAll(d.workerDir("w1"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(ContextRecordPath(d.dir, "w1"), rec.encode()); err != nil {
		t.Fatal(err)
	}
}

// ATR-V0-035: the hand-off of a context-stopped worker keeps reason,
// contextTokens and hardTokens on every exit-recovery event after a refused
// release, and on a hand-off that waited for a detached run.
func TestATRV0035_DeferredAndRecoveredHandoffsKeepContext(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	q := &refusingQueue{fakeQueue: &fakeQueue{}, releaseFails: 2}
	d := exitRig(t, q, now.Add(24*time.Hour), &now, nil)
	d.ledger.Workers[0].KillReason = "CONTEXT"
	writeContextRecord(t, d, "5000", "4000")
	for i := 0; i < 3; i++ {
		tick(t, d)
		now = now.Add(10 * time.Minute)
	}
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	for _, e := range events {
		if e.Kind != "handoff" && e.Kind != "handoff-refused" {
			continue
		}
		seen = append(seen, e.Kind+":"+e.Detail["recovery"])
		if e.Detail["reason"] != "CONTEXT" || e.Detail["contextTokens"] != "5000" || e.Detail["hardTokens"] != "4000" {
			t.Errorf("%s event without the context detail: %+v", e.Kind, e.Detail)
		}
	}
	if got := strings.Join(seen, ","); got != "handoff-refused:RETRYING,handoff-refused:RETRYING,handoff:RELEASED" {
		t.Fatalf("hand-off events %s", got)
	}

	// A hand-off deferred for a detached run (here re-found from its marker
	// after a restart) carries the detail when it is finally made.
	rq := &runQueue{fakeQueue: &fakeQueue{}, runs: map[string][]DetachedRun{}}
	c := testConfig(t, "exit 0")
	c.Roles[0].Match.Labels = []string{"do-not-launch"}
	rq.obs.Tickets = []Ticket{ticket("t1", "P1", 1)}
	dd, err := Open("ctx", c, rq, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dd.Close() })
	tk := rq.obs.Tickets[0].ID
	dd.ledger.Workers = append(dd.ledger.Workers, &Worker{ID: "w1", Role: "impl", Host: "sh", Key: tk, Ticket: tk, PID: -1, Started: now, LastActive: now, KillReason: "CONTEXT"})
	rq.obs.Attempts = []Attempt{{ID: "a1", Ticket: tk, Holder: "w1", Phase: "RUNNING", Generation: "7", Live: true, LeaseExpires: now.Add(time.Hour)}}
	writeContextRecord(t, dd, "5000", "4000")
	dd.detached = map[string]*detachedMarker{testRunID: {Profile: detachedProfile, RunID: testRunID, AttemptID: "a1", Generation: "7", Ticket: tk, Role: "impl", Worker: "w1", Phase: DetachedDeferred, DeferredAt: now, DeferUntil: now.Add(time.Hour)}}
	tick(t, dd)
	h := eventsWith(t, dd, "handoff", "detachedRun", testRunID)
	if len(h) != 1 || h[0].Detail["reason"] != "CONTEXT" || h[0].Detail["contextTokens"] != "5000" || h[0].Detail["hardTokens"] != "4000" || len(rq.released) != 1 {
		t.Fatalf("deferred hand-off %+v released %v", h, rq.released)
	}
	if len(dd.deferredContext) != 0 {
		t.Fatalf("deferred context kept after the hand-off: %v", dd.deferredContext)
	}
}

func TestATRV0054_NewerContextRecordIsPreserved(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].UsageFormat = "claude-code"
	c.Roles[0].Context = &ContextLimits{WarnTokens: 100, HandoffTokens: 200, HardTokens: 300}
	d, err := Open("ctx", c, &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	w := &Worker{ID: "w1", Role: "impl", Ticket: "ticket:a:q:t1", Usage: &WorkerUsage{Format: "claude-code"}}
	if err := os.MkdirAll(d.workerDir(w.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	newer := []byte(`{"profile":"taskman-dispatch-context/1","worker":"w1","future":true}` + "\n")
	path := ContextRecordPath(d.dir, w.ID)
	if err := os.WriteFile(path, newer, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadContextRecord(d.dir, w.ID); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
		t.Fatalf("newer record read as %v", err)
	}
	step := func(tokens uint64) bool {
		w.Usage.observeContext(tokens, true)
		now = now.Add(time.Second)
		return d.noteContext(w, now)
	}
	// Every threshold is crossed; nothing is written, no crossing is
	// reported, one alert names the record, and the hard stop still follows
	// the estimate.
	if step(150) || step(250) {
		t.Fatal("a stop below hardTokens")
	}
	if !step(350) {
		t.Fatal("the hard estimate did not decide a stop over a newer record")
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, newer) {
		t.Fatalf("the newer record was overwritten: %s", got)
	}
	if n := len(eventsOf(t, d, "context")); n != 0 {
		t.Fatalf("%d crossings reported over a newer record", n)
	}
	alerts := eventsOf(t, d, "alert")
	if len(alerts) != 1 || !strings.Contains(alerts[0].Message, "another build") {
		t.Fatalf("alerts %+v", alerts)
	}
	// A restart alerts once more and still preserves the file.
	d.contexts = nil
	step(360)
	if got, _ := os.ReadFile(path); !bytes.Equal(got, newer) {
		t.Fatalf("the newer record was overwritten after a restart: %s", got)
	}
	if n := len(eventsOf(t, d, "alert")); n != 2 {
		t.Fatalf("%d alerts after a restart", n)
	}
}

func TestATRV0055_CorruptContextRecordIsReplacedAfterOneAlert(t *testing.T) {
	for name, raw := range map[string]string{
		"not JSON":      "{\n",
		"truncated":     `{"profile":"taskman-dispatch-context/0","worker":"w1"`,
		"no profile":    `{"worker":"w1"}` + "\n",
		"non-canonical": `{"profile":"taskman-dispatch-context/0","worker":"w1","tokens":"010"}` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			c := testConfig(t, "exit 0")
			c.Roles[0].UsageFormat = "claude-code"
			c.Roles[0].Context = &ContextLimits{WarnTokens: 100}
			d, err := Open("ctx", c, &fakeQueue{}, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { d.Close() })
			w := &Worker{ID: "w1", Role: "impl", Ticket: "ticket:a:q:t1", Usage: &WorkerUsage{Format: "claude-code"}}
			if err := os.MkdirAll(d.workerDir(w.ID), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ContextRecordPath(d.dir, w.ID), []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
			for _, n := range []uint64{50, 150} {
				w.Usage.observeContext(n, true)
				now = now.Add(time.Second)
				d.noteContext(w, now)
			}
			alerts := eventsOf(t, d, "alert")
			if len(alerts) != 1 || !strings.Contains(alerts[0].Message, "unreadable") {
				t.Fatalf("alerts %+v", alerts)
			}
			rec, err := ReadContextRecord(d.dir, w.ID)
			if err != nil || rec.Tokens != "150" || rec.Level != ContextWarn {
				t.Fatalf("replacement %+v %v", rec, err)
			}
			if n := len(eventsOf(t, d, "context")); n != 1 {
				t.Fatalf("%d crossings after the replacement", n)
			}
		})
	}
}
