package dispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Context delivery modes of a worker (ATR-V0-103).
const (
	DeliveryHook        = "HOOK"
	DeliveryUnknown     = "UNKNOWN"
	DeliveryUnsupported = "UNSUPPORTED"
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
	case usageFormat == "opencode":
		mode = DeliveryUnsupported
	case usageFormat == "claude-code" && host != nil && hostUsesContextHook(*host):
		mode = DeliveryHook
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
