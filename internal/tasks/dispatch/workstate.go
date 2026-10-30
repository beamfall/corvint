package dispatch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maxStatusFile   = 64 << 10
	maxStateCommand = 1 << 20
	maxStateValue   = 64
	stateTimeout    = 60 * time.Second
)

// ReadStates is CAL-V0-053's program-defined work state. It fills every
// ticket's State with a value, NONE, or UNKNOWN, and returns one alert per
// failed read. A missing reader leaves every state NONE.
func ReadStates(ctx context.Context, c *Config, tickets []Ticket) []string {
	return readStates(ctx, c, tickets, func(ctx context.Context, _ string, argv []string) (map[string]commandState, error) {
		return standaloneStateCommand(ctx, c, argv)
	})
}

func readStates(ctx context.Context, c *Config, tickets []Ticket, command func(context.Context, string, []string) (map[string]commandState, error)) []string {
	for i := range tickets {
		tickets[i].State = StateNone
		tickets[i].ProgressToken, tickets[i].ProgressDigest = "", ""
	}
	w := c.WorkState
	if w == nil {
		return nil
	}
	var alerts []string
	if w.Kind == "status-line" {
		for i := range tickets {
			path := strings.ReplaceAll(w.Path, "{ticketLocal}", tickets[i].Local)
			v, err := statusLine(path, w.Key)
			if err != nil {
				v = StateUnknown
				alerts = append(alerts, fmt.Sprintf("work state for %s: %v", tickets[i].Local, err))
			}
			tickets[i].State = v
		}
		return alerts
	}
	states, err := command(ctx, c.WorkRoot, w.Argv)
	for i := range tickets {
		switch {
		case err != nil:
			tickets[i].State = StateUnknown
		default:
			v, ok := states[tickets[i].ID]
			if !ok {
				v, ok = states[tickets[i].Local]
			}
			if ok {
				tickets[i].State, tickets[i].ProgressToken = v.State, v.Progress
			}
		}
	}
	if err != nil {
		alerts = append(alerts, fmt.Sprintf("work state command: %v", err))
	}
	return alerts
}

func statusLine(path, key string) (string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return StateNone, nil
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxStatusFile+1))
	if err != nil {
		return "", err
	}
	if len(raw) > maxStatusFile {
		return "", fmt.Errorf("status file exceeds %d bytes", maxStatusFile)
	}
	s := bufio.NewScanner(bytes.NewReader(raw))
	s.Buffer(make([]byte, 0, 4096), maxStatusFile)
	for s.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(s.Text()), ":")
		if ok && strings.TrimSpace(k) == key {
			return stateValue(strings.TrimSpace(v))
		}
	}
	return StateNone, s.Err()
}

func stateValue(v string) (string, error) {
	if v == "" {
		return StateNone, nil
	}
	if len(v) > maxStateValue || strings.IndexFunc(v, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("state value is longer than %d bytes or has control characters", maxStateValue)
	}
	return v, nil
}

type commandState struct{ State, Progress string }

// JSON's replacement of invalid UTF-8 and lone surrogates must not invent
// a progress assertion. Validate scalar strings before the ordinary decoder.
func validScalarJSON(raw []byte) bool {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return false
	}
	quoted := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		n, _ := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, _ := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}

func decodeCommandStates(raw []byte) (map[string]commandState, error) {
	bad := func() (map[string]commandState, error) {
		return nil, errors.New("output is not one closed JSON object of states and optional progress tokens")
	}
	if len(raw) > maxStateCommand || !validScalarJSON(raw) {
		return bad()
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return bad()
	}
	states := map[string]commandState{}
	for d.More() {
		k, err := d.Token()
		if err != nil {
			return bad()
		}
		key, ok := k.(string)
		if !ok {
			return bad()
		}
		if _, duplicate := states[key]; duplicate {
			return bad()
		}
		v, err := d.Token()
		if err != nil {
			return bad()
		}
		var state commandState
		if s, ok := v.(string); ok {
			state.State = s
		} else if v == json.Delim('{') {
			seen := map[string]bool{}
			for d.More() {
				field, err := d.Token()
				if err != nil {
					return bad()
				}
				name, ok := field.(string)
				if !ok || seen[name] || (name != "state" && name != "progress") {
					return bad()
				}
				seen[name] = true
				value, err := d.Token()
				s, ok := value.(string)
				if err != nil || !ok {
					return bad()
				}
				if name == "state" {
					state.State = s
				} else {
					state.Progress = s
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') || !seen["state"] {
				return bad()
			}
		} else {
			return bad()
		}
		state.State, err = stateValue(state.State)
		if err != nil || len(state.Progress) > 128 || strings.IndexFunc(state.Progress, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
			return bad()
		}
		states[key] = state
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	return states, nil
}

type bounded struct {
	buf  bytes.Buffer
	over bool
}

func (b *bounded) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > maxStateCommand {
		b.over = true
		return len(p), nil
	}
	return b.buf.Write(p)
}
