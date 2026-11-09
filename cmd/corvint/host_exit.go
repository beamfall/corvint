package main

import (
	"os"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/groupreap"
)

// exitProcess exits with status after retiring what a read abandoned at an adapter watchdog or a
// dogfood event deadline left running, since os.Exit skips the deferred cleanup and context
// cancellation that would otherwise do it: first every live child process group (AHI-048), so no
// Git child writes after its scratch is gone, then the private scratch of any Git status read
// (AHI-044).
func exitProcess(status int) {
	groupreap.KillLive()
	gitstatus.CloseScratch()
	os.Exit(status)
}
