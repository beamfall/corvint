package main

import (
	"context"
	"errors"
	"os/exec"
)

// gitExitOwnershipRefusal is dubiousOwnershipRefusal over the stderr a failed
// exec.Cmd.Output call retained, for verbs that otherwise map any Git exit to
// their own generic refusal (V1-2353). It returns nil for any other error.
func gitExitOwnershipRefusal(err error) error {
	var exit *exec.ExitError
	if !errors.As(err, &exit) || len(exit.Stderr) == 0 {
		return nil
	}
	return dubiousOwnershipRefusal(errors.New(string(exit.Stderr)))
}

// repositoryOwnershipRefusal probes once, with the hermetic Git environment,
// whether Git refuses root for dubious ownership. Verbs whose Git reads report
// only an uncoded failure (init, adopt and the guidance verbs) call it on
// their failure path alone, so a successful run pays nothing (V1-2353,
// GPK-V0-071). It returns nil when Git is absent or accepts the repository.
func repositoryOwnershipRefusal(ctx context.Context, root string) error {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return nil
	}
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	defer cancel()
	_, err = hermeticGitCommand(deadline, gitExecutable, root, "rev-parse", "--git-dir").Output()
	return gitExitOwnershipRefusal(err)
}
