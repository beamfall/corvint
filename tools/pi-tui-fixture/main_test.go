//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// PWV-V0-016: every prompt must expose a fresh unsupported-host refusal.
func TestProtectedDriverRequiresFreshNoticeForEveryPrompt(t *testing.T) {
	master, err := os.Create(filepath.Join(t.TempDir(), "input"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { master.Close() })
	d := &protectedDriver{first: 3, expected: "unsupported-host-version", sent: true, readyAt: time.Now()}
	s := &session{master: master}
	for _, token := range []string{"PROTECTED_NATIVE_PI_OK_3", "PROTECTED_NATIVE_PI_OK_4", "PROTECTED_NATIVE_PI_OK_5"} {
		phase := d.phase
		s.output = []byte(token)
		d.responseAt = time.Now().Add(-3 * time.Second)
		if err := d.step(s); err != nil {
			t.Fatal(err)
		}
		if d.phase != phase || s.stopped {
			t.Fatal("provider token without fresh notice advanced prompt")
		}
		s.output = append(s.output, []byte(" unsupported-host-version")...)
		if err := d.step(s); err != nil {
			t.Fatal(err)
		}
		if d.phase != phase+1 {
			t.Fatalf("fresh notice did not advance phase %d", phase)
		}
		if phase == 4 {
			if !s.stopped {
				t.Fatal("final prompt did not request shutdown")
			}
			break
		}
		if len(s.output) != 0 || !d.responseAt.IsZero() {
			t.Fatal("advance retained stale output or response timestamp")
		}
		if phase == 0 {
			s.output = []byte("Reloaded keybindings")
		} else {
			s.output = []byte("New session started")
		}
		if err := d.step(s); err != nil {
			t.Fatal(err)
		}
		if d.phase != phase+2 || len(s.output) != 0 || !d.responseAt.IsZero() {
			t.Fatal("session transition retained stale notice")
		}
	}
}

func TestProtectedDriverNoticePreservesTokenAndDelay(t *testing.T) {
	t.Setenv("CORVINT_PI_TUI_FIRST", "3")
	t.Setenv("CORVINT_PI_TUI_EXPECT", "unsupported-host-version")
	value, _, err := newDriver(true)
	if err != nil {
		t.Fatal(err)
	}
	d := value.(*protectedDriver)
	if d.expected != "unsupported-host-version" {
		t.Fatal("protected mode ignored expected notice")
	}
	d.sent, d.readyAt = true, time.Now()
	s := &session{output: []byte("unsupported-host-version")}
	if err := d.step(s); err != nil {
		t.Fatal(err)
	}
	if !d.responseAt.IsZero() || d.phase != 0 {
		t.Fatal("notice without provider token advanced prompt")
	}
	s.output = append(s.output, []byte(" PROTECTED_NATIVE_PI_OK_3")...)
	if err := d.step(s); err != nil {
		t.Fatal(err)
	}
	if d.responseAt.IsZero() || d.phase != 0 {
		t.Fatal("provider token and notice bypassed settlement delay")
	}
}
