//go:build darwin || linux

package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// fakeAdb is a recording adb on the executor's PATH. It joins its shell
// arguments and runs them with /bin/sh, as adb does on the device, against a
// fake device whose pm, am, dumpsys and pidof keep state in marker files, so
// the cleanup scripts themselves are exercised, not only their argv.
const fakeAdb = `#!/bin/sh
d=${0%/*}
printf '%s\n' "$*" >> "$d/adb.log"
[ "$1" = -s ] && [ "$2" = emulator-5554 ] && [ "$3" = shell ] || exit 64
shift 3
PATH="$d/device" /bin/sh -c "$*"
rc=$?
[ -f "$d/drop-status" ] && exit 0
exit $rc
`

var fakeDevice = map[string]string{
	"am": `#!/bin/sh
s=${0%/*}/..
case "$1" in
instrument) [ -f "$s/instrument-completes" ] && exit 0; exec /bin/sleep 30;;
force-stop) echo "force-stop $2" >> "$s/device.log"; [ "$2" = local.proof ] && [ ! -f "$s/stuck" ] && : > "$s/stopped"; exit 0;;
esac
exit 1
`,
	"pm": `#!/bin/sh
s=${0%/*}/..
[ "$1 $2" = "list instrumentation" ] || exit 1
[ -f "$s/uninstalled" ] || echo "instrumentation:local.proof.test/androidx.test.runner.AndroidJUnitRunner (target=local.proof)"
echo "instrumentation:other.test/androidx.test.runner.AndroidJUnitRunner (target=other)"
`,
	"dumpsys": `#!/bin/sh
s=${0%/*}/..
[ -f "$s/dumpsys-fails" ] && { echo "Can't find service: activity"; exit 1; }
[ -f "$s/dumpsys-headless" ] && exit 0
echo "ACTIVITY MANAGER RUNNING PROCESSES (dumpsys activity processes)"
[ -f "$s/stopped" ] || echo "      mClass=ComponentInfo{local.proof.test/androidx.test.runner.AndroidJUnitRunner} mFinished=false"
echo "      mClass=ComponentInfo{other.test/androidx.test.runner.AndroidJUnitRunner} mFinished=false"
`,
	"pidof": `#!/bin/sh
s=${0%/*}/..
[ -f "$s/pidof-broken" ] && exit 2
[ "$1" = local.proof ] && [ ! -f "$s/stopped" ] && { echo 1234; exit 0; }
exit 1
`,
}

// TestAndroidTimeoutStopsDeviceProcesses binds TRE-V0-087 for the Android
// instrumentation profiles (V1-1553): a timed-out run force-stops the
// instrumentation's target package, which is the only stop observed to end
// it live, verifies the stop, and records an incomplete cleanup truthfully
// when the device does not stop, the instrumentation is not resolvable, or
// the transport drops the remote exit status.
func TestAndroidTimeoutStopsDeviceProcesses(t *testing.T) {
	for _, c := range []struct {
		name, marker, outcome, verify string
		status                        []string
	}{
		{"clean", "", tr.CleanupClean, "verified", []string{"ok", "ok"}},
		{"stuck", "stuck", tr.CleanupIncomplete, "instrumentation-still-active", []string{"ok", "failed"}},
		{"uninstalled", "uninstalled", tr.CleanupIncomplete, "instrumentation-target-unresolved", []string{"failed", "failed"}},
		{"exit-status-dropped", "drop-status,stuck", tr.CleanupIncomplete, "instrumentation-still-active", []string{"ok", "failed"}},
		// A failed probe is not an absent process: a failing or headerless
		// dumpsys, and a pidof that errors or is missing, never verify.
		{"dumpsys-fails", "dumpsys-fails,stuck,pidof-broken", tr.CleanupIncomplete, "instrumentation-probe-failed", []string{"ok", "failed"}},
		{"dumpsys-headless", "dumpsys-headless,stuck", tr.CleanupIncomplete, "instrumentation-probe-failed", []string{"ok", "failed"}},
		{"pidof-broken", "pidof-broken", tr.CleanupIncomplete, "process-probe-failed", []string{"ok", "failed"}},
		{"pidof-missing", "pidof-missing", tr.CleanupIncomplete, "process-probe-failed", []string{"ok", "failed"}},
		{"completed", "instrument-completes", tr.CleanupNotRequired, "", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, _ := filepath.EvalSymlinks(t.TempDir())
			bin := filepath.Join(root, "bin")
			if err := os.MkdirAll(filepath.Join(bin, "device"), 0700); err != nil {
				t.Fatal(err)
			}
			write := func(name, body string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(bin, name), []byte(body), mode); err != nil {
					t.Fatal(err)
				}
			}
			write("adb", fakeAdb, 0700)
			for name, body := range fakeDevice {
				if name == "pidof" && c.marker == "pidof-missing" {
					continue
				}
				write(filepath.Join("device", name), body, 0700)
			}
			// The device PATH is only the fakes and grep, so a missing pidof
			// is missing on every host.
			grep, err := exec.LookPath("grep")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(grep, filepath.Join(bin, "device", "grep")); err != nil {
				t.Fatal(err)
			}
			for _, m := range strings.Split(c.marker, ",") {
				if m != "" {
					write(m, "", 0600)
				}
			}
			if err := os.WriteFile(filepath.Join(root, "source"), []byte("source"), 0600); err != nil {
				t.Fatal(err)
			}
			r := tr.Request{Runner: "android-junit", Root: root, Executable: filepath.Join(bin, "adb"), ExecutableSha256: tr.Digest([]byte(fakeAdb)), Target: "emulator-5554", Project: "local.proof.test/androidx.test.runner.AndroidJUnitRunner", Selectors: []string{"local.proof.Case#test"}, InputFiles: map[string]string{"source": tr.Digest([]byte("source"))}, ReportDir: filepath.Join(root, "report"), TimeoutSeconds: 1}
			inv, err := Build(r)
			if err != nil {
				t.Fatal(err)
			}
			out, err := tr.Execute(context.Background(), r, inv)
			if err != nil {
				t.Fatal(err)
			}
			d := out.DeviceCleanup
			if d == nil || d.Outcome != c.outcome || len(d.Steps) != len(c.status) {
				t.Fatalf("cleanup %+v, want %s", d, c.outcome)
			}
			for i, s := range c.status {
				if d.Steps[i].Status != s {
					t.Fatalf("step %+v, want %s", d.Steps[i], s)
				}
			}
			if c.verify != "" && !strings.Contains(d.Steps[1].Detail, c.verify) {
				t.Fatalf("verify step %+v, want %s", d.Steps[1], c.verify)
			}
			calls, _ := os.ReadFile(filepath.Join(bin, "adb.log"))
			lines := strings.Split(strings.TrimSuffix(string(calls), "\n"), "\n")
			if lines[0] != "-s emulator-5554 shell am instrument -w -r -e class local.proof.Case#test local.proof.test/androidx.test.runner.AndroidJUnitRunner" || len(lines) != 1+len(c.status) {
				t.Fatalf("adb invocations: %q", lines)
			}
			stops, _ := os.ReadFile(filepath.Join(bin, "device.log"))
			problem := false
			for _, p := range out.Input.ExecutionProblems {
				problem = problem || p.Code == "device-cleanup"
			}
			switch c.outcome {
			case tr.CleanupNotRequired:
				if out.Input.TimedOut || len(stops) != 0 || problem {
					t.Fatalf("completed run stopped the device: %q %+v", stops, out.Input)
				}
				return
			case tr.CleanupClean:
				if problem || string(stops) != "force-stop local.proof\nforce-stop local.proof.test\n" {
					t.Fatalf("target not force-stopped cleanly: %q %+v", stops, out.Input.ExecutionProblems)
				}
			default:
				if !problem {
					t.Fatalf("incomplete cleanup claimed clean: %+v", out.Input.ExecutionProblems)
				}
			}
			if !out.Input.TimedOut {
				t.Fatalf("timeout not observed: %+v", out.Input)
			}
			if o, err := Parse(out.Input); err == nil && o.Complete {
				t.Fatal("timed-out run observed complete")
			}
		})
	}
}
