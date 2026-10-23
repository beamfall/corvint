package perfbudget

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// StartError is a measured or helper executable that could not be started at all (missing,
// not executable). The harness could not run that measurement, so it is a harness failure
// (PRB-V0-029, exit 2), never a scenario failure: a scenario fails only when a command or tool
// actually ran and refused.
type StartError struct {
	Command string
	Err     error
}

func (e *StartError) Error() string { return "cannot start " + e.Command + ": " + e.Err.Error() }
func (e *StartError) Unwrap() error { return e.Err }

// startFailure is the StartError for cmd when it never started, and nil otherwise.
func startFailure(cmd *exec.Cmd, err error) error {
	if err == nil || cmd.Process != nil {
		return nil
	}
	return &StartError{Command: cmd.Path, Err: err}
}

// runContained runs cmd as the leader of its own process group under the repository's owned
// process containment (internal/groupreap, PRB-V0-037): cancelling cmd's context stops the
// leader, and every group member is killed before the leader is reaped, so no descendant of a
// measured or helper process outlives the call. The group sweep is one signal, so a measured
// wall time is not inflated by it.
func runContained(cmd *exec.Cmd) error {
	groupreap.Contain(cmd)
	return groupreap.Run(cmd)
}

// outputContained is cmd.Output under runContained.
func outputContained(cmd *exec.Cmd) ([]byte, error) {
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := runContained(cmd)
	return stdout.Bytes(), err
}

// combinedContained is cmd.CombinedOutput under runContained.
func combinedContained(cmd *exec.Cmd) ([]byte, error) {
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := runContained(cmd)
	return output.Bytes(), err
}

// RunOwned is runContained plus retirement of every descendant that left the group, where the
// platform supports it: the process carries a fresh owner token and, once the leader exits and
// before it is reaped, every token- or ancestry-owned process is killed and awaited
// (groupreap.Retirer). When RunOwned returns, no descendant is still running, so the caller may
// remove the directories they wrote (PRB-V0-037). It is for unmeasured work such as building
// the measured binaries.
func RunOwned(cmd *exec.Cmd) error {
	if !groupreap.RetirementSupported {
		return runContained(cmd)
	}
	retirer, err := groupreap.NewRetirer()
	if err != nil {
		return err
	}
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = append(env, groupreap.OwnerEnvironmentKey+"="+retirer.Token())
	groupreap.Contain(cmd)
	err = groupreap.RunRetiring(cmd, retirer)
	if result := retirer.Result(); !result.Clean() && err == nil {
		err = fmt.Errorf("%s left %d unretired descendants (%v)", cmd.Path, len(result.Unretired), result.Problems)
	}
	return err
}
