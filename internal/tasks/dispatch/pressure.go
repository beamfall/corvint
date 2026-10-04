package dispatch

import (
	"fmt"
	"math"
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
}

// StepPressure is pure: thresholds use one-minute load per positive host CPU,
// either metric can raise pressure, and both must be calm before release.
func StepPressure(c PressureConfig, state PressureState, sample PressureSample) (PressureState, error) {
	if err := c.Validate(); err != nil {
		return state, err
	}
	if state.Level < 0 || state.Level > 2 || state.PendingLevel < 0 || state.PendingLevel > 2 || state.PendingTicks < 0 || state.PendingTicks >= c.TicksToChange {
		return state, fmt.Errorf("invalid pressure state")
	}
	load, loadOK := sample.LoadPerCPU()
	swap, swapOK := sample.SwapFraction()
	if !loadOK || !swapOK {
		state.Unknown = true
		state.PendingLevel = state.Level
		state.PendingTicks = 0
		return state, nil
	}
	state.Unknown = false
	target := state.Level
	if load <= c.CalmLoadPerCPU && swap <= c.CalmSwap {
		target = 0
	} else if load >= c.LoadPerCPUCritical || swap >= c.SwapCritical {
		target = 2
	} else if (load >= c.LoadPerCPUHigh || swap >= c.SwapHigh) && target < 1 {
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
