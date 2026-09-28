package doccompiler

import (
	"context"
	"github.com/Beamfall/corvint/internal/groupreap"
	"os/exec"
)

func navGitText(ctx context.Context, git, root, temp string, args ...string) (string, error) {
	r, err := runCommand(ctx, git, append([]string{"--no-optional-locks", "-C", root}, args...), temp, append(isolatedEnvironment(git, temp), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1"), 40<<20, 64<<10)
	return r.stdout, err
}

func navGitBlob(ctx context.Context, git, root, object, temp string) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, git, "--no-optional-locks", "-C", root, "show", object)
	cmd.Dir = temp
	cmd.Env = append(isolatedEnvironment(git, temp), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1")
	configureProcess(cmd)
	output := &boundedBuffer{limit: (1 << 20) + 1, cancel: cancel}
	stderr := &boundedBuffer{limit: 64 << 10, cancel: cancel}
	cmd.Stdout = output
	cmd.Stderr = stderr
	if err := groupreap.Run(cmd); err != nil || output.exceeded || stderr.exceeded {
		return nil, failure("trusted-nav-source", "cannot read bounded committed blob")
	}
	return append([]byte{}, output.data.Bytes()...), nil
}
