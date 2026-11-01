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
instrument) echo instrument >> "$s/instrument.log"; [ -f "$s/instrument-prints" ] && { /bin/cat "$s/instrument.txt"; [ -f "$s/reinstall-during-run" ] && : > "$s/reinstalled"; exit 0; }; [ -f "$s/instrument-completes" ] && exit 0; exec /bin/sleep 30;;
force-stop) echo "force-stop $2" >> "$s/device.log"; [ "$2" = local.proof ] && [ ! -f "$s/stuck" ] && : > "$s/stopped"; exit 0;;
esac
exit 1
`,
	"pm": `#!/bin/sh
s=${0%/*}/..
case "$1 $2" in
"list instrumentation")
	# "uninstalled-during-run" removes the instrumentation once it started.
	if [ ! -f "$s/uninstalled" ] && { [ ! -f "$s/uninstalled-during-run" ] || [ ! -f "$s/instrument.log" ]; }; then
		echo "instrumentation:local.proof.test/androidx.test.runner.AndroidJUnitRunner (target=local.proof)"
	fi
	echo "instrumentation:other.test/androidx.test.runner.AndroidJUnitRunner (target=other)";;
"path local.proof.test") echo "package:/data/app/t/base.apk"; [ -f "$s/split" ] && echo "package:/data/app/t/split_config.apk"; exit 0;;
"path local.proof") [ -f "$s/app-missing" ] && exit 1; echo "package:/data/app/a/base.apk";;
*) exit 1;;
esac
`,
	// sha256sum answers with the digest of the APK the fake device holds at
	// that path; "reinstalled" swaps in an APK from another build.
	"sha256sum": `#!/bin/sh
s=${0%/*}/..
case "$1" in
/data/app/t/base.apk) [ -f "$s/reinstalled" ] && { echo "` + otherAPK + `  $1"; exit 0; }; echo "` + testAPK + `  $1";;
/data/app/a/base.apk) echo "` + appAPK + `  $1";;
*) exit 1;;
esac
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

var (
	testAPK  = tr.Digest([]byte("test-apk"))
	appAPK   = tr.Digest([]byte("app-apk"))
	otherAPK = tr.Digest([]byte("other-apk"))
)

// androidDevice writes the fake adb and device into a fresh root with the
// given marker files, and returns the root, the bin directory and an
// Android request that binds the test and app APKs.
func androidDevice(t *testing.T, markers string) (string, string, tr.Request) {
	t.Helper()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(filepath.Join(bin, "device"), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(bin, "adb"), fakeAdb, 0700)
	for name, body := range fakeDevice {
		if name == "pidof" && strings.Contains(","+markers+",", ",pidof-missing,") {
			continue
		}
		write(filepath.Join(bin, "device", name), body, 0700)
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
	for _, m := range strings.Split(markers, ",") {
		if m != "" {
			write(filepath.Join(bin, m), "", 0600)
		}
	}
	write(filepath.Join(root, "apk", "test.apk"), "test-apk", 0600)
	write(filepath.Join(root, "apk", "app.apk"), "app-apk", 0600)
	r := tr.Request{Runner: "android-junit", Root: root, Executable: filepath.Join(bin, "adb"), ExecutableSha256: tr.Digest([]byte(fakeAdb)), Target: "emulator-5554", Project: "local.proof.test/androidx.test.runner.AndroidJUnitRunner", Selectors: []string{"local.proof.Case#test"}, InputFiles: map[string]string{"apk/test.apk": testAPK, "apk/app.apk": appAPK}, ReportDir: filepath.Join(root, "report"), TimeoutSeconds: 1}
	return root, bin, r
}

// TestAndroidInstalledAPKIdentity binds TRE-V0-820 (V1-1554): the runner
// itself compares the installed test and app APKs with the bound digests
// before it instruments, and again after. A matching install is verified and
// instrumented; a reinstall from another build, a split install or a missing
// app is never instrumented and is incomplete, also when the transport drops
// the exit status; a reinstall during the run keeps the tests visible but is
// incomplete.
func TestAndroidInstalledAPKIdentity(t *testing.T) {
	for _, c := range []struct {
		name, markers, reason string
		instrumented          bool
	}{
		{"verified", "instrument-prints", "", true},
		{"mismatch", "instrument-prints,reinstalled", "installed APK identity mismatch", false},
		{"mismatch-status-dropped", "instrument-prints,reinstalled,drop-status", "installed APK identity mismatch", false},
		{"split", "instrument-prints,split", "installed APK identity unobservable", false},
		{"app-missing", "instrument-prints,app-missing", "installed APK identity unobservable", false},
		{"uninstalled", "instrument-prints,uninstalled", "installed APK identity unresolved", false},
		{"reinstalled-during-run", "instrument-prints,reinstall-during-run", "installed APK identity changed after the instrumentation", true},
		{"reinstalled-during-run-status-dropped", "instrument-prints,reinstall-during-run,drop-status", "installed APK identity changed after the instrumentation", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, bin, r := androidDevice(t, c.markers)
			if err := os.WriteFile(filepath.Join(bin, "instrument.txt"), fixture(t, "android.txt"), 0600); err != nil {
				t.Fatal(err)
			}
			inv, err := Build(r)
			if err != nil {
				t.Fatal(err)
			}
			r.TimeoutSeconds = 10
			out, err := tr.Execute(context.Background(), r, inv)
			if err != nil {
				t.Fatal(err)
			}
			o, err := Parse(out.Input)
			if err != nil {
				t.Fatalf("%v: %q", err, out.Input.Stdout)
			}
			o = tr.Normalize(out.Input, o)
			log, _ := os.ReadFile(filepath.Join(bin, "instrument.log"))
			if c.reason == "" {
				if !o.Complete || len(o.Tests) != 4 || string(log) != "instrument\n" {
					t.Fatalf("verified install not instrumented: %+v %q", o, log)
				}
				return
			}
			if o.Complete || (len(log) != 0) != c.instrumented || (c.instrumented && len(o.Tests) != 4) {
				t.Fatalf("unverified install complete, or instrumented=%v wrong: %+v %q", c.instrumented, o, log)
			}
			found := false
			for _, p := range o.Problems {
				found = found || (p.Code == "INSTALLED_APK_UNVERIFIED" && p.Detail == c.reason)
			}
			if !found {
				t.Fatalf("problems %+v, want INSTALLED_APK_UNVERIFIED %q", o.Problems, c.reason)
			}
		})
	}
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
		{"uninstalled", "uninstalled-during-run", tr.CleanupIncomplete, "instrumentation-target-unresolved", []string{"failed", "failed"}},
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
			_, bin, r := androidDevice(t, c.marker)
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
			if !strings.HasPrefix(lines[0], "-s emulator-5554 shell c=local.proof.test/androidx.test.runner.AndroidJUnitRunner; ") || !strings.Contains(lines[0], "; am instrument -w -r -e class local.proof.Case#test local.proof.test/androidx.test.runner.AndroidJUnitRunner; r=$?; ") || len(lines) != 1+len(c.status) {
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
