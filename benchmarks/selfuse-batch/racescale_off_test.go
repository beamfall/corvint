//go:build !race

package main

// raceScale is 1 without -race, so every scaled budget keeps the exact value it
// had before. The uninstrumented child measures 0.10s against these budgets.
const raceScale = 1
