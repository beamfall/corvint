//go:build windows

package repository

import (
	"os/exec"
	"testing"
)

func TestWindowsContainmentFailsClosedBeforeStart(t *testing.T) {
	command := exec.Command("unused")
	containment, ok := prepareContainment(command)
	if ok || containment != nil {
		t.Fatalf("prepareContainment = (%v, %t), want fail closed", containment, ok)
	}
	if command.Process != nil {
		t.Fatal("fail-closed containment started a process")
	}
}
