// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"os/exec"
	"testing"
)

// TestCINV02140AbsenceAlarmReplay exercises CIN-V0-2140..2142 through the
// standalone alarm's entry point, with GitHub reads replaced by frozen responses.
func TestCINV02140AbsenceAlarmReplay(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("developer absence alarm needs python3")
	}
	cmd := exec.Command(python, "-B", "absence_alarm_test.py")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("absence alarm replay: %v\n%s", err, out)
	}
	t.Logf("%s", out)
}
