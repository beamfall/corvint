package dispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Context delivery modes of a worker (ATR-V0-103).
const (
	DeliveryHook        = "HOOK"
	DeliveryUnknown     = "UNKNOWN"
	DeliveryUnsupported = "UNSUPPORTED"
	// DeliveryFile is the host-neutral polled-file path (ATR-V0-125).
	DeliveryFile = "FILE"
)

const (
	// ContextNoticesFile is the polled Host.ContextNotices value (ATR-V0-125).
	ContextNoticesFile = "file"
	// ContextNoticesPlugin declares an opencode host whose Corvint plugin
	// runs `dispatch context-hook` before each model request (DSG-V0-008).
	ContextNoticesPlugin = "plugin"
	contextNoticeFile    = "context-notices.txt"
	contextNoticeEnv     = "CORVINT_DISPATCH_CONTEXT_NOTICE_FILE"
)

const (
	// contextHookPlaceholder renders to the worker's hook settings file
	// (ATR-V0-102).
	contextHookPlaceholder = "{contextHookSettings}"
	contextHookSettings    = "context-hook-settings.json"
	// deliveredPrefix names a level's ATR-V0-101 delivery marker.
	deliveredPrefix = "context-delivered-"
)

// writeMarker writes a delivery marker's content; tests replace it.
var writeMarker = func(f *os.File, s string) error {
	_, err := f.WriteString(s)
	return err
}

// deliveredLevels are the levels a hook delivers, in ascending order.
var deliveredLevels = []string{ContextWarn, ContextHandoff}

// hostUsesContextHook reports a host whose argv or env uses
// {contextHookSettings}.
func hostUsesContextHook(h Host) bool {
	for _, a := range h.Argv {
		if strings.Contains(a, contextHookPlaceholder) {
			return true
		}
	}
	for _, v := range h.Env {
		if strings.Contains(v, contextHookPlaceholder) {
			return true
		}
	}
	return false
}

// hostUsesNoticeFile reports a host that declares the ATR-V0-125 polled
// notice file.
func hostUsesNoticeFile(h Host) bool { return h.ContextNotices == ContextNoticesFile }

// ContextNoticePath is a worker's ATR-V0-125 notice file, handed to the
// session as CORVINT_DISPATCH_CONTEXT_NOTICE_FILE.
func ContextNoticePath(dir, worker string) string {
	return filepath.Join(dir, "workers", worker, contextNoticeFile)
}

// ContextHookSettingsPath is a worker's ATR-V0-102 hook settings file.
func ContextHookSettingsPath(dir, worker string) string {
	return filepath.Join(dir, "workers", worker, contextHookSettings)
}

// shellQuote quotes s for a POSIX shell.
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// writeHookSettings writes the Claude Code settings file that registers the
// context hook for one worker (ATR-V0-102).
func (d *Dispatcher) writeHookSettings(worker string) (string, error) {
	cmd := d.HookCommand
	if len(cmd) == 0 {
		cmd = []string{"corvint-tasks", "dispatch", "context-hook"}
	}
	quoted := make([]string, len(cmd))
	for i, s := range cmd {
		quoted[i] = shellQuote(s)
	}
	hook := map[string]any{"type": "command", "command": strings.Join(quoted, " ")}
	settings := map[string]any{"hooks": map[string]any{"PostToolUse": []any{map[string]any{"matcher": "*", "hooks": []any{hook}}}}}
	raw, err := json.Marshal(settings)
	if err != nil {
		return "", err
	}
	path := ContextHookSettingsPath(d.dir, worker)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return path, writeAtomic(path, append(raw, '\n'))
}

// ContextDelivery is the ATR-V0-103 delivery view of a worker of role on
// host: its mode and the levels whose markers exist. ok is false for a role
// whose usage format reports no context figure.
func ContextDelivery(dir, worker, usageFormat string, host *Host) (mode string, delivered []string, ok bool) {
	switch {
	case usageFormat == "claude-code" && host != nil && hostUsesContextHook(*host):
		mode = DeliveryHook
	case usageFormat == "opencode" && host != nil && host.ContextNotices == ContextNoticesPlugin:
		mode = DeliveryHook // DSG-V0-008
	case contextFormat(usageFormat) && host != nil && hostUsesNoticeFile(*host):
		mode = DeliveryFile
	case usageFormat == "opencode":
		mode = DeliveryUnsupported
	case usageFormat == "claude-code":
		mode = DeliveryUnknown
	default:
		return "", nil, false
	}
	delivered = []string{}
	for _, l := range deliveredLevels {
		if _, err := os.Lstat(filepath.Join(dir, "workers", worker, deliveredPrefix+l)); err == nil {
			delivered = append(delivered, l)
		}
	}
	return mode, delivered, true
}

// deliveredText is the ATR-V0-127 summary of the levels delivered.
func deliveredText(levels []string) string {
	if len(levels) == 0 {
		return "NONE"
	}
	return strings.Join(levels, ",")
}

// deliverNoticeFile appends to the worker's ATR-V0-125 notice file the
// notice of each WARN or HANDOFF crossing of r that has no delivery marker,
// then marks each one. A failed append marks nothing, so the next tick
// retries; a level the hook marked first is not repeated. It returns the
// levels it delivered.
func deliverNoticeFile(dir string, r *ContextRecord) ([]string, error) {
	wdir := filepath.Join(dir, "workers", r.Worker)
	var due []ContextCrossing
	var text strings.Builder
	for _, l := range deliveredLevels {
		c := r.crossed(l)
		if c == nil {
			continue
		}
		if _, err := os.Lstat(filepath.Join(wdir, deliveredPrefix+l)); err == nil {
			continue
		}
		due = append(due, *c)
		text.WriteString(contextNoticeText(r, *c) + "\n")
	}
	if len(due) == 0 {
		return nil, nil
	}
	// The worker directory is the session's to write: the file is opened
	// beneath it and must be absent or regular, so it never names a FIFO
	// or a path outside it.
	if fi, err := os.Lstat(filepath.Join(wdir, contextNoticeFile)); err == nil && !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", contextNoticeFile)
	}
	root, err := os.OpenRoot(wdir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.OpenFile(contextNoticeFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	_, werr := f.WriteString(text.String())
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return nil, werr
	}
	var levels []string
	for _, c := range due {
		m, err := root.OpenFile(deliveredPrefix+c.Level, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			continue // the hook marked it first, or the next tick repeats it
		}
		_, werr := m.WriteString(c.At + "\n")
		if cerr := m.Close(); werr != nil || cerr != nil {
			_ = root.Remove(deliveredPrefix + c.Level)
			continue
		}
		levels = append(levels, c.Level)
	}
	return levels, nil
}

// contextNoticeText is the notice of one delivered crossing.
func contextNoticeText(r *ContextRecord, c ContextCrossing) string {
	next := "finish the current step, keep the remaining work small and record your progress."
	if c.Level == ContextHandoff {
		next = "stop starting new work: commit or record your progress and write hand-off notes into $CORVINT_DISPATCH_EVIDENCE_DIR now."
		if r.HardTokens != ContextNone {
			next += " The dispatcher stops this session at " + r.HardTokens + " tokens."
		}
	}
	return fmt.Sprintf("Corvint dispatch context notice: worker %s's context estimate of %s tokens reached its %s threshold of %s tokens; %s", r.Worker, c.Tokens, c.Level, c.Threshold, next)
}

// ContextHookNotice applies the ATR-V0-101 hook to the record at path: each
// WARN or HANDOFF crossing without a marker is marked exclusively and its
// notice returned. An empty notice delivers nothing.
func ContextHookNotice(path string) (string, error) {
	if path == "" {
		return "", errors.New("CORVINT_DISPATCH_CONTEXT_FILE is not set")
	}
	if filepath.Base(path) != contextFile {
		return "", fmt.Errorf("%s is not a context record path", path)
	}
	wdir := filepath.Dir(path)
	worker := filepath.Base(wdir)
	r, err := ReadContextRecord(filepath.Dir(filepath.Dir(wdir)), worker)
	if err != nil {
		return "", err
	}
	var notes []string
	for _, l := range deliveredLevels {
		c := r.crossed(l)
		if c == nil {
			continue
		}
		marker := filepath.Join(wdir, deliveredPrefix+l)
		f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return strings.Join(notes, "\n"), err
		}
		werr := writeMarker(f, c.At+"\n")
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			// The level stays deliverable: the marker is removed so the next
			// hook run retries, or, when it cannot be, the notice goes now.
			if rerr := os.Remove(marker); rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
				notes = append(notes, contextNoticeText(r, *c))
			}
			return strings.Join(notes, "\n"), werr
		}
		notes = append(notes, contextNoticeText(r, *c))
	}
	return strings.Join(notes, "\n"), nil
}

// Salvage values of a finished worker (ATR-V0-127).
const (
	SalvageHardStop = "HARD_STOP"
	SalvageOnDemand = "ON_DEMAND"
	SalvageNone     = "NONE"
)

// finishContextDetail records on worker w's finished event how its context
// warnings were delivered, which levels were, and whether its hand-off note
// was salvaged at the hard stop or on demand (ATR-V0-127), and how far the
// session ran past its HANDOFF crossing (DSG-V0-009).
func (d *Dispatcher) finishContextDetail(w *Worker, detail map[string]string, now time.Time) {
	if r := d.Config.roleNamed(w.Role); r != nil {
		var host *Host
		if h, ok := d.Config.Hosts[w.Host]; ok {
			host = &h
		}
		if mode, delivered, ok := ContextDelivery(d.dir, w.ID, r.UsageFormat, host); ok {
			detail["contextDelivery"], detail["contextDelivered"] = mode, deliveredText(delivered)
		}
	}
	afterHandoff(d.dir, w.ID, detail, now)
	detail["salvage"] = SalvageNone
	if w.KillReason == "CONTEXT" {
		detail["salvage"] = SalvageHardStop
	} else if n, err := ReadHandoffNote(HandoffNotePath(d.dir, w.ID)); err == nil && n.Worker == w.ID {
		detail["salvage"] = SalvageOnDemand
	}
}

// afterHandoff records, for a worker whose context record holds a HANDOFF
// crossing, the tokens its last estimate added past that crossing and the
// seconds from the crossing to its finish (DSG-V0-009). A worker that never
// crossed HANDOFF, or whose record is unreadable, records neither.
func afterHandoff(dir, worker string, detail map[string]string, now time.Time) {
	rec, err := ReadContextRecord(dir, worker)
	if err != nil {
		return
	}
	c := rec.crossed(ContextHandoff)
	if c == nil {
		return
	}
	at, err := time.Parse(time.RFC3339, c.At)
	crossedAt, ok1 := decimal(c.Tokens)
	last, ok2 := decimal(rec.Tokens)
	if err != nil || !ok1 || !ok2 {
		return
	}
	tokens := uint64(0)
	if last > crossedAt {
		tokens = last - crossedAt
	}
	secs := int64(0)
	if now.After(at) {
		secs = int64(now.Sub(at) / time.Second)
	}
	detail["tokensAfterHandoff"] = strconv.FormatUint(tokens, 10)
	detail["secondsAfterHandoff"] = strconv.FormatInt(secs, 10)
}
