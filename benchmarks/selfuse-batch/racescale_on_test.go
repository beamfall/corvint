//go:build race

package main

// raceScale stretches the fixture wall-clock budgets that bound a child started
// from os.Executable(). Under -race that child is the race-instrumented test
// binary, and the race runtime's startup is a fixed cost the budgets were not
// written for: measured on a quiet host, a single child run costs 1.07s against
// 0.10s uninstrumented, so the 1s budget at main_test.go:140 missed by 70ms and
// seven subtests failed with process-timeout. The budgets are hang detectors,
// not performance budgets (decision 0082), so the factor is deliberately well
// clear of the measured 11x rather than tuned to it. Its upper limit is the
// fixture's own 30s hang in fakeProcessFault: 500ms * raceScale must still
// expire first.
const raceScale = 20
