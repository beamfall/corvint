//go:build darwin || linux

package store_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

func init() { leaderFault = forgedIdentityLeader }

// forgedIdentityLeader boots with a start identity that is not its own, so
// the supervisor refuses it after the spawn with an empty outcome class. A
// detached holder in its own session keeps standard output open past the
// drain, so the drain cannot prove the output closed.
func forgedIdentityLeader(dir string) {
	raw, e := os.ReadFile(filepath.Join(dir, "capsule"))
	var c supervisor.Capsule
	if e == nil {
		e = json.Unmarshal(raw, &c)
	}
	if e != nil {
		os.Exit(1)
	}
	holder := exec.Command(os.Args[0], "output-holder")
	holder.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	holder.Stdout = os.Stdout
	if e = holder.Start(); e != nil {
		os.Exit(1)
	}
	if e = supervisor.Publish(dir, "boot", supervisor.Boot{Effect: c.Effect, PID: os.Getpid(), Started: "forged"}); e != nil {
		os.Exit(1)
	}
	time.Sleep(90 * time.Second)
	os.Exit(1)
}
