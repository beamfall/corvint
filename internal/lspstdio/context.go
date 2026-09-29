// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/goplsclient"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/procgroup"
)

var errRepositoryChanged = errors.New("repository changed")
var errCoreUnavailable = errors.New("core unavailable")

type editorContextParams struct {
	TextDocument *struct {
		URI *string `json:"uri"`
	} `json:"textDocument"`
	Task  *string `json:"task"`
	Limit *int    `json:"limit"`
}

func parseContextParams(raw []byte, root string) (contextWorkerInput, string, error) {
	var p editorContextParams
	if len(raw) > contextInputLimit || jsonv2.Unmarshal(raw, &p, jsonv2.RejectUnknownMembers(true)) != nil || p.TextDocument == nil || p.TextDocument.URI == nil || p.Task == nil || p.Limit == nil {
		return contextWorkerInput{}, "", ErrFraming
	}
	uri := *p.TextDocument.URI
	u, err := url.Parse(uri)
	if err != nil || len(uri) > 4096 || u.Scheme != "file" || u.Host != "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (&url.URL{Scheme: "file", Path: u.Path}).String() != uri {
		return contextWorkerInput{}, "", ErrFraming
	}
	subject, err := filepath.Rel(root, u.Path)
	input := contextWorkerInput{Subject: filepath.ToSlash(subject), Task: *p.Task, Limit: *p.Limit}
	if err != nil || !validWorkerInput(input) || filepath.Clean(u.Path) != u.Path || strings.ContainsAny(u.Path, "\\\x00") {
		return contextWorkerInput{}, "", ErrFraming
	}
	return input, uri, nil
}

type repositoryObservation struct {
	root       os.FileInfo
	repository gokernel.Repository
	head       string
}

func symbolicHEAD(ctx context.Context, root string) (string, error) {
	executable, err := gitstatus.Pin()
	if err != nil {
		return "", errCoreUnavailable
	}
	o := procgroup.Run(ctx, procgroup.Spec{Argv: []string{executable, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-C", root, "symbolic-ref", "--quiet", "HEAD"}, Dir: root, Env: gokernel.SanitizedGitEnvironment(), Timeout: contextBudget, ShutdownTimeout: contextShutdownBudget, OutputLimit: 4096, StderrLimit: 4096})
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if !o.WaitCompleted || !o.PipesDrained || !o.OwnedProcessGroupCleanup || o.DescendantCleanupQualification != "" || o.OutputOverflow {
		return "", errCoreUnavailable
	}
	if o.ExitStatus == 1 && len(o.Stdout) == 0 && len(o.Stderr) == 0 {
		return "detached", nil
	}
	if o.Err != nil || o.ExitStatus != 0 || len(o.Stdout) == 0 {
		return "", errCoreUnavailable
	}
	return string(o.Stdout), nil
}

func observeRepository(ctx context.Context, root string) (repositoryObservation, error) {
	first, err := os.Stat(root)
	if err != nil || !first.IsDir() {
		return repositoryObservation{}, errCoreUnavailable
	}
	head, err := symbolicHEAD(ctx, root)
	if err != nil {
		return repositoryObservation{}, err
	}
	repository, err := gokernel.ProbeRepositoryContext(ctx, root)
	if err != nil {
		return repositoryObservation{}, errCoreUnavailable
	}
	lastHead, err := symbolicHEAD(ctx, root)
	if err != nil {
		return repositoryObservation{}, err
	}
	last, err := os.Stat(root)
	if err != nil {
		return repositoryObservation{}, errCoreUnavailable
	}
	if head != lastHead || !os.SameFile(first, last) {
		return repositoryObservation{}, errRepositoryChanged
	}
	return repositoryObservation{first, repository, head}, nil
}
func sameRepository(a, b repositoryObservation) bool {
	return os.SameFile(a.root, b.root) && a.repository == b.repository && a.head == b.head
}
func coreMatches(raw []byte, observed repositoryObservation) bool {
	var object struct {
		Repository *bridge.RepositoryBinding `json:"repository"`
	}
	if json.Unmarshal(raw, &object) != nil {
		return false
	}
	b := object.Repository
	if b == nil {
		return true
	} // Valid binding-free abstentions remain successful.
	r := observed.repository
	state := "MIXED"
	if r.WorktreeState == "clean" {
		state = "CLEAN"
	}
	return b.CommitRevision == r.CommitRevision && b.TreeRevision == r.TreeRevision && b.ObjectFormat == r.ObjectFormat && b.WorktreeState == state && b.DirtyPathCount == r.DirtyPathCount && b.DirtyPathsSHA256 == r.DirtyPathsSHA
}

// The observations bracket Core, not the filesystem's lifetime. Unseen ABA and
// mutations after the final observation are intentionally not claimed away.
func editorContext(ctx context.Context, identity workerIdentity, root string, input contextWorkerInput) (json.RawMessage, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return nil, errCoreUnavailable
	}
	return contextEvidence(ctx, root, input, observeRepository, func(ctx context.Context, root string, input contextWorkerInput) (json.RawMessage, error) {
		return runContextWorker(ctx, identity, root, input)
	})
}

func contextEvidence(ctx context.Context, root string, input contextWorkerInput, observe func(context.Context, string) (repositoryObservation, error), coreCall func(context.Context, string, contextWorkerInput) (json.RawMessage, error)) (json.RawMessage, error) {
	before, err := observe(ctx, root)
	if err != nil {
		return nil, err
	}
	core, err := coreCall(ctx, root, input)
	if err != nil {
		return nil, err
	}
	after, err := observe(ctx, root)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !sameRepository(before, after) || !coreMatches(core, before) {
		return nil, errRepositoryChanged
	}
	return core, nil
}

func contextResponse(out io.Writer, id json.RawMessage, core json.RawMessage, source goplsclient.Snapshot, session string, err error, current, cancelled bool) error {
	code, message, reason := 0, "", ""
	switch {
	case cancelled || errors.Is(err, context.Canceled):
		code, message, reason = -32800, "Request cancelled", "CANCELLED"
	case errors.Is(err, context.DeadlineExceeded):
		code, message, reason = -32800, "Request deadline exceeded", "DEADLINE"
	case !current:
		code, message, reason = -32801, "Context stale", "CONTENT_CHANGED"
	case errors.Is(err, errRepositoryChanged):
		code, message, reason = -32801, "Context stale", "REPOSITORY_CHANGED"
	case err != nil:
		code, message, reason = -32001, "Core unavailable", "CORE_UNAVAILABLE"
	}
	envelope := map[string]any{"jsonrpc": "2.0", "id": id}
	if code != 0 {
		envelope["error"] = map[string]any{"code": code, "message": message, "data": map[string]string{"reason": reason}}
	} else {
		digest := sha256.Sum256([]byte(source.Text))
		envelope["result"] = map[string]any{"schema": "corvint-editor-context/0", "core": core, "overlayObservation": map[string]any{"sessionID": session, "captureID": source.Identity, "uri": source.URI, "version": source.Version, "digestAlgorithm": "sha256", "digest": hex.EncodeToString(digest[:])}, "inclusionReason": "requested-open-document-subject"}
	}
	body, e := json.Marshal(envelope)
	if e != nil || len(body) > contextOutputLimit {
		envelope = map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32001, "message": "Core unavailable", "data": map[string]string{"reason": "CORE_UNAVAILABLE"}}}
		body, _ = json.Marshal(envelope)
	}
	_, e = fmt.Fprintf(out, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return e
}
