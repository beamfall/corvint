package mutate

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// sandbox confines one go test run to the paths the runner owns and cuts it
// off from the network: sandbox-exec on macOS, bwrap on Linux. A host without
// a sandbox runs no cited test at all.
type sandbox struct {
	name    string
	confine func(writable []string) ([]string, error)
}

// findSandbox is the seam tests use to simulate a host without a sandbox.
var findSandbox = hostSandbox

// HostSandbox names the sandbox the runner would use on this host, or says
// why the host cannot run cited tests.
func HostSandbox() (string, error) {
	box, err := findSandbox()
	return box.name, err
}

// sandboxes is the closed table of host sandboxes: one executable per
// operating system, and no run at all where the table has no row. macOS names
// the system binary by absolute path so PATH cannot substitute it.
var sandboxes = map[string]struct {
	executable string
	confine    func(string) func([]string) ([]string, error)
}{
	"darwin": {"/usr/bin/sandbox-exec", seatbelt},
	"linux":  {"bwrap", bubblewrap},
}

func hostSandbox() (sandbox, error) {
	entry, known := sandboxes[runtime.GOOS]
	if !known {
		return sandbox{}, fmt.Errorf("no sandbox is implemented for %s", runtime.GOOS)
	}
	executable, err := exec.LookPath(entry.executable)
	if err != nil {
		return sandbox{}, fmt.Errorf("%s is not available", filepath.Base(entry.executable))
	}
	return sandbox{name: filepath.Base(entry.executable), confine: entry.confine(executable)}, nil
}

// seatbelt renders the argv prefix that runs a command under a macOS
// sandbox-exec profile allowing writes only under the writable directories.
func seatbelt(executable string) func([]string) ([]string, error) {
	return func(writable []string) ([]string, error) {
		profile, err := seatbeltProfile(writable)
		if err != nil {
			return nil, err
		}
		return []string{executable, "-p", profile}, nil
	}
}

// seatbeltProfile denies every network operation, every signal to a process
// outside the sandbox, and every write outside the writable directories and
// /dev/null. Directories are resolved through symlinks because the kernel
// matches real paths.
func seatbeltProfile(writable []string) (string, error) {
	rules := []string{`(literal "/dev/null")`}
	for _, directory := range writable {
		resolved, err := filepath.EvalSymlinks(directory)
		if err != nil {
			return "", fmt.Errorf("mutate: resolve sandbox path: %w", err)
		}
		if strings.ContainsAny(resolved, "\"\\") {
			return "", fmt.Errorf("mutate: sandbox path %q cannot be quoted in a profile", resolved)
		}
		rules = append(rules, `(subpath "`+resolved+`")`)
	}
	return "(version 1)(allow default)(deny network*)(deny signal)(allow signal (target same-sandbox))" +
		"(deny file-write*)(allow file-write* " + strings.Join(rules, " ") + ")", nil
}

// bubblewrap renders the argv prefix that runs a command under bwrap: the
// whole filesystem read-only except the writable directories, a fresh /dev
// and /proc, no network, and a private pid namespace whose death takes every
// descendant with it.
func bubblewrap(executable string) func([]string) ([]string, error) {
	return func(writable []string) ([]string, error) {
		argv := []string{executable, "--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc"}
		for _, directory := range writable {
			resolved, err := filepath.EvalSymlinks(directory)
			if err != nil {
				return nil, fmt.Errorf("mutate: resolve sandbox path: %w", err)
			}
			argv = append(argv, "--bind", resolved, resolved)
		}
		return append(argv, "--unshare-net", "--unshare-pid", "--die-with-parent", "--"), nil
	}
}
