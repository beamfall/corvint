//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	stdjson "encoding/json"
	json "encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	profile                                                           = "corvint-analyzer-candidate/experimental"
	maxInputs, maxInputBytes, maxAggregateInputBytes                  = 128, 1 << 20, 4 << 20
	maxRequestBytes, maxArtifactBytes, maxManifestBytes, maxCoreBytes = 1_500_000, 64 << 20, 64 << 10, 128 << 20
	ioChunkBytes                                                      = 64 << 10
)

type stringsFlag []string

func (v *stringsFlag) String() string { return strings.Join(*v, ",") }
func (v *stringsFlag) Set(s string) error {
	if s == "" {
		return errors.New("empty --input")
	}
	if len(*v) == maxInputs {
		return errors.New("input count limit exceeded")
	}
	*v = append(*v, s)
	return nil
}

type options struct {
	targetRepo, targetRev, manifest          string
	inputs                                   stringsFlag
	targetOS, targetArch, targetABI, receipt string
	runs                                     int
	timeout                                  time.Duration
}

// These types are the frozen corvint-analyzer-candidate/experimental envelope.
// Harness identity intentionally exists only in dogfoodManifest and receipts.
type target struct {
	OS           string   `json:"os"`
	Architecture string   `json:"architecture"`
	ABI          string   `json:"abi"`
	Features     []string `json:"features"`
}
type input struct {
	Handle        string `json:"handle"`
	Family        string `json:"family"`
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	ContentBase64 string `json:"content_base64"`
}
type request struct {
	Profile           string  `json:"profile"`
	Family            string  `json:"family"`
	RequestID         string  `json:"request_id"`
	ScopeID           string  `json:"scope_id"`
	CompilationUnitID string  `json:"compilation_unit_id"`
	Target            target  `json:"target"`
	Inputs            []input `json:"inputs"`
}

type manifestTuple struct {
	Family          string `json:"family"`
	Language        string `json:"language"`
	Framework       string `json:"framework"`
	Toolchain       string `json:"toolchain"`
	Version         string `json:"version"`
	CandidateFamily string `json:"candidate_family"`
	InputFamily     string `json:"input_family"`
}
type artifactSelection struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type dogfoodManifest struct {
	Schema    string            `json:"schema"`
	PluginID  string            `json:"plugin_id"`
	ReleaseID string            `json:"release_id"`
	Version   string            `json:"version"`
	Tuple     manifestTuple     `json:"tuple"`
	Artifact  artifactSelection `json:"artifact"`
}
type dogfoodIdentity struct {
	PluginID        string `json:"plugin_id"`
	ReleaseID       string `json:"release_id"`
	Version         string `json:"version"`
	ManifestSHA256  string `json:"manifest_sha256"`
	ArtifactSHA256  string `json:"artifact_sha256"`
	HarnessDigest   string `json:"harness_executable_digest"`
	Family          string `json:"family"`
	Language        string `json:"language"`
	Framework       string `json:"framework"`
	Toolchain       string `json:"toolchain"`
	ToolchainVer    string `json:"toolchain_version"`
	CandidateFamily string `json:"candidate_family"`
	InputFamily     string `json:"input_family"`
}
type inputReceipt struct {
	BlobSHA1      string `json:"blob_sha1"`
	ContentSHA256 string `json:"content_sha256"`
	Bytes         int    `json:"bytes"`
	Family        string `json:"family"`
}
type cleanupReceipt struct {
	ProcessGroup       string `json:"process_group"`
	ExecutionDirectory string `json:"execution_directory"`
	Artifact           string `json:"artifact"`
	RunDirectory       string `json:"run_directory"`
}
type runReceipt struct {
	Identity                 dogfoodIdentity `json:"identity"`
	Kind                     string          `json:"kind"`
	Family                   string          `json:"family"`
	Status                   string          `json:"status"`
	Outcome                  string          `json:"outcome"`
	Reason                   string          `json:"reason"`
	OutputSHA256             string          `json:"output_sha256"`
	OutputBytes              int             `json:"output_bytes"`
	StderrSHA256             string          `json:"stderr_sha256"`
	StderrBytes              int             `json:"stderr_bytes"`
	RequestSHA256            string          `json:"request_sha256"`
	RequestBytes             int             `json:"request_bytes"`
	ResponseBindingSHA256    string          `json:"response_binding_sha256"`
	ExitCode                 int             `json:"exit_code"`
	WallNS                   int64           `json:"wall_ns"`
	UserNS                   int64           `json:"user_ns"`
	SystemNS                 int64           `json:"system_ns"`
	MaxRSSBytes              int64           `json:"max_rss_bytes"`
	MaxRSSState              string          `json:"max_rss_state"`
	ProcessGroup             string          `json:"process_group"`
	ExecutionBinding         string          `json:"execution_binding"`
	ExecutedArtifactSHA256   string          `json:"executed_artifact_sha256"`
	DescriptorIdentityBefore string          `json:"descriptor_identity_before"`
	DescriptorIdentityAfter  string          `json:"descriptor_identity_after"`
	PathIdentityBefore       string          `json:"path_identity_before"`
	PathIdentityAfter        string          `json:"path_identity_after"`
	Cleanup                  cleanupReceipt  `json:"cleanup"`
}
type receipt struct {
	Schema                    string          `json:"schema"`
	Outcome                   string          `json:"outcome"`
	Reason                    string          `json:"reason"`
	Mode                      string          `json:"mode"`
	Support                   string          `json:"support"`
	Launch                    string          `json:"launch"`
	Promotion                 string          `json:"promotion"`
	Network                   string          `json:"network"`
	RepositoryReadDenial      string          `json:"repository_read_denial"`
	ProcessContainment        string          `json:"process_containment"`
	TargetCommit              string          `json:"target_commit"`
	TargetTree                string          `json:"target_tree"`
	WallNS                    int64           `json:"wall_ns"`
	MaxRSSBytes               int64           `json:"max_rss_bytes"`
	MaxRSSState               string          `json:"max_rss_state"`
	Identity                  dogfoodIdentity `json:"identity"`
	Target                    target          `json:"target"`
	Inputs                    []inputReceipt  `json:"inputs"`
	RequestSHA256             string          `json:"request_sha256"`
	RequestBytes              int             `json:"request_bytes"`
	CanonicalPermutationEqual bool            `json:"canonical_permutation_equal"`
	Runs                      []runReceipt    `json:"runs"`
	Cleanup                   cleanupReceipt  `json:"cleanup"`
	ReceiptBindingSHA256      string          `json:"receipt_binding_sha256"`
	Preparation               struct {
		ExecutorBuildNS int64  `json:"executor_build_ns"`
		RequestNS       int64  `json:"request_ns"`
		Claim           string `json:"claim"`
	} `json:"preparation"`
}

// The registry is closed: rows, not atom combinations, are admitted. Empty
// candidate_family means the outer dogfood tuple has no mapping in the frozen
// four-family candidate envelope and therefore receives a NOT_RUN receipt.
var supportedDogfoodTuples = [...]manifestTuple{
	{"go", "go", "stdlib", "go", "1.27.0", "go", "go.source"},
	{"typescript-javascript-react", "typescript-javascript-react", "react", "node", "22.23.2", "javascript-typescript", "js.source"},
	{"html-css", "html-css", "web", "node", "22.23.2", "javascript-typescript", "js.source"},
	{"swift-objc-c-metal", "swift-objective-c-c-metal", "apple-native", "xcode", "16.4", "", ""},
	{"kotlin-java-jni-glsl", "kotlin-java-jni-glsl", "jvm-native", "jdk-21", "21.0.7", "", ""},
	{"sql", "sql", "portable-sql", "sqlite", "3.50.4", "", ""},
	{"shell", "shell", "posix-shell", "bash", "5.2.37", "", ""},
	{"python-tooling", "python", "python-tooling", "python", "3.13.7", "", ""},
	{"structured-web-assets", "structured-web-assets", "web", "node", "22.23.2", "javascript-typescript", "js.source"},
	{"ruby", "ruby", "bundler", "ruby", "2.6.10", "ruby", "ruby.source"},
	{"dotnet", "dotnet", "msbuild", "dotnet-sdk", "8.0.423", "dotnet", "dotnet.project"},
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer stop()
	if err := runContext(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "beamfall-shadow:", err)
		os.Exit(2)
	}
}
func run(args []string, out io.Writer) error { return runContext(context.Background(), args, out) }

func runContext(ctx context.Context, args []string, out io.Writer) (err error) {
	started := time.Now()
	var o options
	f := flag.NewFlagSet("beamfall-shadow", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&o.targetRepo, "target-repo", "", "target repository")
	f.StringVar(&o.targetRev, "target-rev", "", "immutable target commit")
	f.StringVar(&o.manifest, "plugin-manifest", "", "canonical dogfood manifest")
	f.Var(&o.inputs, "input", "logical input path (repeat)")
	f.StringVar(&o.targetOS, "target-os", "darwin", "target OS")
	f.StringVar(&o.targetArch, "target-arch", "arm64", "target architecture")
	f.StringVar(&o.targetABI, "target-abi", "none", "target ABI")
	f.StringVar(&o.receipt, "receipt", "", "required private receipt")
	f.IntVar(&o.runs, "runs", 5, "fresh executions")
	f.DurationVar(&o.timeout, "timeout", 5*time.Second, "per-child timeout")
	if err = f.Parse(args); err != nil {
		return err
	}
	r := newReceipt()
	writeReceipt := filepath.IsAbs(o.receipt)
	if writeReceipt {
		defer func() {
			r.WallNS = time.Since(started).Nanoseconds()
			for _, run := range r.Runs {
				if run.MaxRSSState == "OBSERVED" && (r.MaxRSSState != "OBSERVED" || run.MaxRSSBytes > r.MaxRSSBytes) {
					r.MaxRSSBytes, r.MaxRSSState = run.MaxRSSBytes, "OBSERVED"
				}
			}
			if err == nil {
				r.Outcome, r.Reason = "OBSERVED", "NONE"
			} else {
				r.Outcome, r.Reason = "FAILED", receiptReason(err)
			}
			r.ReceiptBindingSHA256 = receiptBinding(r)
			raw, marshalErr := json.Marshal(r)
			if marshalErr != nil {
				err = errors.Join(err, marshalErr)
				return
			}
			receiptCtx, receiptDone := cleanupCtx()
			defer receiptDone()
			if writeErr := writePrivate(receiptCtx, o.receipt, append(raw, '\n')); writeErr != nil {
				err = errors.Join(err, fmt.Errorf("write receipt: %w", writeErr))
			}
		}()
	}
	if f.NArg() != 0 || o.manifest == "" || o.targetRepo == "" || o.targetRev == "" || o.receipt == "" || len(o.inputs) < 2 {
		return errors.New("require --plugin-manifest --target-repo --target-rev --receipt and at least two --input")
	}
	if !filepath.IsAbs(o.receipt) {
		return errors.New("receipt path must be absolute")
	}
	if o.runs != 5 || o.timeout <= 0 {
		return errors.New("invalid experimental run limits")
	}
	for _, p := range o.inputs {
		if !logicalPath(p) {
			return errors.New("invalid logical path")
		}
	}
	m, manifestBytes, err := readDogfoodManifest(ctx, o.manifest)
	if err != nil {
		return err
	}
	harness, err := harnessExecutableDigest(ctx)
	if err != nil {
		return err
	}
	r.Identity = m.identity(manifestBytes, harness)
	git, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	r.TargetCommit, err = gitText(ctx, git, o.targetRepo, "rev-parse", o.targetRev+"^{commit}")
	if err != nil {
		return err
	}
	r.TargetTree, err = gitText(ctx, git, o.targetRepo, "rev-parse", r.TargetCommit+"^{tree}")
	if err != nil {
		return err
	}
	r.Target = target{OS: o.targetOS, Architecture: o.targetArch, ABI: o.targetABI, Features: []string{}}
	if m.Tuple.CandidateFamily == "" {
		return errors.New("UNMAPPED_FROZEN_ENVELOPE")
	}
	runDir, err := os.MkdirTemp("", "corvint-beamfall-shadow-")
	if err != nil {
		return err
	}
	if err = os.Chmod(runDir, 0o700); err != nil {
		return errors.Join(err, os.RemoveAll(runDir))
	}
	defer func() {
		cleanCtx, cleanDone := cleanupCtx()
		e := removeAllContext(cleanCtx, runDir)
		cleanDone()
		r.Cleanup.RunDirectory = cleanupState(e, "REMOVED")
		if e != nil {
			err = errors.Join(err, fmt.Errorf("cleanup run directory: %w", e))
		}
	}()
	goTool, err := exec.LookPath("go")
	if err != nil {
		return err
	}
	executorStart := time.Now()
	executor := ""
	if descriptorExecutorAvailable() {
		executor, err = buildFDExecutor(ctx, goTool, runDir)
		if err != nil {
			return err
		}
	}
	r.Preparation.ExecutorBuildNS = time.Since(executorStart).Nanoseconds()
	guard, err := stageExternalArtifact(ctx, m, filepath.Join(runDir, "candidate"))
	if err != nil {
		return err
	}
	defer func() {
		e := guard.Close()
		r.Cleanup.Artifact = cleanupState(e, "CLOSED")
		if e != nil {
			err = errors.Join(err, fmt.Errorf("cleanup artifact: %w", e))
		}
	}()
	requestStart := time.Now()
	inputs, inputsReceipt, err := loadInputs(ctx, git, o.targetRepo, r.TargetCommit, o.inputs, m.Tuple.InputFamily)
	if err != nil {
		return err
	}
	base, err := canonicalRequest(r.Target, r.TargetCommit, m.Tuple.CandidateFamily, inputs)
	if err != nil {
		return err
	}
	if len(base) > maxRequestBytes {
		return errors.New("request limit exceeded")
	}
	r.Inputs, r.RequestSHA256, r.RequestBytes, r.Preparation.RequestNS, r.Preparation.Claim = inputsReceipt, digest(base), len(base), time.Since(requestStart).Nanoseconds(), "DIAGNOSTIC_TIMINGS_ONLY_NO_LATENCY_CLAIM"
	reversed := append([]input(nil), inputs...)
	reverse(reversed)
	permuted, err := canonicalRequest(r.Target, r.TargetCommit, m.Tuple.CandidateFamily, reversed)
	if err != nil {
		return err
	}
	if !bytes.Equal(base, permuted) {
		return errors.New("caller canonicalization is permutation-sensitive")
	}
	r.CanonicalPermutationEqual = true
	for range 5 {
		one := invoke(ctx, executor, runDir, base, o.timeout, "canonical", guard, r.Identity)
		r.Runs = append(r.Runs, one)
		if one.Status != "CANDIDATE" {
			return fmt.Errorf("canonical run did not return candidate: %s", one.Status)
		}
	}
	if !sameOutputs(r.Runs[:5]) {
		return errors.New("fresh canonical outputs differ")
	}
	rawReverse, err := rawRequest(r.Target, r.TargetCommit, m.Tuple.CandidateFamily, reversed)
	if err != nil {
		return err
	}
	one := invoke(ctx, executor, runDir, rawReverse, o.timeout, "raw_reverse", guard, r.Identity)
	r.Runs = append(r.Runs, one)
	if one.Status != "REJECTED_DUPLICATE_VALUE" {
		return errors.New("raw reverse did not reject ordering")
	}
	bad := bytes.Replace(append([]byte(nil), base...), []byte(inputs[0].SHA256), []byte("sha256:"+strings.Repeat("0", 64)), 1)
	one = invoke(ctx, executor, runDir, bad, o.timeout, "digest_mismatch", guard, r.Identity)
	r.Runs = append(r.Runs, one)
	if one.Status != "REJECTED_DIGEST_MISMATCH" {
		return errors.New("digest mismatch did not reject")
	}
	_ = out
	return nil
}

func newReceipt() receipt {
	return receipt{Schema: "beamfall-shadow-dogfood/2", Mode: "SHADOW_DOGFOOD", Support: "NO_SUPPORT", Launch: "NO_CORVINT_CORE_LAUNCH", Promotion: "NO_ADMISSION_NO_EXACT_VERIFIER_NO_AUTHORITY", Network: "NOT_OBSERVED", RepositoryReadDenial: "NOT_OBSERVED", ProcessContainment: processContainmentLabel(), MaxRSSState: "NOT_OBSERVED", Cleanup: cleanupReceipt{ProcessGroup: "NOT_RUN", ExecutionDirectory: "NOT_RUN", Artifact: "NOT_RUN", RunDirectory: "NOT_RUN"}, Identity: dogfoodIdentity{PluginID: "NOT_AVAILABLE", ReleaseID: "NOT_AVAILABLE", Version: "NOT_AVAILABLE", ManifestSHA256: "NOT_AVAILABLE", ArtifactSHA256: "NOT_AVAILABLE", HarnessDigest: "NOT_AVAILABLE"}}
}
func receiptReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "TIMEOUT"
	}
	if errors.Is(err, context.Canceled) {
		return "CANCELED"
	}
	if strings.Contains(err.Error(), "UNMAPPED_FROZEN_ENVELOPE") {
		return "UNMAPPED_FROZEN_ENVELOPE"
	}
	return "FAILED"
}
func (m dogfoodManifest) identity(raw []byte, harness string) dogfoodIdentity {
	return dogfoodIdentity{m.PluginID, m.ReleaseID, m.Version, digest(raw), m.Artifact.SHA256, harness, m.Tuple.Family, m.Tuple.Language, m.Tuple.Framework, m.Tuple.Toolchain, m.Tuple.Version, m.Tuple.CandidateFamily, m.Tuple.InputFamily}
}

func readDogfoodManifest(ctx context.Context, name string) (dogfoodManifest, []byte, error) {
	if !filepath.IsAbs(name) {
		return dogfoodManifest{}, nil, errors.New("plugin manifest path must be absolute")
	}
	f, info, err := openRegularNoFollow(name, maxManifestBytes)
	if err != nil || info.Size() < 2 {
		return dogfoodManifest{}, nil, errors.New("plugin manifest is not a bounded regular file")
	}
	raw, readErr := readBoundedContext(ctx, f, maxManifestBytes)
	closeErr := f.Close()
	if readErr != nil || closeErr != nil {
		return dogfoodManifest{}, nil, errors.Join(readErr, closeErr)
	}
	var m dogfoodManifest
	if stdjson.Unmarshal(raw, &m) != nil {
		return dogfoodManifest{}, nil, errors.New("invalid plugin manifest")
	}
	canonical, marshalErr := json.Marshal(m)
	if marshalErr != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return dogfoodManifest{}, nil, errors.New("noncanonical plugin manifest")
	}
	if m.Schema != "beamfall-shadow-dogfood/2" || !candidateIdentifier(m.PluginID) || !candidateIdentifier(m.ReleaseID) || !candidateIdentifier(m.Version) || !filepath.IsAbs(m.Artifact.Path) || !validDigest(m.Artifact.SHA256) || !registeredTuple(m.Tuple) {
		return dogfoodManifest{}, nil, errors.New("unknown or incomplete dogfood tuple")
	}
	return m, raw, nil
}
func registeredTuple(tuple manifestTuple) bool {
	for _, row := range supportedDogfoodTuples {
		if tuple == row {
			return true
		}
	}
	return false
}

type artifactGuard struct {
	directory, artifact         string
	directoryInfo, artifactInfo os.FileInfo
	digest                      string
	file                        *os.File
	beforeExecution             func()
}

// Stage exactly once: descriptor-bound nofollow source -> hash/copy -> retained staged descriptor.
func stageExternalArtifact(ctx context.Context, m dogfoodManifest, directory string) (artifactGuard, error) {
	source, info, err := openRegularNoFollow(m.Artifact.Path, maxArtifactBytes)
	if err != nil || info.Size() == 0 || !soleArtifactLink(info) {
		return artifactGuard{}, errors.New("external artifact is not a bounded unlinked regular file")
	}
	if err = os.MkdirAll(directory, 0o700); err != nil {
		return artifactGuard{}, errors.Join(err, source.Close())
	}
	staged := filepath.Join(directory, "plugin")
	dst, err := os.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o700)
	if err != nil {
		return artifactGuard{}, closeAndRemove(source, directory, err)
	}
	got, written, copyErr := copyHashContext(ctx, dst, source, maxArtifactBytes)
	sourceClose := source.Close()
	if copyErr != nil || sourceClose != nil || written != info.Size() || got != m.Artifact.SHA256 {
		return artifactGuard{}, closeAndRemove(dst, directory, errors.Join(copyErr, sourceClose, artifactCopyError(written, info.Size(), got, m.Artifact.SHA256)))
	}
	if err = dst.Sync(); err != nil {
		return artifactGuard{}, closeAndRemove(dst, directory, err)
	}
	if err = dst.Chmod(0o500); err != nil {
		return artifactGuard{}, closeAndRemove(dst, directory, err)
	}
	stagedInfo, statErr := dst.Stat()
	if statErr != nil || !stagedInfo.Mode().IsRegular() || stagedInfo.Size() != written {
		return artifactGuard{}, closeAndRemove(dst, directory, errors.Join(statErr, errors.New("staged artifact identity mismatch")))
	}
	if err = sealArtifact(directory, staged); err != nil {
		return artifactGuard{}, closeAndRemove(dst, directory, err)
	}
	dirInfo, dirErr := os.Lstat(directory)
	if dirErr != nil || !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 || dirInfo.Mode().Perm() != 0o500 {
		return artifactGuard{}, closeAndRemove(dst, directory, errors.Join(dirErr, errors.New("staged artifact directory is not sealed")))
	}
	// The retained descriptor must carry no write authority: close the copy
	// descriptor, reopen the same inode read-only nofollow, and require identity.
	if err = dst.Close(); err != nil {
		return artifactGuard{}, errors.Join(err, os.RemoveAll(directory))
	}
	reopened, reopenedInfo, err := openRegularNoFollow(staged, maxArtifactBytes)
	if err != nil {
		return artifactGuard{}, errors.Join(err, os.RemoveAll(directory))
	}
	if !os.SameFile(stagedInfo, reopenedInfo) || reopenedInfo.Size() != written || !soleArtifactLink(reopenedInfo) {
		return artifactGuard{}, closeAndRemove(reopened, directory, errors.New("staged artifact identity changed across read-only reopen"))
	}
	return artifactGuard{directory, staged, dirInfo, reopenedInfo, got, reopened, nil}, nil
}
func artifactCopyError(want, got int64, actual, expected string) error {
	if want == got && actual == expected {
		return nil
	}
	return errors.New("external artifact copy drift")
}
func closeAndRemove(f *os.File, directory string, err error) error {
	var closeErr error
	if f != nil {
		closeErr = f.Close()
	}
	return errors.Join(err, closeErr, os.RemoveAll(directory))
}
func soleArtifactLink(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return !ok || stat.Nlink == 1
}
func sealArtifact(directory, artifact string) error {
	if err := os.Chmod(artifact, 0o500); err != nil {
		return err
	}
	if err := os.Chmod(directory, 0o500); err != nil {
		return err
	}
	return sealImmutable(directory, artifact)
}
func (g artifactGuard) descriptorUnchanged() error {
	info, err := g.file.Stat()
	if err != nil || !os.SameFile(g.artifactInfo, info) || info.Mode() != g.artifactInfo.Mode() || info.Size() != g.artifactInfo.Size() || info.ModTime() != g.artifactInfo.ModTime() {
		return errors.New("candidate artifact descriptor identity changed")
	}
	return nil
}
func (g artifactGuard) pathUnchanged() error {
	dir, err := os.Lstat(g.directory)
	if err != nil || !os.SameFile(g.directoryInfo, dir) || dir.Mode() != g.directoryInfo.Mode() || dir.ModTime() != g.directoryInfo.ModTime() {
		return errors.New("candidate artifact directory identity changed")
	}
	file, err := os.Lstat(g.artifact)
	if err != nil || !os.SameFile(g.artifactInfo, file) || file.Mode() != g.artifactInfo.Mode() || file.Size() != g.artifactInfo.Size() || file.ModTime() != g.artifactInfo.ModTime() {
		return errors.New("candidate artifact identity changed")
	}
	return nil
}
func (g artifactGuard) Close() error {
	// Restoring directory write permission is part of observable cleanup: the
	// parent cannot remove the sealed staged file otherwise.
	return errors.Join(unsealImmutable(g.directory, g.artifact), g.file.Close(), os.Chmod(g.directory, 0o700))
}

// harnessExecutableDigest hashes THIS harness executable. It is not a Corvint
// Core identity: the harness never launches Core and makes no running-Core claim.
func harnessExecutableDigest(ctx context.Context) (string, error) {
	name, err := os.Executable()
	if err != nil {
		return "", err
	}
	f, _, err := openRegularNoFollow(name, maxCoreBytes)
	if err != nil {
		return "", err
	}
	sum, _, hashErr := copyHashContext(ctx, io.Discard, f, maxCoreBytes)
	return sum, errors.Join(hashErr, f.Close())
}

func gitText(ctx context.Context, git, repo string, args ...string) (string, error) {
	raw, err := gitLimitedBytes(ctx, maxArtifactBytes, git, repo, args...)
	return strings.TrimSpace(string(raw)), err
}
func gitLimitedBytes(ctx context.Context, limit int, git, repo string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, git, append([]string{"-C", repo}, args...)...)
	c.Env = []string{"LC_ALL=C", "LANG=C", "PATH=/usr/bin:/bin"}
	var stdout, stderr boundedBuffer
	stdout.limit = limit
	stderr.limit = 64 << 10
	c.Stdout, c.Stderr = &stdout, &stderr
	err := c.Run()
	if stdout.exceeded {
		return nil, errors.New("git output limit exceeded")
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("git command failed")
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}
func loadInputs(ctx context.Context, git, repo, rev string, paths []string, fam string) ([]input, []inputReceipt, error) {
	if !candidateIdentifier(fam) || len(paths) > maxInputs {
		return nil, nil, errors.New("manifest must declare one canonical input family")
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	inputs := make([]input, 0, len(sorted))
	records := make([]inputReceipt, 0, len(sorted))
	total := 0
	for i, p := range sorted {
		if i > 0 && p == sorted[i-1] {
			return nil, nil, errors.New("duplicate input")
		}
		blob, err := gitText(ctx, git, repo, "rev-parse", rev+":"+p)
		if err != nil {
			return nil, nil, err
		}
		sizeText, err := gitText(ctx, git, repo, "cat-file", "-s", rev+":"+p)
		if err != nil {
			return nil, nil, err
		}
		var size int
		if _, err = fmt.Sscan(sizeText, &size); err != nil || size < 0 || size > maxInputBytes || size > maxAggregateInputBytes-total {
			return nil, nil, errors.New("input blob byte limit exceeded")
		}
		body, err := gitLimitedBytes(ctx, size, git, repo, "show", rev+":"+p)
		if err != nil || len(body) != size {
			return nil, nil, errors.Join(err, errors.New("input blob changed while reading"))
		}
		total += len(body)
		sum := digest(body)
		inputs = append(inputs, input{fmt.Sprintf("input-%03d", i+1), fam, p, sum, base64.StdEncoding.EncodeToString(body)})
		records = append(records, inputReceipt{blob, sum, len(body), fam})
	}
	return inputs, records, nil
}
func canonicalRequest(t target, commit, family string, inputs []input) ([]byte, error) {
	cp := append([]input(nil), inputs...)
	sort.Slice(cp, func(i, j int) bool { return compareInput(cp[i], cp[j]) < 0 })
	return rawRequest(t, commit, family, cp)
}
func rawRequest(t target, commit, family string, inputs []input) ([]byte, error) {
	if len(commit) < 12 || !frozenCandidateFamily(family) {
		return nil, errors.New("invalid frozen candidate family")
	}
	raw, err := json.Marshal(request{profile, family, "shadow-" + commit[:12], "target-" + commit, "unit-" + commit[:12], t, inputs})
	return append(raw, '\n'), err
}
func frozenCandidateFamily(value string) bool {
	switch value {
	case "go", "javascript-typescript", "dotnet", "ruby":
		return true
	}
	return false
}
func compareInput(a, b input) int {
	for _, p := range [][2]string{{a.Handle, b.Handle}, {a.Family, b.Family}, {a.Path, b.Path}, {a.SHA256, b.SHA256}} {
		if p[0] < p[1] {
			return -1
		}
		if p[0] > p[1] {
			return 1
		}
	}
	return 0
}
func reverse[T any](v []T) {
	for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
}
func logicalPath(p string) bool {
	if p == "" || len(p) > 4096 || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\:\x00") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func invoke(parent context.Context, executor, runDir string, raw []byte, timeout time.Duration, kind string, guard artifactGuard, id dogfoodIdentity) runReceipt {
	source, ok := candidateSourceRequest(raw)
	family := "NOT_AVAILABLE"
	if ok {
		family = source.Family
	}
	r := runReceipt{Identity: id, Kind: kind, Family: family, RequestSHA256: digest(raw), RequestBytes: len(raw), OutputSHA256: digest(nil), StderrSHA256: digest(nil), MaxRSSState: "NOT_OBSERVED", ProcessGroup: "NOT_RUN", ExecutionBinding: "NOT_RUN", ExecutedArtifactSHA256: "NOT_RUN", DescriptorIdentityBefore: "NOT_RUN", DescriptorIdentityAfter: "NOT_RUN", PathIdentityBefore: "NOT_RUN", PathIdentityAfter: "NOT_RUN", Cleanup: cleanupReceipt{"NOT_RUN", "NOT_RUN", "HELD", "PENDING"}}
	if guard.descriptorUnchanged() != nil {
		r.Status, r.Reason = "ARTIFACT_DESCRIPTOR_CHANGED", "ARTIFACT_DESCRIPTOR_CHANGED"
		return boundRun(r)
	}
	r.DescriptorIdentityBefore = "MATCHED"
	if guard.pathUnchanged() != nil {
		r.Status, r.Reason = "ARTIFACT_PATH_CHANGED", "ARTIFACT_PATH_CHANGED"
		return boundRun(r)
	}
	r.PathIdentityBefore = "MATCHED"
	if executor == "" {
		r.Status, r.Reason, r.ExecutionBinding = "UNSUPPORTED_EXECUTION_AUTHORITY", "UNSUPPORTED_EXECUTION_AUTHORITY", "UNSUPPORTED"
		return boundRun(r)
	}
	if establishExecutionAuthority() != nil {
		r.Status, r.Reason, r.ExecutionBinding = "UNSUPPORTED_EXECUTION_AUTHORITY", "UNSUPPORTED_EXECUTION_AUTHORITY", "UNSUPPORTED"
		return boundRun(r)
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cwd, err := os.MkdirTemp(runDir, "cwd-")
	if err != nil {
		r.Status, r.Reason = "EXECUTION_DIRECTORY_FAILURE", "EXECUTION_DIRECTORY_FAILURE"
		return boundRun(r)
	}
	if err = os.Chmod(cwd, 0o700); err != nil {
		cleanCtx, cleanDone := cleanupCtx()
		e := removeAllContext(cleanCtx, cwd)
		cleanDone()
		r.Status, r.Reason, r.Cleanup.ExecutionDirectory = "EXECUTION_DIRECTORY_FAILURE", "EXECUTION_DIRECTORY_FAILURE", cleanupState(e, "REMOVED")
		return boundRun(r)
	}
	c := exec.CommandContext(ctx, executor)
	c.ExtraFiles = []*os.File{guard.file}
	c.Dir, c.Env, c.Stdin = cwd, []string{}, bytes.NewReader(raw)
	configureProcess(c)
	var lock sync.Mutex
	var killErr error
	stop := func() {
		if c.Process == nil {
			return
		}
		e := syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
		if e != nil && !errors.Is(e, syscall.ESRCH) {
			lock.Lock()
			killErr = errors.Join(killErr, e)
			lock.Unlock()
		}
	}
	var stdout, stderr boundedBuffer
	stdout.limit, stderr.limit = maxInputBytes, maxInputBytes
	stdout.onExceeded, stderr.onExceeded = stop, stop
	c.Stdout, c.Stderr = &stdout, &stderr
	if guard.beforeExecution != nil {
		guard.beforeExecution()
	}
	started := time.Now()
	runErr := c.Run()
	r.WallNS, r.OutputSHA256, r.OutputBytes = time.Since(started).Nanoseconds(), digest(stdout.Bytes()), stdout.Len()
	r.StderrSHA256, r.StderrBytes = digest(stderr.Bytes()), stderr.Len()
	r.ExecutionBinding, r.ExecutedArtifactSHA256 = "DESCRIPTOR_FD_3_"+processContainmentLabel(), guard.digest
	r.ProcessGroup, err = processGroupState(c.Process)
	if err != nil {
		r.Status, r.Reason, r.Cleanup.ProcessGroup = "CLEANUP_ERROR", "PROCESS_GROUP_STATE", "ERROR"
	} else if r.ProcessGroup != "GROUP_EMPTY_OBSERVED" {
		state, e := cleanupProcessGroup(c.Process)
		r.ProcessGroup, r.Cleanup.ProcessGroup = state, state
		if e != nil {
			r.Status, r.Reason = "CLEANUP_ERROR", "PROCESS_GROUP_CLEANUP"
		} else {
			r.Status, r.Reason = "PROCESS_GROUP_RESIDUE", "PROCESS_GROUP_RESIDUE"
		}
	} else {
		r.Cleanup.ProcessGroup = "GROUP_EMPTY_OBSERVED"
	}
	if r.Status == "" {
		if guard.descriptorUnchanged() != nil {
			r.Status, r.Reason, r.DescriptorIdentityAfter = "ARTIFACT_DESCRIPTOR_CHANGED", "ARTIFACT_DESCRIPTOR_CHANGED", "CHANGED"
		} else {
			r.DescriptorIdentityAfter = "MATCHED"
		}
	}
	if r.Status == "" {
		if guard.pathUnchanged() != nil {
			r.Status, r.Reason, r.PathIdentityAfter = "ARTIFACT_PATH_CHANGED", "ARTIFACT_PATH_CHANGED", "CHANGED"
		} else {
			r.PathIdentityAfter = "MATCHED"
		}
	}
	if c.ProcessState != nil {
		r.ExitCode = c.ProcessState.ExitCode()
		if usage, ok := c.ProcessState.SysUsage().(*syscall.Rusage); ok {
			r.UserNS = usage.Utime.Sec*1e9 + int64(usage.Utime.Usec)*1e3
			r.SystemNS = usage.Stime.Sec*1e9 + int64(usage.Stime.Usec)*1e3
			r.MaxRSSBytes = int64(usage.Maxrss)
			if runtime.GOOS != "darwin" {
				r.MaxRSSBytes *= 1024
			}
			r.MaxRSSState = "OBSERVED"
		}
	}
	lock.Lock()
	kill := killErr
	lock.Unlock()
	if r.Status == "" && kill != nil {
		r.Status, r.Reason = "CLEANUP_ERROR", "OUTPUT_LIMIT_KILL"
	}
	if r.Status == "" && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		r.Status, r.Reason = "TIMEOUT", "TIMEOUT"
	}
	if r.Status == "" && errors.Is(ctx.Err(), context.Canceled) {
		r.Status, r.Reason = "CANCELED", "CANCELED"
	}
	if r.Status == "" && (stdout.exceeded || stderr.exceeded) {
		r.Status, r.Reason = "NONCANONICAL_OR_OVERSIZE_OUTPUT", "OUTPUT_LIMIT"
	}
	if r.Status == "" && runErr != nil {
		r.Status, r.Reason = "PROCESS_ERROR", "PROCESS_ERROR"
	}
	if r.Status == "" {
		if status, ok := decodeCandidateOutput(stdout.Bytes(), raw); ok {
			r.Status, r.Reason = status, status
		} else {
			r.Status, r.Reason = "UNEXPECTED_OUTPUT", "UNEXPECTED_OUTPUT"
		}
	}
	cleanCtx, cleanDone := cleanupCtx()
	e := removeAllContext(cleanCtx, cwd)
	cleanDone()
	r.Cleanup.ExecutionDirectory = cleanupState(e, "REMOVED")
	if e != nil && r.Status != "CLEANUP_ERROR" {
		r.Status, r.Reason = "CLEANUP_ERROR", "EXECUTION_DIRECTORY_CLEANUP"
	}
	return boundRun(r)
}
func boundRun(r runReceipt) runReceipt {
	r.Outcome = r.Status
	r.ResponseBindingSHA256 = responseBinding(r.Identity, r.RequestSHA256, r.OutputSHA256, r.StderrSHA256, r.Kind, r.Status)
	return r
}
func responseBinding(id dogfoodIdentity, request, output, stderr, kind, status string) string {
	raw, err := json.Marshal(struct {
		Identity dogfoodIdentity `json:"identity"`
		Request  string          `json:"request"`
		Output   string          `json:"output"`
		Stderr   string          `json:"stderr"`
		Kind     string          `json:"kind"`
		Status   string          `json:"status"`
	}{id, request, output, stderr, kind, status})
	if err != nil {
		return "NOT_AVAILABLE"
	}
	return digest(raw)
}

// receiptBinding binds the BSD-003 fields only. Each run contributes its response
// binding and bounded result; wall/RSS measurements and descriptor identities are
// BSD-005 observations that differ on every execution, so they stay out of it.
func receiptBinding(r receipt) string {
	type boundRunResult struct {
		Response string `json:"response"`
		Outcome  string `json:"outcome"`
		Reason   string `json:"reason"`
	}
	runs := make([]boundRunResult, 0, len(r.Runs))
	for _, run := range r.Runs {
		runs = append(runs, boundRunResult{run.ResponseBindingSHA256, run.Outcome, run.Reason})
	}
	raw, err := json.Marshal(struct {
		Identity dogfoodIdentity  `json:"identity"`
		Request  string           `json:"request"`
		Outcome  string           `json:"outcome"`
		Reason   string           `json:"reason"`
		Runs     []boundRunResult `json:"runs"`
	}{r.Identity, r.RequestSHA256, r.Outcome, r.Reason, runs})
	if err != nil {
		return "NOT_AVAILABLE"
	}
	return digest(raw)
}
func sameOutputs(v []runReceipt) bool {
	if len(v) == 0 {
		return false
	}
	for _, r := range v[1:] {
		if r.OutputSHA256 != v[0].OutputSHA256 || r.OutputBytes != v[0].OutputBytes || r.Status != v[0].Status {
			return false
		}
	}
	return true
}

type boundedBuffer struct {
	buf        bytes.Buffer
	limit      int
	exceeded   bool
	onExceeded func()
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			b.exceeded = true
			_, _ = b.buf.Write(p[:remaining])
		} else {
			_, _ = b.buf.Write(p)
		}
	} else {
		b.exceeded = true
	}
	if b.exceeded {
		if b.onExceeded != nil {
			b.onExceeded()
		}
		return 0, errors.New("bounded output exceeded")
	}
	return len(p), nil
}
func (b *boundedBuffer) Bytes() []byte { return b.buf.Bytes() }
func (b *boundedBuffer) Len() int      { return b.buf.Len() }
func digest(b []byte) string           { sum := sha256.Sum256(b); return "sha256:" + fmt.Sprintf("%x", sum[:]) }
func readBoundedContext(ctx context.Context, source io.Reader, limit int64) ([]byte, error) {
	var out bytes.Buffer
	if limit > 0 && limit < 1<<20 {
		out.Grow(int(limit))
	}
	_, _, err := copyHashContext(ctx, &out, source, limit)
	return out.Bytes(), err
}
func copyHashContext(ctx context.Context, destination io.Writer, source io.Reader, limit int64) (string, int64, error) {
	if limit < 0 {
		return "", 0, errors.New("negative copy limit")
	}
	var closeDone chan struct{}
	var cancelClose chan error
	if closer, ok := source.(io.Closer); ok {
		closeDone = make(chan struct{})
		cancelClose = make(chan error, 1)
		go func() {
			select {
			case <-ctx.Done():
				cancelClose <- closer.Close()
			case <-closeDone:
			}
		}()
		defer close(closeDone)
	}
	buffer := make([]byte, ioChunkBytes)
	h := sha256.New()
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return "", total, err
		}
		n, readErr := source.Read(buffer)
		if contextErr := ctx.Err(); contextErr != nil {
			if cancelClose != nil {
				return "", total, errors.Join(contextErr, <-cancelClose)
			}
			return "", total, contextErr
		}
		if n > 0 {
			total += int64(n)
			if total > limit {
				return "", total, errors.New("bounded copy limit exceeded")
			}
			if _, err := h.Write(buffer[:n]); err != nil {
				return "", total, err
			}
			for written := 0; written < n; {
				if err := ctx.Err(); err != nil {
					return "", total, err
				}
				m, writeErr := destination.Write(buffer[written:n])
				written += m
				if writeErr != nil {
					return "", total, writeErr
				}
				if m == 0 {
					return "", total, io.ErrShortWrite
				}
			}
		}
		if readErr == io.EOF {
			return "sha256:" + fmt.Sprintf("%x", h.Sum(nil)), total, nil
		}
		if readErr != nil {
			return "", total, readErr
		}
		if n == 0 {
			return "", total, io.ErrNoProgress
		}
	}
}
func writePrivate(ctx context.Context, name string, data []byte) error {
	dir := filepath.Dir(name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return errors.New("receipt directory must be private")
	}
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err = f.Chmod(0o600); err != nil {
		return errors.Join(err, f.Close())
	}
	_, _, writeErr := copyHashContext(ctx, f, bytes.NewReader(data), int64(len(data)))
	return errors.Join(writeErr, f.Close())
}
func cleanupState(err error, ok string) string {
	if err != nil {
		return "ERROR"
	}
	return ok
}

// cleanupCtx returns a bounded context independent of any expired or canceled
// execution context, so mandatory cleanup and the terminal receipt always run.
func cleanupCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func removeAllContext(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.RemoveAll(name)
}
func buildFDExecutor(ctx context.Context, goTool, runDir string) (string, error) {
	root, err := harnessRoot()
	if err != nil {
		return "", err
	}
	path := filepath.Join(runDir, "fdexec")
	cache := filepath.Join(runDir, "fdexec-cache")
	for _, d := range []string{filepath.Join(cache, "home"), filepath.Join(cache, "tmp"), filepath.Join(cache, "build"), filepath.Join(cache, "mod")} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return "", err
		}
	}
	c := exec.CommandContext(ctx, goTool, "build", "-trimpath", "-buildvcs=false", "-o", path, "./tools/beamfall-shadow/fdexec")
	c.Dir = root
	c.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + filepath.Join(cache, "home"), "TMPDIR=" + filepath.Join(cache, "tmp"), "GOCACHE=" + filepath.Join(cache, "build"), "GOMODCACHE=" + filepath.Join(cache, "mod"), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOVCS=*:off"}
	var output boundedBuffer
	output.limit = 64 << 10
	c.Stdout, c.Stderr = &output, &output
	if err := c.Run(); err != nil || output.exceeded {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("descriptor executor build failed")
	}
	if err := os.Chmod(path, 0o500); err != nil {
		return "", err
	}
	return path, nil
}
func harnessRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		module, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && bytes.Contains(module, []byte("module github.com/Beamfall/corvint\n")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("shadow harness module root unavailable")
		}
		dir = parent
	}
}
