package dispatch

import (
	"math"
	"reflect"
	"testing"
)

func issue497Config() PressureConfig {
	return PressureConfig{LoadPerCPUHigh: 5, LoadPerCPUCritical: 8, CalmLoadPerCPU: 3.5, SwapHigh: .8, SwapCritical: .9, CalmSwap: .7, TicksToChange: 2, LevelCaps: map[string]int{"1": 5, "2": 3}}
}
func issue497Sample(load, swap float64) PressureSample {
	return PressureSample{LoadAverage: load * 10, CPUs: 10, SwapTotalBytes: 1000, SwapUsedBytes: uint64(swap * 1000), LoadKnown: true, CPUKnown: true, SwapKnown: true}
}
func issue497Step(t *testing.T, c PressureConfig, s PressureState, sample PressureSample) PressureState {
	t.Helper()
	next, err := StepPressure(c, s, sample)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestIssue497_PressureHysteresis(t *testing.T) {
	c := issue497Config()
	s := PressureState{}
	s = issue497Step(t, c, s, issue497Sample(5, .7))
	if s.Level != 0 || s.PendingTicks != 1 {
		t.Fatal(s)
	}
	s = issue497Step(t, c, s, issue497Sample(4, .7))
	if s.Level != 0 || s.PendingTicks != 0 {
		t.Fatal("broken consecutive dwell", s)
	}
	for range 2 {
		s = issue497Step(t, c, s, issue497Sample(5, .7))
	}
	if s.Level != 1 {
		t.Fatal(s)
	}
	s = issue497Step(t, c, s, issue497Sample(8, .7))
	if s.Level != 1 || s.PendingLevel != 2 {
		t.Fatal(s)
	}
	s = issue497Step(t, c, s, issue497Sample(8, .7))
	if s.Level != 2 {
		t.Fatal(s)
	}
	for _, sample := range []PressureSample{issue497Sample(5, .7), issue497Sample(4, .75), issue497Sample(3.5, .75), issue497Sample(4, .7)} {
		s = issue497Step(t, c, s, sample)
		if s.Level != 2 || s.PendingTicks != 0 {
			t.Fatal("level2 must hold until both calm", s)
		}
	}
	s = issue497Step(t, c, s, issue497Sample(3.5, .7))
	if s.Level != 2 || s.PendingTicks != 1 {
		t.Fatal(s)
	}
	s = issue497Step(t, c, s, issue497Sample(3.5, .7))
	if s.Level != 0 {
		t.Fatal(s)
	}
	s = issue497Step(t, c, s, issue497Sample(5, .7))
	s = issue497Step(t, c, s, issue497Sample(8, .7))
	if s.PendingLevel != 2 || s.PendingTicks != 1 {
		t.Fatal("changed target must restart dwell", s)
	}
}

func TestIssue497_UnknownRetainsLevel(t *testing.T) {
	c := issue497Config()
	invalid := []PressureSample{issue497Sample(10, .95), issue497Sample(0, 0), issue497Sample(10, .95), issue497Sample(10, .95), issue497Sample(10, .95), issue497Sample(10, .95)}
	invalid[0].LoadKnown = false
	invalid[1].SwapKnown = false
	invalid[2].CPUKnown = false
	invalid[3].CPUs = 0
	invalid[4].LoadAverage = math.NaN()
	invalid[5].SwapUsedBytes = 1001
	for _, level := range []int{0, 1, 2} {
		for _, sample := range invalid {
			s := issue497Step(t, c, PressureState{Level: level, PendingLevel: 2, PendingTicks: 1}, sample)
			if s.Level != level || !s.Unknown || s.PendingTicks != 0 || s.PendingLevel != level {
				t.Fatal(s)
			}
		}
	}
	s := issue497Step(t, c, PressureState{Unknown: true}, issue497Sample(5, .7))
	if s.Unknown || s.PendingTicks != 1 {
		t.Fatal(s)
	}
}

func TestIssue497_NormalizesHostCPU(t *testing.T) {
	s := issue497Sample(4, 0)
	x, ok := s.LoadPerCPU()
	if !ok || x != 4 {
		t.Fatal(x, ok)
	}
	s.CPUs = 20
	x, ok = s.LoadPerCPU()
	if !ok || x != 2 {
		t.Fatal(x, ok)
	}
	for _, raw := range []string{"0", "-1", "1.5", "10\n10", "NaN", ""} {
		if _, err := parseCPUs([]byte(raw)); err == nil {
			t.Fatal("invalid CPU accepted", raw)
		}
	}
	for _, raw := range []string{"", "cpu0 1 2 3 4\ncpu0 1 2 3 4", "cpu01 1 2 3 4", "cpu-1 1 2 3 4", "cpu0 1 bad 3 4"} {
		if _, err := parseLinuxCPUs([]byte(raw)); err == nil {
			t.Fatal("invalid CPU records", raw)
		}
	}
	count, err := parseLinuxCPUs([]byte("cpu 1 2 3 4\ncpu0 1 2 3 4\ncpu2 1 2 3 4\nintr 4"))
	if err != nil || count != 2 {
		t.Fatal(count, err)
	}
}

func TestIssue497_MixedMetricTransitions(t *testing.T) {
	c := issue497Config()
	c.TicksToChange = 1
	for _, row := range []struct {
		load, swap float64
		want       int
	}{{5, .1, 1}, {1, .8, 1}, {8, .1, 2}, {1, .9, 2}, {3.5, .7, 0}, {4, .1, 0}, {1, .75, 0}} {
		s := issue497Step(t, c, PressureState{}, issue497Sample(row.load, row.swap))
		if s.Level != row.want {
			t.Fatal(row, s)
		}
	}
	for _, sample := range []PressureSample{issue497Sample(3.5, .75), issue497Sample(4, .7)} {
		s := issue497Step(t, c, PressureState{Level: 2, PendingLevel: 2}, sample)
		if s.Level != 2 {
			t.Fatal(s)
		}
	}
}

func TestIssue497_NonExemptBudget(t *testing.T) {
	c := issue497Config()
	c.ExemptRoles = []string{"review", "integration"}
	c.ExemptTickets = []string{"critical"}
	author := Assignment{Role: "author", Ticket: "a"}
	exempt := []Assignment{{Role: "review", Ticket: "r"}, {Role: "integration", Ticket: "i"}, {Role: "author", Ticket: "critical"}, {Role: "author", Ticket: "pinned"}}
	running := []Assignment{author, author, author, author}
	before := append([]Assignment{}, running...)
	b, err := NewPressureBudget(&c, PressureState{Level: 2}, running, []string{"pinned"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Accept(author) {
		t.Fatal("busy excess admitted")
	}
	for _, a := range exempt {
		if !b.Accept(a) {
			t.Fatal("explicit exemption held", a)
		}
	}
	if !reflect.DeepEqual(before, running) {
		t.Fatal("running set mutated")
	}
	b, _ = NewPressureBudget(&c, PressureState{Level: 2}, nil, nil)
	for range 3 {
		if !b.Accept(author) {
			t.Fatal("budget lost")
		}
	}
	if b.Accept(author) {
		t.Fatal("tick overlaunch")
	}
	c.LevelCaps["2"] = 0
	b, _ = NewPressureBudget(&c, PressureState{Level: 2}, nil, nil)
	if b.Accept(author) || !b.Accept(exempt[0]) {
		t.Fatal("zero cap/exemption")
	}
	if b.Exempt(Assignment{Role: "review-like", Ticket: "r"}) {
		t.Fatal("inferred role-name exemption")
	}
	for _, state := range []PressureState{{}, {Unknown: true}} {
		b, _ = NewPressureBudget(&c, state, nil, nil)
		for range 100 {
			if !b.Accept(author) {
				t.Fatal("inactive pressure held")
			}
		}
	}
	b, _ = NewPressureBudget(nil, PressureState{Level: 2}, nil, nil)
	if !b.Accept(author) {
		t.Fatal("absent config held")
	}
}

func TestIssue497_InvalidConfig(t *testing.T) {
	edits := []func(*PressureConfig){func(c *PressureConfig) { c.LoadPerCPUHigh = c.CalmLoadPerCPU }, func(c *PressureConfig) { c.LoadPerCPUCritical = math.Inf(1) }, func(c *PressureConfig) { c.CalmSwap = math.NaN() }, func(c *PressureConfig) { c.SwapCritical = 1.1 }, func(c *PressureConfig) { c.TicksToChange = 0 }, func(c *PressureConfig) { c.TicksToChange = 3601 }, func(c *PressureConfig) { delete(c.LevelCaps, "2") }, func(c *PressureConfig) { c.LevelCaps["3"] = 0 }, func(c *PressureConfig) { c.LevelCaps["2"] = 6 }, func(c *PressureConfig) { c.LevelCaps["1"] = -1 }, func(c *PressureConfig) { c.ExemptRoles = []string{"review", "review"} }, func(c *PressureConfig) { c.ExemptTickets = []string{"a*"} }, func(c *PressureConfig) { c.ExemptTickets = []string{"a", "a"} }}
	for i, edit := range edits {
		c := issue497Config()
		edit(&c)
		if c.Validate() == nil {
			t.Fatal("invalid config accepted", i)
		}
	}
	for _, s := range []PressureState{{Level: 3}, {PendingLevel: -1}, {PendingTicks: 2}} {
		if _, err := StepPressure(issue497Config(), s, issue497Sample(0, 0)); err == nil {
			t.Fatal("invalid state accepted", s)
		}
	}
}
