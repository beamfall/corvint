package main

import (
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

// exitProcess exits with status after removing the private scratch of any Git status read an
// adapter watchdog abandoned: that read is still running, and os.Exit skips the deferred cleanup
// that would otherwise remove it (AHI-044).
func exitProcess(status int) {
	gitstatus.CloseScratch()
	os.Exit(status)
}

// adapterStdout turns a hook stdout the host closed into a write error instead of a SIGPIPE death,
// and names that fault on stderr (AHI-044).
func adapterStdout(stdout, stderr io.Writer) io.Writer {
	notifyBrokenPipe()
	return hookStdout{stdout: stdout, stderr: stderr}
}

// hookStdout names a hook stdout the host closed before the adapter wrote on stderr, the one
// channel left, since the host can no longer read the degradation stdout would have carried. Hook
// adapters still exit 0, so a closed pipe never becomes a failed or blocking hook (AHI-044); the
// `pi-tool` command, which is not a hook, keeps its own failure status.
type hookStdout struct {
	stdout, stderr io.Writer
}

func (w hookStdout) Write(p []byte) (int, error) {
	n, err := w.stdout.Write(p)
	if err != nil {
		_, _ = io.WriteString(w.stderr, "Corvint FALLBACK degraded: hook-stdout-unwritable; coding continues\n")
	}
	return n, err
}
