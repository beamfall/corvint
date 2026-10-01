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
	"os/exec"
	"strings"
	"time"
	"unicode"
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
	for i := range tickets {
		tickets[i].State = StateNone
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
	states, err := stateCommand(ctx, c.WorkRoot, w.Argv)
	for i := range tickets {
		switch {
		case err != nil:
			tickets[i].State = StateUnknown
		case states[tickets[i].ID] != "":
			tickets[i].State = states[tickets[i].ID]
		case states[tickets[i].Local] != "":
			tickets[i].State = states[tickets[i].Local]
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

func stateCommand(ctx context.Context, dir string, argv []string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, stateTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.WaitDelay = time.Second
	var out bounded
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if out.over {
		return nil, fmt.Errorf("output exceeds %d bytes", maxStateCommand)
	}
	var raw map[string]string
	if err := json.Unmarshal(out.buf.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("output is not one JSON object of strings: %w", err)
	}
	states := map[string]string{}
	for k, v := range raw {
		s, err := stateValue(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		states[k] = s
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
