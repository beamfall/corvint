// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

type absenceResult struct {
	KillSwitch string `json:"killSwitch"`
	Window     int    `json:"window"`
	Absent     int    `json:"absent"`
	Error      string `json:"error"`
}

// absence runs absence.jq for the live epoch over names, newest first.
func absence(t *testing.T, names []string, extra ...string) absenceResult {
	t.Helper()
	args := append([]string{"-R", "-s", "-c", "--arg", "epoch", forwardEpoch(t)}, extra...)
	cmd := exec.Command("jq", append(args, "-f", "absence.jq")...)
	cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("absence.jq: %v", err)
	}
	var r absenceResult
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	return r
}

// TestCINV0861RowAbsenceTripsTheKillSwitch pins CIN-V0-861 (V1-3332): three runs of the window
// that the collector names NOROW (no selection document, or no single epoch) trip the kill
// switch, while EPOCH markers and rows of an earlier epoch take no place, so epoch-1 runs are
// never counted as absence; INFRA and NARROWED rows take no place either, as in bar.jq.
func TestCINV0861RowAbsenceTripsTheKillSwitch(t *testing.T) {
	// The live epoch comes from ci.yml; "earlier" and "later" are the epochs around it (CIN-V0-960).
	live := forwardEpoch(t)
	earlier, later := shiftEpoch(t, live, -1), shiftEpoch(t, live, 1)
	head := func(n int) string { return fmt.Sprintf("%040x", n) }
	row := func(class string, n int, epoch string) string {
		return fmt.Sprintf("ci-forward-%s-pr%d-%s-e%s-f0", class, n, head(n), epoch)
	}
	norow := func(n int, why string) string {
		return fmt.Sprintf("ci-forward-NOROW-pr%d-%s-e%s-r%s", n, head(n), live, why)
	}
	epochMarker := func(n int, epoch string) string {
		return fmt.Sprintf("ci-forward-EPOCH-pr%d-%s-e%s", n, head(n), epoch)
	}
	outage := []string{norow(1, "nodoc"), norow(2, "nodoc"), norow(3, "mixed")}
	with := func(first []string, rest ...string) []string { return append(append([]string(nil), first...), rest...) }
	for _, tc := range []struct {
		name           string
		names          []string
		extra          []string
		kill           string
		absent, window int
	}{
		{"an outage", outage, nil, "TRIPPED", 3, 3},
		{"two absent", outage[:2], nil, "clear", 2, 2},
		{"a lower threshold", outage[:2], []string{"--arg", "threshold", "2"}, "TRIPPED", 2, 2},
		{"rows beside", with([]string{row("CLEAN", 9, live), row("MISS", 8, live)}, outage...), nil, "TRIPPED", 3, 5},
		{"epoch-1 markers take no place", []string{epochMarker(1, earlier), epochMarker(2, earlier), epochMarker(3, earlier), row("CLEAN", 4, live)}, nil, "clear", 0, 1},
		{"epoch-1 rows take no place", []string{row("INCOMPLETE", 1, earlier), row("CLEAN", 2, earlier), row("CLEAN", 3, earlier)}, nil, "clear", 0, 0},
		{"another epoch's NOROW takes no place", []string{strings.Replace(norow(1, "nodoc"), "-e"+live+"-", "-e"+earlier+"-", 1), strings.Replace(norow(2, "nodoc"), "-e"+live+"-", "-e"+later+"-", 1)}, nil, "clear", 0, 0},
		{"INFRA and NARROWED rows take no place", with([]string{row("INFRA", 7, live), row("NARROWED", 6, live)}, outage...), nil, "TRIPPED", 3, 3},
		// A re-run that wrote its row replaces the older NOROW of the same commit.
		{"a re-run with its row", []string{row("CLEAN", 1, live), norow(1, "nodoc"), norow(2, "nodoc"), norow(3, "nodoc")}, nil, "clear", 2, 3},
		{"an unknown reason is no marker", []string{norow(1, "late"), norow(2, "late"), norow(3, "late")}, nil, "clear", 0, 0},
		{"a malformed threshold", outage, []string{"--arg", "threshold", "0"}, "TRIPPED", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := absence(t, tc.names, tc.extra...)
			if got.KillSwitch != tc.kill || got.Absent != tc.absent || got.Window != tc.window {
				t.Fatalf("got %+v, want kill %s absent %d window %d", got, tc.kill, tc.absent, tc.window)
			}
		})
	}
	// The window is the latest 20 entries: three NOROW markers older than 20 rows do not trip.
	var names []string
	for n := 23; n > 3; n-- {
		names = append(names, row("CLEAN", n, live))
	}
	if got := absence(t, with(names, outage...)); got.KillSwitch != "clear" || got.Window != 20 || got.Absent != 0 {
		t.Fatalf("window: %+v", got)
	}
}
