package perfbudget

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// Sample is one measured wall time and the peak resident set of the process that served it.
type Sample struct {
	WallMS  float64 `json:"wall_ms"`
	PeakRSS *int64  `json:"peak_rss_bytes"`
}

// ScenarioMeasured is one scenario's samples and their summary. A metric the host cannot
// observe is null with RSSUnknown saying why; it is never estimated.
type ScenarioMeasured struct {
	ID           string   `json:"id"`
	Command      []string `json:"command"`
	Samples      []Sample `json:"samples"`
	P50WallMS    *float64 `json:"p50_wall_ms"`
	P95WallMS    *float64 `json:"p95_wall_ms"`
	PeakRSSBytes *int64   `json:"peak_rss_bytes"`
	RSSUnknown   string   `json:"rss_unknown,omitempty"`
	Error        string   `json:"error,omitempty"`
}

func (m ScenarioMeasured) metric(name string) *float64 {
	switch name {
	case MetricP50Wall:
		return m.P50WallMS
	case MetricP95Wall:
		return m.P95WallMS
	case MetricPeakRSS:
		if m.PeakRSSBytes == nil {
			return nil
		}
		value := float64(*m.PeakRSSBytes)
		return &value
	}
	return nil
}

// Percentile is the nearest-rank percentile (p in (0,1]) of values; nil for no values.
func Percentile(values []float64, p float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	rank := int(math.Ceil(p*float64(len(sorted)))) - 1
	value := sorted[max(0, min(rank, len(sorted)-1))]
	return &value
}

// summarize fills the percentiles and the peak resident set from the samples.
func (m *ScenarioMeasured) summarize() {
	walls := []float64{}
	for _, s := range m.Samples {
		walls = append(walls, s.WallMS)
		if s.PeakRSS == nil {
			m.RSSUnknown = rssUnknownReason
		} else if m.PeakRSSBytes == nil || *s.PeakRSS > *m.PeakRSSBytes {
			peak := *s.PeakRSS
			m.PeakRSSBytes = &peak
		}
	}
	if m.RSSUnknown != "" {
		m.PeakRSSBytes = nil
	}
	m.P50WallMS, m.P95WallMS = Percentile(walls, 0.50), Percentile(walls, 0.95)
}

// runOnce runs one CLI read to completion and measures it; a non-zero exit is an error, because
// a refused read measures nothing the budget means. An executable that cannot start is a
// *StartError.
func runOnce(ctx context.Context, dir string, argv []string) (Sample, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = measuredEnv()
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = io.Discard, &stderr
	start := time.Now()
	err := runContained(cmd)
	wall := time.Since(start)
	if failed := startFailure(cmd, err); failed != nil {
		return Sample{}, failed
	}
	if err != nil {
		return Sample{}, fmt.Errorf("%s: %v: %.400s", filepath.Base(argv[0]), err, stderr.String())
	}
	return Sample{WallMS: ms(wall), PeakRSS: peakRSS(cmd.ProcessState)}, nil
}

// measuredEnv strips the profile knobs and every CORVINT_ setting from measured processes, so an
// operator's shell cannot silently change what is measured.
func measuredEnv() []string {
	env := []string{}
	for _, kv := range os.Environ() {
		if len(kv) >= 8 && kv[:8] == "CORVINT_" {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func ms(d time.Duration) float64 { return math.Round(float64(d.Microseconds())/10) / 100 }

// mcpSession is one corpus MCP server process driven over stdio.
type mcpSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr bytes.Buffer
	next   int
}

func startMCP(ctx context.Context, dir string, argv []string) (*mcpSession, time.Time, error) {
	s := &mcpSession{cmd: exec.CommandContext(ctx, argv[0], argv[1:]...)}
	s.cmd.Dir = dir
	s.cmd.Env = measuredEnv()
	s.cmd.Stderr = &s.stderr
	stdin, err := s.cmd.StdinPipe()
	if err != nil {
		return nil, time.Time{}, err
	}
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return nil, time.Time{}, err
	}
	s.stdin, s.stdout = stdin, bufio.NewReaderSize(stdout, 1<<20)
	groupreap.Contain(s.cmd)
	start := time.Now()
	if err := groupreap.StartLive(s.cmd); err != nil {
		if failed := startFailure(s.cmd, err); failed != nil {
			return nil, time.Time{}, failed
		}
		return nil, time.Time{}, err
	}
	if _, err := s.request("initialize", map[string]any{"protocolVersion": mcpProtocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "corvint-perf-budget", "version": "0"}}); err != nil {
		return nil, time.Time{}, s.fail(err)
	}
	if _, err := io.WriteString(s.stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"); err != nil {
		return nil, time.Time{}, s.fail(err)
	}
	return s, start, nil
}

const mcpProtocolVersion = "2025-11-25"

// maxResponseBytes bounds one read MCP response line.
const maxResponseBytes = 64 << 20

func (s *mcpSession) request(method string, params map[string]any) (map[string]any, error) {
	s.next++
	data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": s.next, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	if _, err := s.stdin.Write(append(data, '\n')); err != nil {
		return nil, err
	}
	line, err := readBoundedLine(s.stdout, maxResponseBytes)
	if err != nil {
		return nil, err
	}
	var response map[string]any
	if err := json.Unmarshal(line, &response); err != nil {
		return nil, err
	}
	if response["error"] != nil {
		return nil, fmt.Errorf("%s: %v", method, response["error"])
	}
	return response, nil
}

func readBoundedLine(r *bufio.Reader, limit int) ([]byte, error) {
	line := []byte{}
	for {
		chunk, err := r.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > limit {
			return nil, fmt.Errorf("MCP response exceeds %d bytes", limit)
		}
		if err == nil {
			return line, nil
		}
		if err != bufio.ErrBufferFull {
			return nil, err
		}
	}
}

// call issues one tools/call and returns its wall time; a tool error result is an error.
func (s *mcpSession) call(tool string, arguments map[string]any) (time.Duration, error) {
	start := time.Now()
	response, err := s.request("tools/call", map[string]any{"name": tool, "arguments": arguments})
	wall := time.Since(start)
	if err != nil {
		return 0, err
	}
	result, _ := response["result"].(map[string]any)
	if isError, _ := result["isError"].(bool); result == nil || isError {
		return 0, fmt.Errorf("%s returned an error result", tool)
	}
	return wall, nil
}

// close ends the session at end of input and returns the server's peak resident set. The
// server's process group is swept before it is reaped (groupreap.Wait).
func (s *mcpSession) close() (*int64, error) {
	_ = s.stdin.Close()
	if err := groupreap.Wait(s.cmd); err != nil {
		return nil, fmt.Errorf("corpus MCP server exit: %v: %.400s", err, s.stderr.String())
	}
	return peakRSS(s.cmd.ProcessState), nil
}

func (s *mcpSession) fail(err error) error {
	_ = s.stdin.Close()
	_ = s.cmd.Process.Kill() // the leader's exit triggers groupreap.Wait's group sweep
	_ = groupreap.Wait(s.cmd)
	return fmt.Errorf("%v: %.400s", err, s.stderr.String())
}
