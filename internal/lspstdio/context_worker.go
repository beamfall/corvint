// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const contextInputLimit = 256 << 10
const contextOutputLimit = 1 << 20
const contextBudget = 20 * time.Second
const contextShutdownBudget = 2 * time.Second

// ContextWorkerMode is an internal, fixed one-shot entry point of this executable.
const ContextWorkerMode = "--internal-context-worker"

type contextWorkerInput struct {
	Subject string `json:"subject"`
	Task    string `json:"task"`
	Limit   int    `json:"limit"`
}

func validWorkerInput(input contextWorkerInput) bool {
	return input.Subject != "" && input.Subject != "." && len(input.Subject) <= 4096 && !filepath.IsAbs(input.Subject) && filepath.ToSlash(filepath.Clean(input.Subject)) == input.Subject && input.Subject != ".." && !strings.HasPrefix(input.Subject, "../") && strings.TrimSpace(input.Task) != "" && len(input.Task) <= 32000 && input.Limit >= 1 && input.Limit <= 50
}

// ServeContextWorker must be dispatched before any other work in main. Normal
// input EOF terminates the one message, not the operation. Editor EOF cancels the
// parent's runner. The parent alone owns termination of this worker group.
func ServeContextWorker(root string, in io.Reader, out io.Writer) int {
	if gitstatus.EnableOwnedWorker() != nil {
		return 1
	}
	if !filepath.IsAbs(root) {
		return 1
	}
	raw, err := io.ReadAll(io.LimitReader(in, contextInputLimit+1))
	if err != nil || len(raw) > contextInputLimit {
		return 1
	}
	var input contextWorkerInput
	if jsonv2.Unmarshal(raw, &input, jsonv2.RejectUnknownMembers(true)) != nil || !validWorkerInput(input) {
		return 1
	}
	if _, err := gitstatus.Pin(); err != nil {
		return 1
	}
	registry, failure := bridge.NewTaskReview(root)
	if failure != nil {
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), contextBudget)
	defer cancel()
	arguments, _ := json.Marshal(input)
	result, failure := registry.Call(ctx, bridge.ToolContext, arguments)
	if failure != nil || ctx.Err() != nil {
		return 1
	}
	object, failure := result.Object()
	if failure != nil {
		return 1
	}
	body, err := json.Marshal(object)
	if err != nil || len(body) > contextOutputLimit {
		return 1
	}
	if _, err = out.Write(body); err != nil {
		return 1
	}
	return 0
}

func contextWorkerSpec(executable, root string, input []byte) procgroup.Spec {
	return procgroup.Spec{Argv: []string{executable, ContextWorkerMode, root}, Dir: root,
		Env: append(gokernel.SanitizedGitEnvironment(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local"), Stdin: input,
		Timeout: contextBudget, ShutdownTimeout: contextShutdownBudget,
		InputLimit: contextInputLimit, OutputLimit: contextOutputLimit, StderrLimit: 4096,
		ObserveDescendants: true}
}

func contextWorkerCleanup(observation procgroup.Observation) bool {
	return observation.WaitCompleted && observation.PipesDrained && observation.OwnedProcessGroupCleanup && observation.DescendantCleanupStatus == "owned-process-group" && observation.DescendantCleanupQualification == "" && observation.DescendantObservation != nil && observation.DescendantObservation.Scope == "observed-pid-start-identities" && observation.DescendantObservation.IntervalMS > 0 && observation.DescendantObservation.Absent && len(observation.DescendantObservation.Failures) == 0
}

func runContextWorker(ctx context.Context, identity workerIdentity, root string, input contextWorkerInput) (json.RawMessage, error) {
	if !identity.current() {
		return nil, errors.New("core unavailable")
	}
	body, err := json.Marshal(input)
	if err != nil || !validWorkerInput(input) {
		return nil, errors.New("core unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, contextBudget)
	defer cancel()
	observation := procgroup.Run(ctx, contextWorkerSpec(identity.path, root, body))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !identity.current() || observation.Err != nil || observation.ExitStatus != 0 || !contextWorkerCleanup(observation) || !json.Valid(observation.Stdout) {
		return nil, errors.New("core unavailable")
	}
	var result bridge.Result
	if jsonv2.Unmarshal(observation.Stdout, &result, jsonv2.RejectUnknownMembers(true)) != nil {
		return nil, errCoreUnavailable
	}
	if _, failure := result.Object(); failure != nil {
		return nil, errCoreUnavailable
	}
	return observation.Stdout, nil
}

// This binds the trusted operator's on-disk build at session startup and both
// request endpoints. It does not claim exec-fd or hostile replacement isolation.
type workerIdentity struct {
	path   string
	digest [32]byte
}

func captureWorkerIdentity() (workerIdentity, error) {
	path, err := os.Executable()
	if err != nil {
		return workerIdentity{}, err
	}
	digest, err := workerDigest(path)
	if err != nil {
		return workerIdentity{}, err
	}
	return workerIdentity{path, digest}, nil
}
func workerDigest(path string) ([32]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 128<<20+1))
	if err != nil || len(b) > 128<<20 {
		return [32]byte{}, errors.New("worker identity unavailable")
	}
	return sha256.Sum256(b), nil
}
func (i workerIdentity) current() bool {
	digest, err := workerDigest(i.path)
	return err == nil && digest == i.digest
}
