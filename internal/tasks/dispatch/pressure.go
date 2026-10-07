package dispatch

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"unicode"
)

// PressureConfig is optional launch admission policy. It does not authorize
// stopping workers or bypassing the ordinary roster and native lease checks.
type PressureConfig struct {
	LoadPerCPUHigh     float64        `json:"loadPerCpuHigh"`
	LoadPerCPUCritical float64        `json:"loadPerCpuCritical"`
	SwapHigh           float64        `json:"swapHigh"`
	SwapCritical       float64        `json:"swapCritical"`
	CalmLoadPerCPU     float64        `json:"calmLoadPerCpu"`
	CalmSwap           float64        `json:"calmSwap"`
	TicksToChange      int            `json:"ticksToChange"`
	LevelCaps          map[string]int `json:"levelCaps"`
	ExemptRoles        []string       `json:"exemptRoles,omitempty"`
	ExemptTickets      []string       `json:"exemptTickets,omitempty"`
	// CPU utilization thresholds are busy/total tick fractions over the
	// interval since the previous sample (CAL-V0-125). They are required
	// only when a Signals entry selects the cpu signal.
	CPUHigh     float64 `json:"cpuHigh,omitempty"`
	CPUCritical float64 `json:"cpuCritical,omitempty"`
	CalmCPU     float64 `json:"calmCpu,omitempty"`
	// Signals selects, per host OS, which signals set the level
	// (CAL-V0-126). An OS without an entry keeps the default: load plus
	// the kernel memory-pressure level (Darwin) or swap (Linux).
	Signals map[string][]string `json:"signals,omitempty"`
}

// pressureSignalsAvailable names the signals each supported host sampler
// can observe. Darwin CPU ticks need host_processor_info, which is not
// reachable from a CGO_ENABLED=0 binary without forbidden linkname
// trampolines, so Darwin cannot select cpu (CAL-V0-125).
var pressureSignalsAvailable = map[string][]string{
	"darwin": {PressureSignalLoad, PressureSignalMemory},
	"linux":  {PressureSignalCPU, PressureSignalLoad, PressureSignalSwap},
}

func (c PressureConfig) selectsCPU() bool {
	for _, names := range c.Signals {
		if contains(names, PressureSignalCPU) {
			return true
		}
	}
	return false
}

func finiteNonnegative(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) && x >= 0 }

// Validate checks local bounds. Config validation additionally requires each
// exempt role to name a configured role; exempt tickets are matched against a
// candidate's native ID or local name and are not resolved in advance.
func (c PressureConfig) Validate() error {
	if !finiteNonnegative(c.CalmLoadPerCPU) || !finiteNonnegative(c.LoadPerCPUHigh) || !finiteNonnegative(c.LoadPerCPUCritical) || !(c.CalmLoadPerCPU < c.LoadPerCPUHigh && c.LoadPerCPUHigh < c.LoadPerCPUCritical) {
		return fmt.Errorf("pressure load thresholds require finite 0 <= calm < high < critical")
	}
	if !finiteNonnegative(c.CalmSwap) || !finiteNonnegative(c.SwapHigh) || !finiteNonnegative(c.SwapCritical) || !(c.CalmSwap < c.SwapHigh && c.SwapHigh < c.SwapCritical && c.SwapCritical <= 1) {
		return fmt.Errorf("pressure swap thresholds require 0 <= calm < high < critical <= 1")
	}
	if (c.selectsCPU() || c.CPUHigh != 0 || c.CPUCritical != 0 || c.CalmCPU != 0) && (!finiteNonnegative(c.CalmCPU) || !finiteNonnegative(c.CPUHigh) || !finiteNonnegative(c.CPUCritical) || !(c.CalmCPU < c.CPUHigh && c.CPUHigh < c.CPUCritical && c.CPUCritical <= 1)) {
		return fmt.Errorf("pressure cpu thresholds require 0 <= calmCpu < cpuHigh < cpuCritical <= 1 when cpu is selected or configured")
	}
	oses := make([]string, 0, len(c.Signals))
	for goos := range c.Signals {
		oses = append(oses, goos)
	}
	sort.Strings(oses)
	for _, goos := range oses {
		names := c.Signals[goos]
		available, ok := pressureSignalsAvailable[goos]
		if !ok {
			return fmt.Errorf("pressure signals name unsupported host OS %q (darwin or linux)", goos)
		}
		if len(names) == 0 || len(names) > len(available) {
			return fmt.Errorf("pressure signals for %s must select 1..%d signals", goos, len(available))
		}
		seen := map[string]bool{}
		for _, name := range names {
			if !contains(available, name) || seen[name] {
				return fmt.Errorf("pressure signals for %s must be unique names from %s", goos, strings.Join(available, ", "))
			}
			seen[name] = true
		}
	}
	if c.TicksToChange < 1 || c.TicksToChange > 3600 {
		return fmt.Errorf("pressure ticksToChange must be 1..3600")
	}
	one, ok1 := c.LevelCaps["1"]
	two, ok2 := c.LevelCaps["2"]
	if len(c.LevelCaps) != 2 || !ok1 || !ok2 || one < 0 || one > 64 || two < 0 || two > one {
		return fmt.Errorf("pressure levelCaps requires levels 1 and 2 with 0 <= cap2 <= cap1 <= 64")
	}
	if len(c.ExemptRoles) > 32 || len(c.ExemptTickets) > 512 {
		return fmt.Errorf("pressure exemptions exceed bounds")
	}
	seen := map[string]bool{}
	for _, role := range c.ExemptRoles {
		if !ValidName(role) || seen[role] {
			return fmt.Errorf("invalid or duplicate pressure exempt role")
		}
		seen[role] = true
	}
	seen = map[string]bool{}
	for _, ticket := range c.ExemptTickets {
		if ticket == "" || len(ticket) > 256 || strings.IndexFunc(ticket, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 || strings.ContainsAny(ticket, "*?[]") || seen[ticket] {
			return fmt.Errorf("invalid or duplicate pressure exempt ticket")
		}
		seen[ticket] = true
	}
	return nil
}

// PressureState retains hysteresis across ticks. UNKNOWN cancels pending dwell,
// while retaining the current level; it never manufactures calm evidence.
type PressureState struct {
	Level        int  `json:"level"`
	PendingLevel int  `json:"pendingLevel"`
	PendingTicks int  `json:"pendingTicks"`
	Unknown      bool `json:"unknown"`
	// Reason names the signals that set the current level (V1-0862): at a
	// raise, those at or above the new level's threshold. Empty at level 0
	// and for a level recorded before reasons existed.
	Reason []string `json:"reason,omitempty"`
}

// Pressure signal names reported as a level's reason, in sorted order.
const (
	PressureSignalCPU    = "cpu"
	PressureSignalLoad   = "load"
	PressureSignalMemory = "memory"
	PressureSignalSwap   = "swap"
)

// ValidPressureReason accepts a level's recorded reason: none at level 0,
// otherwise sorted unique signal names (an older record may carry none).
func ValidPressureReason(level int, reason []string) bool {
	if level == 0 {
		return len(reason) == 0
	}
	for i, r := range reason {
		if r != PressureSignalCPU && r != PressureSignalLoad && r != PressureSignalMemory && r != PressureSignalSwap {
			return false
		}
		if i > 0 && reason[i-1] >= r {
			return false
		}
	}
	return true
}

// PressureReasonText reports a level's reason: NONE at level 0, the
// comma-joined signals, or UNKNOWN for a level recorded without one.
func PressureReasonText(st PressureState) string {
	switch {
	case st.Level == 0:
		return "NONE"
	case len(st.Reason) == 0:
		return StateUnknown
	}
	return strings.Join(st.Reason, ",")
}

// pressureClass orders one signal against its thresholds.
type pressureClass int

const (
	pressureCalm     pressureClass = iota // at or below calm
	pressureElevated                      // above calm, below high
	pressureHigh
	pressureCritical
)

type pressureSignal struct {
	name  string
	class pressureClass
}

func classifyPressure(x, calm, high, critical float64) pressureClass {
	switch {
	case x <= calm:
		return pressureCalm
	case x >= critical:
		return pressureCritical
	case x >= high:
		return pressureHigh
	}
	return pressureElevated
}

// pressureSignals returns the participating signals, or false when any is
// UNKNOWN. Without a Signals entry for goos they are load and one memory
// signal: the kernel memory-pressure level when the sample carries one
// (Darwin: normal is calm, warn is high, critical is critical), otherwise the
// used/total swap fraction (Linux). A sample whose memory signal is absent
// never falls back to another one. A Signals entry replaces that default with
// exactly the named signals (CAL-V0-126).
func pressureSignals(c PressureConfig, goos string, sample PressureSample) ([]pressureSignal, bool) {
	names, selected := c.Signals[goos]
	if !selected {
		names = []string{PressureSignalLoad, PressureSignalSwap}
		if sample.MemoryPressureKnown || sample.MemoryPressureLevel != 0 {
			names[1] = PressureSignalMemory
		}
	}
	signals := make([]pressureSignal, 0, len(names))
	for _, name := range names {
		var class pressureClass
		switch name {
		case PressureSignalLoad:
			load, ok := sample.LoadPerCPU()
			if !ok {
				return nil, false
			}
			class = classifyPressure(load, c.CalmLoadPerCPU, c.LoadPerCPUHigh, c.LoadPerCPUCritical)
		case PressureSignalMemory:
			level, ok := sample.MemoryPressure()
			if !ok {
				return nil, false
			}
			class = map[int]pressureClass{MemoryPressureNormal: pressureCalm, MemoryPressureWarn: pressureHigh, MemoryPressureCritical: pressureCritical}[level]
		case PressureSignalSwap:
			swap, ok := sample.SwapFraction()
			if !ok {
				return nil, false
			}
			class = classifyPressure(swap, c.CalmSwap, c.SwapHigh, c.SwapCritical)
		case PressureSignalCPU:
			cpu, ok := sample.CPUUtilizationFraction()
			if !ok {
				return nil, false
			}
			class = classifyPressure(cpu, c.CalmCPU, c.CPUHigh, c.CPUCritical)
		default:
			return nil, false
		}
		signals = append(signals, pressureSignal{name, class})
	}
	return signals, true
}

// StepPressure is pure: thresholds use one-minute load per positive host CPU,
// any participating signal can raise pressure, and all must be calm before
// release. The participating signals are those selected for this host OS.
func StepPressure(c PressureConfig, state PressureState, sample PressureSample) (PressureState, error) {
	return stepPressure(c, runtime.GOOS, state, sample)
}

func stepPressure(c PressureConfig, goos string, state PressureState, sample PressureSample) (PressureState, error) {
	if err := c.Validate(); err != nil {
		return state, err
	}
	if state.Level < 0 || state.Level > 2 || state.PendingLevel < 0 || state.PendingLevel > 2 || state.PendingTicks < 0 || state.PendingTicks >= c.TicksToChange {
		return state, fmt.Errorf("invalid pressure state")
	}
	signals, ok := pressureSignals(c, goos, sample)
	if !ok {
		state.Unknown = true
		state.PendingLevel = state.Level
		state.PendingTicks = 0
		return state, nil
	}
	state.Unknown = false
	calm, high, critical := true, false, false
	for _, s := range signals {
		calm = calm && s.class == pressureCalm
		high = high || s.class >= pressureHigh
		critical = critical || s.class == pressureCritical
	}
	target := state.Level
	if calm {
		target = 0
	} else if critical {
		target = 2
	} else if high && target < 1 {
		target = 1
	}
	if target == state.Level {
		state.PendingLevel = state.Level
		state.PendingTicks = 0
		return state, nil
	}
	if state.PendingLevel != target {
		state.PendingLevel = target
		state.PendingTicks = 0
	}
	state.PendingTicks++
	if state.PendingTicks >= c.TicksToChange {
		state.Level = target
		state.PendingLevel = target
		state.PendingTicks = 0
		state.Reason = nil
		if target > 0 {
			threshold := map[int]pressureClass{1: pressureHigh, 2: pressureCritical}[target]
			for _, s := range signals {
				if s.class >= threshold {
					state.Reason = append(state.Reason, s.name)
				}
			}
			sort.Strings(state.Reason)
		}
	}
	return state, nil
}

// PressureBudget counts only non-exempt concurrency. Roster integration must
// call Accept before consuming static slots or reserving a candidate's key.
type PressureBudget struct {
	enabled        bool
	cap, used      int
	roles, tickets map[string]bool
}

// NewPressureBudget observes running assignments without changing them. A nil
// configuration disables pressure and preserves ordinary admission behavior.
func NewPressureBudget(c *PressureConfig, state PressureState, running []Assignment, pinned []string) (*PressureBudget, error) {
	b := &PressureBudget{}
	if c == nil {
		return b, nil
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if state.Level < 0 || state.Level > 2 {
		return nil, fmt.Errorf("invalid pressure level")
	}
	b.roles, b.tickets = map[string]bool{}, map[string]bool{}
	for _, r := range c.ExemptRoles {
		b.roles[r] = true
	}
	for _, t := range append(append([]string{}, c.ExemptTickets...), pinned...) {
		b.tickets[t] = true
	}
	b.enabled = state.Level != 0
	if b.enabled {
		b.cap = c.LevelCaps[fmt.Sprint(state.Level)]
	}
	for _, a := range running {
		if !b.Exempt(a) {
			b.used++
		}
	}
	return b, nil
}

// Exempt is based only on explicit role/ticket identities and admitted pins.
func (b *PressureBudget) Exempt(a Assignment) bool {
	return b.roles[a.Role] || (a.Ticket != "" && b.tickets[a.Ticket]) || (a.Local != "" && b.tickets[a.Local])
}

// Accept reserves pressure budget only; callers retain every static/native
// admission fence and must not reserve a rejected candidate's key or role slot.
func (b *PressureBudget) Accept(a Assignment) bool {
	if !b.enabled || b.Exempt(a) {
		return true
	}
	if b.used >= b.cap {
		return false
	}
	b.used++
	return true
}
