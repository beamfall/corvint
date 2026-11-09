package perfbudget

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

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
	// Provenance is set only on a committed baseline measurement taken in another run than the
	// baseline's own; that measurement is attributed to it, not to the baseline (PRB-V0-053).
	Provenance *Provenance `json:"provenance,omitempty"`
}

// Provenance names the run one baseline measurement came from: when it started, the measured
// corvint executable (which also built the fixture artifacts it read) by source revision and
// SHA-256, and the host observation of that run (PRB-V0-053).
type Provenance struct {
	RecordedAt         string `json:"recorded_at"`
	ExecutableRevision string `json:"executable_revision"`
	ExecutableSHA256   string `json:"executable_sha256"`
	Host               Host   `json:"host"`
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
func runOnce(ctx context.Context, dir string, argv []string, in *input) (Sample, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = measuredEnv()
	if in != nil {
		cmd.Stdin = bytes.NewReader(in.stdin)
		cmd.Env = append(cmd.Env, in.env...)
	}
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = io.Discard, &stderr
	rl, relayErr := startRelayed(cmd)
	if relayErr != nil && relayErr != errNoRelay {
		return Sample{}, relayErr
	}
	start := time.Now()
	err := runContained(cmd)
	wall := time.Since(start)
	report := rl.result()
	if report != nil && report.StartError != "" {
		return Sample{}, &StartError{Command: rl.target, Err: errors.New(report.StartError)}
	}
	if failed := startFailure(cmd, err); failed != nil {
		return Sample{}, failed
	}
	if err != nil {
		return Sample{}, fmt.Errorf("%s: %v: %.400s", filepath.Base(argv[0]), err, stderr.String())
	}
	peak, relayWall := measuredPeak(cmd, rl, relayErr, report)
	if relayWall > 0 {
		wall = relayWall
	}
	return Sample{WallMS: ms(wall), PeakRSS: peak}, nil
}

// errNoRelay is why a Linux measurement has no peak resident set when the harness cannot locate
// its own executable to start the relay (PRB-V0-060).
var errNoRelay = errors.New("peak resident set NOT_OBSERVED: the harness cannot locate its own executable to relay the measured process")

// measuredPeak is the measured process's own peak resident set (PRB-V0-060) and, when the relay
// timed it, its wall time: the relay's report on Linux, nil when a relay was needed and wrote
// none, and the process's own rusage elsewhere.
func measuredPeak(cmd *exec.Cmd, rl *relay, relayErr error, report *relayReport) (*int64, time.Duration) {
	switch {
	case report != nil && report.StartError == "":
		v := report.MaxRSSKiB * 1024
		return &v, time.Duration(report.WallUS) * time.Microsecond
	case rl != nil || relayErr != nil:
		return nil, 0
	}
	return peakRSS(cmd.ProcessState), 0
}

// input is what a measured hook invocation reads beyond its argv: the host's event payload on
// stdin and the host's environment, appended after measuredEnv so it overrides the operator's.
type input struct {
	stdin []byte
	env   []string
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
