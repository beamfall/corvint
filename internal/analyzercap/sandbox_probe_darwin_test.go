//go:build darwin

package analyzercap

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/analyzerexec"
)

// requireSandboxedPayload runs the production sandbox profile around binary,
// with the same argv, directory, and environment as the contained launch, but
// outside the 100 ms plan cap. The wall is a hang detector (decision 0082), so
// only an observed refusal skips and any other payload failure is a defect.
func requireSandboxedPayload(t *testing.T, binary string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", analyzerexec.SandboxProfile(binary), binary)
	command.Dir, command.Env = "/", []string{}
	command.Stdin = bytes.NewReader([]byte("probe"))
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err := command.Run()
	if bytes.Contains(stderr.Bytes(), []byte("sandbox_apply: Operation not permitted")) {
		t.Skipf("parent process forbids nested Darwin sandbox installation: %s", stderr.Bytes())
	}
	if err != nil {
		t.Fatalf("production Darwin sandbox profile did not run a trivial payload outside the plan cap: %v stderr=%q", err, stderr.Bytes())
	}
}
