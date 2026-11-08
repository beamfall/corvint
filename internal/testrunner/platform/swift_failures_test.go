// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"bytes"
	"reflect"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// RSP-V0-003: only terminals validated by the native protocol can survive
// an incomplete capture. An issue, forged identity or duplicate key is insufficient.
func TestSwiftTerminalFailuresRequireValidatedTerminal(t *testing.T) {
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		t.Run(runner, func(t *testing.T) {
			var start, terminal []byte
			var id string
			if runner == "swift-testing" {
				id = "Proof.fail()/Proof.swift:2:2"
				start = []byte(`{"version":0,"kind":"test","payload":{"id":"Proof.fail()/Proof.swift:2:2","name":"fail()","kind":"function"}}
{"version":0,"kind":"event","payload":{"kind":"runStarted"}}
{"version":0,"kind":"event","payload":{"kind":"testStarted","testID":"Proof.fail()/Proof.swift:2:2"}}
{"version":0,"kind":"event","payload":{"kind":"issueRecorded","testID":"Proof.fail()/Proof.swift:2:2","issue":{"isKnown":false}}}
`)
				terminal = []byte(`{"version":0,"kind":"event","payload":{"kind":"testEnded","testID":"Proof.fail()/Proof.swift:2:2"}}
`)
			} else {
				id = "Proof.Tests/testFail"
				start = []byte("Test Suite 'Selected tests' started at 2026-10-08 11:57:58.610.\nTest Case '-[Proof.Tests testFail]' started.\n/Tests.swift:2: error: -[Proof.Tests testFail] : XCTAssert failed\n")
				terminal = []byte("Test Case '-[Proof.Tests testFail]' failed (0.001 seconds).\n")
			}
			capture := func(raw []byte) tr.Input {
				in := tr.Input{Runner: runner, TimedOut: true, ExitCode: 99}
				if runner == "swift-testing" {
					in.Reports = map[string][]byte{"swift-events.jsonl": raw}
				} else {
					in.Stdout = raw
				}
				return in
			}
			alien := bytes.ReplaceAll(terminal, []byte("testFail"), []byte("testAlien"))
			alien = bytes.ReplaceAll(alien, []byte("Proof.fail()"), []byte("Proof.alien()"))
			badCaptures := [][]byte{start, terminal, append(append([]byte(nil), start...), alien...)}
			if runner == "swift-testing" {
				duplicate := bytes.Replace(terminal, []byte(`"kind":"testEnded"`), []byte(`"kind":"testEnded","kind":"testEnded"`), 1)
				badCaptures = append(badCaptures, append(append([]byte(nil), start...), duplicate...))
			}
			for _, bad := range badCaptures {
				if got := SwiftTerminalFailures(capture(bad)); len(got) != 0 {
					t.Fatalf("unvalidated failure retained: %v", got)
				}
			}
			completeTerminal := append(append([]byte(nil), start...), terminal...)
			if got := SwiftTerminalFailures(capture(completeTerminal)); !reflect.DeepEqual(got, []string{id}) {
				t.Fatalf("validated terminal lost: %v", got)
			}
			malformedTail := append(completeTerminal, []byte("{broken\n")...)
			if got := SwiftTerminalFailures(capture(malformedTail)); !reflect.DeepEqual(got, []string{id}) {
				t.Fatalf("later malformed record erased failure: %v", got)
			}
		})
	}
}
