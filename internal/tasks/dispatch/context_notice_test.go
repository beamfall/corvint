//go:build darwin || linux

package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ATR-V0-125, ATR-V0-126, ATR-V0-127 (V1-1834, issue 1148): a host that
// declares contextNotices "file" gets the polled notice file; each crossing
// is appended once and marked, so the hook path and the file path never
// repeat a level; status reports FILE, never UNSUPPORTED; an on-demand
// salvage of the running session writes the screened note; and the launched
// and finished events record the delivery mode, the levels delivered and
// the salvage.
func TestATRV0125_NoticeFileDeliversOnAnyHost(t *testing.T) {
	q := &fakeQueue{}
	secret := fakeSecret()
	script := `printf '%s\n' "${CORVINT_DISPATCH_CONTEXT_NOTICE_FILE-unset}" >&2
printf '%s\n' '` + claudeText("progress at step 2\n"+secret) + `' '` + claudeAssistant(1500, 0) + `'
exec sleep 300`
	c := testConfig(t, script)
	h := c.Hosts["sh"]
	h.ContextNotices = ContextNoticesFile
	c.Hosts["sh"] = h
	c.Roles[0].Context = &ContextLimits{WarnTokens: 1000, HandoffTokens: 1200}
	d, now := openCtx(t, q, c)
	tick(t, d)
	if d.Running() != 1 {
		t.Fatalf("running %d", d.Running())
	}
	w := d.ledger.Workers[0]
	killOnCleanup(t, w)
	path := ContextNoticePath(d.dir, w.ID)
	tickUntil(t, d, func() bool {
		raw, _ := os.ReadFile(path)
		return strings.Count(string(raw), "\n") == 2
	})
	if got := stderrOf(d, w.ID); got != path {
		t.Fatalf("notice file env %q, want %q", got, path)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "its WARN threshold of 1000 tokens") || !strings.Contains(string(raw), "its HANDOFF threshold of 1200 tokens") {
		t.Fatalf("notices %q", raw)
	}
	tick(t, d)
	if again, _ := os.ReadFile(path); string(again) != string(raw) {
		t.Fatalf("a delivered level was repeated: %q", again)
	}
	var delivered []string
	for _, e := range eventsOf(t, d, "context") {
		if e.Detail["delivery"] == DeliveryFile {
			delivered = append(delivered, e.Detail["level"])
		}
	}
	if strings.Join(delivered, ",") != "WARN,HANDOFF" {
		t.Fatalf("file delivery events %v", delivered)
	}
	// The hook path sees the levels as delivered and repeats nothing.
	if notice, err := ContextHookNotice(ContextRecordPath(d.dir, w.ID)); err != nil || notice != "" {
		t.Fatalf("hook after file delivery %q %v", notice, err)
	}
	for format, want := range map[string]string{"claude-code": DeliveryFile, "opencode": DeliveryFile} {
		mode, levels, ok := ContextDelivery(d.dir, w.ID, format, &h)
		if !ok || mode != want || strings.Join(levels, ",") != "WARN,HANDOFF" {
			t.Fatalf("%s delivery %s %v %v", format, mode, levels, ok)
		}
	}
	if mode, _, _ := ContextDelivery(d.dir, w.ID, "opencode", &Host{Argv: h.Argv}); mode != DeliveryUnsupported {
		t.Fatalf("opencode without a declared path: %s", mode)
	}
	launched := eventsOf(t, d, "launched")
	if len(launched) != 1 || launched[0].Detail["contextDelivery"] != DeliveryFile || launched[0].Detail["handoffNote"] != "NONE" {
		t.Fatalf("launched %+v", launched)
	}

	// ATR-V0-126: an on-demand salvage of the running session.
	notePath, n, err := SalvageWorker(d.dir, w, *now, "")
	if err != nil || notePath != HandoffNotePath(d.dir, w.ID) {
		t.Fatalf("salvage %s %v", notePath, err)
	}
	if n.ContextTokens != "1530" || n.RedactedLines != 1 || len(n.Messages) != 1 || n.Messages[0] != "progress at step 2\n"+redactedLine {
		t.Fatalf("note %+v", n)
	}
	if b, _ := os.ReadFile(notePath); strings.Contains(string(b), secret) {
		t.Fatal("the note holds the secret")
	}

	// An externally stopped session: its finished event records the
	// delivery and the on-demand salvage.
	q.obs.Tickets = nil
	syscall.Kill(-w.PID, syscall.SIGKILL)
	tickUntil(t, d, func() bool { return d.Running() == 0 })
	finished := eventsOf(t, d, "finished")
	if len(finished) != 1 {
		t.Fatalf("finished %+v", finished)
	}
	f := finished[0].Detail
	if f["contextDelivery"] != DeliveryFile || f["contextDelivered"] != "WARN,HANDOFF" || f["salvage"] != SalvageOnDemand {
		t.Fatalf("finished detail %+v", f)
	}
}

// ATR-V0-125: only "file" is a declared notice path, and an unwritable
// notice file alerts once, marks nothing, and delivers on a later tick.
func TestATRV0125_NoticeFileValidationAndRetry(t *testing.T) {
	c := testConfig(t, "exit 0")
	for v, ok := range map[string]bool{"": true, "file": true, "poll": false, "FILE": false} {
		h := c.Hosts["sh"]
		h.ContextNotices = v
		c.Hosts["sh"] = h
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); (err == nil) != ok {
			t.Errorf("contextNotices %q: err %v, want accepted %v", v, err, ok)
		}
	}
	dir := t.TempDir()
	rec := &ContextRecord{Worker: "w1", Tokens: "1500", WarnTokens: "1000", HandoffTokens: ContextNone, HardTokens: ContextNone,
		Crossings: []ContextCrossing{{Level: ContextWarn, Threshold: "1000", Tokens: "1500", At: "2026-10-18T09:00:00Z"}}}
	wdir := filepath.Join(dir, "workers", "w1")
	if err := os.MkdirAll(filepath.Join(wdir, contextNoticeFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := deliverNoticeFile(dir, rec); err == nil {
		t.Fatal("a directory notice file was accepted")
	}
	if _, err := os.Lstat(filepath.Join(wdir, deliveredPrefix+ContextWarn)); err == nil {
		t.Fatal("a failed delivery was marked")
	}
	if err := os.Remove(filepath.Join(wdir, contextNoticeFile)); err != nil {
		t.Fatal(err)
	}
	if levels, err := deliverNoticeFile(dir, rec); err != nil || strings.Join(levels, ",") != ContextWarn {
		t.Fatalf("retry %v %v", levels, err)
	}
	if levels, err := deliverNoticeFile(dir, rec); err != nil || len(levels) != 0 {
		t.Fatalf("repeat %v %v", levels, err)
	}
}

// ATR-V0-127: a worker stopped for context records the hard-stop salvage;
// one with neither a hard stop nor a note records none.
func TestATRV0127_FinishedRecordsSalvage(t *testing.T) {
	c := testConfig(t, "exit 0")
	d, _ := openCtx(t, &fakeQueue{}, c)
	for reason, want := range map[string]string{"CONTEXT": SalvageHardStop, "": SalvageNone} {
		detail := map[string]string{}
		d.finishContextDetail(&Worker{ID: "w-" + want, Role: "impl", Host: "sh", KillReason: reason}, detail, time.Now())
		if detail["salvage"] != want || detail["contextDelivery"] != DeliveryUnknown || detail["contextDelivered"] != "NONE" {
			t.Fatalf("%q: %+v", reason, detail)
		}
	}
}
