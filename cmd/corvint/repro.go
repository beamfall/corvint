package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/repro"
)

const reproHelp = `Record a corvint invocation as a reproduction bundle, or replay one (RPB-V0, experimental).

Usage:
  corvint [--root PATH] repro record --output FILE [--input PATH ...]
    [--expect-exit N] [--expect-stdout-contains TEXT ...]
    [--expect-stdout-not-contains TEXT ...] [--expect-stderr-contains TEXT ...]
    [--expect-same-stdout] -- ARGUMENT ...
  corvint [--root PATH] repro run BUNDLE

record runs "corvint --root PATH ARGUMENT ..." in this process with empty
stdin and writes one corvint-repro/0 JSON bundle to the new file FILE: the
recording build's --version --json identity, the arguments, the repository
pinned by HEAD commit and tree (the tracked tree must be clean), each --input
file pinned by SHA-256, the exit code, the stdout digest and bounded stdout
and stderr, and the expectations. Without an --expect flag the expectations
are the recorded exit code and identical stdout. record exits 0 whatever the
recorded command returned. The bundle is a plain file to attach to an issue.

run replays the bundle against this build in the same repository. It refuses,
running nothing, when a pinned input is unavailable: HEAD is not the pinned
commit, the tracked tree is dirty, or an input file is missing or changed. It
prints corvint-repro-run/0: PASS or FAIL per expectation, this build's
identity, the recording build's identity and whether they are the same build.
Exit 0 when every expectation passes, 1 when one fails.
A replay runs the recorded command, so it writes whatever that command writes.

Refusals (exit 2): repro-invalid-arguments, repro-repository-unavailable,
repro-inputs-unpinned, repro-output-exists, repro-output-failed,
repro-bundle-unavailable, repro-bundle-invalid, repro-inputs-unavailable.
Contract: docs/specs/reproduction-bundle-v0.md.

Example:
  corvint repro record --output impact.repro.json --expect-exit 0 -- impact --path README.md
`

const (
	reproGitTimeout   = 30 * time.Second
	reproMaxInputSize = 16 << 20
)

func reproInvocation(args []string) (string, []string, bool) {
	root := ""
	for len(args) > 0 {
		if args[0] == "--root" {
			if !rootPreambleValue(args, 1) {
				return "", nil, false
			}
			root = args[1]
			args = args[2:]
			continue
		}
		if strings.HasPrefix(args[0], "--root=") {
			root = strings.TrimPrefix(args[0], "--root=")
			args = args[1:]
			continue
		}
		break
	}
	if len(args) == 0 || args[0] != "repro" {
		return "", nil, false
	}
	return root, args[1:], true
}

func reproRefusal(code, message string) error {
	return &gokernel.Error{Code: code, Message: message}
}

func runRepro(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if root == "" {
		root = "."
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		emitError(stderr, reproRefusal("repro-repository-unavailable", "cannot resolve --root"))
		return 2
	}
	var payload map[string]any
	exit := 0
	switch {
	case len(args) > 0 && args[0] == "record":
		payload, err = reproRecord(ctx, absolute, args[1:])
	case len(args) > 0 && args[0] == "run":
		payload, exit, err = reproRun(ctx, absolute, args[1:])
	default:
		err = reproRefusal("repro-invalid-arguments", "repro needs record or run (corvint repro --help)")
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if code := emitPayload(stdout, stderr, "repro", reproPlain(payload).(map[string]any)); code != 0 {
		return code
	}
	return exit
}

type reproRecordOptions struct {
	output       string
	inputs       []string
	expectations []repro.Expectation
	arguments    []string
}

func parseReproRecord(args []string) (reproRecordOptions, error) {
	var options reproRecordOptions
	invalid := func(message string) (reproRecordOptions, error) {
		return options, reproRefusal("repro-invalid-arguments", message)
	}
	expect := func(kind, text string, exit *int) {
		options.expectations = append(options.expectations, repro.Expectation{ID: "e" + strconv.Itoa(len(options.expectations)+1), Kind: kind, Text: text, Exit: exit})
	}
	for index := 0; index < len(args); index++ {
		name := args[index]
		if name == "--" {
			options.arguments = args[index+1:]
			break
		}
		if name == "--expect-same-stdout" {
			expect(repro.StdoutSame, "", nil)
			continue
		}
		if index+1 >= len(args) {
			return invalid(name + " needs a value")
		}
		value := args[index+1]
		index++
		switch name {
		case "--output":
			if options.output != "" || value == "" {
				return invalid("--output is required once")
			}
			options.output = value
		case "--input":
			if err := repro.ValidatePath(filepath.ToSlash(value)); err != nil {
				return invalid(err.Error())
			}
			options.inputs = append(options.inputs, filepath.ToSlash(value))
		case "--expect-exit":
			code, err := strconv.Atoi(value)
			if err != nil {
				return invalid("--expect-exit needs an integer")
			}
			expect(repro.ExitCode, "", &code)
		case "--expect-stdout-contains":
			expect(repro.StdoutContains, value, nil)
		case "--expect-stdout-not-contains":
			expect(repro.StdoutNotContains, value, nil)
		case "--expect-stderr-contains":
			expect(repro.StderrContains, value, nil)
		default:
			return invalid("unknown argument " + name)
		}
	}
	if options.output == "" {
		return invalid("--output is required")
	}
	if err := repro.ValidateArguments(options.arguments); err != nil {
		return invalid(err.Error() + " (give them after --)")
	}
	if len(options.inputs) > repro.MaxInputFiles {
		return invalid(fmt.Sprintf("at most %d --input files", repro.MaxInputFiles))
	}
	if len(options.expectations) > 0 {
		if err := repro.ValidateExpectations(options.expectations); err != nil {
			return invalid(err.Error())
		}
	}
	return options, nil
}

func reproRecord(ctx context.Context, root string, args []string) (map[string]any, error) {
	options, err := parseReproRecord(args)
	if err != nil {
		return nil, err
	}
	repository, problems, err := reproRepository(ctx, root, "")
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, reproRefusal("repro-inputs-unpinned", strings.Join(problems, "; "))
	}
	files := []repro.InputFile{}
	seen := map[string]bool{}
	for _, name := range options.inputs {
		if seen[name] {
			return nil, reproRefusal("repro-invalid-arguments", "--input "+name+" is repeated")
		}
		seen[name] = true
		digest, problem := reproInputDigest(root, name)
		if problem != "" {
			return nil, reproRefusal("repro-inputs-unpinned", problem)
		}
		files = append(files, repro.InputFile{Path: name, SHA256: digest})
	}
	exitCode, stdout, stderr := reproExecute(ctx, root, options.arguments)
	bundle := repro.Bundle{
		Profile: repro.Profile, RecordedBy: reproCurrentBuild(), Command: repro.Command{Arguments: options.arguments},
		Inputs: repro.Inputs{Repository: repository, Files: files}, Observed: repro.Observe(exitCode, stdout, stderr),
		Expectations: options.expectations,
	}
	if len(bundle.Expectations) == 0 {
		code := exitCode
		bundle.Expectations = []repro.Expectation{{ID: "e1", Kind: repro.ExitCode, Exit: &code}, {ID: "e2", Kind: repro.StdoutSame}}
	}
	if err := bundle.Validate(); err != nil {
		return nil, reproRefusal("repro-invalid-arguments", err.Error())
	}
	encoded, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, reproRefusal("repro-output-failed", "cannot encode the bundle")
	}
	output := options.output
	if !filepath.IsAbs(output) {
		if output, err = filepath.Abs(output); err != nil {
			return nil, reproRefusal("repro-output-failed", "cannot resolve --output")
		}
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil, reproRefusal("repro-output-exists", "--output "+options.output+" already exists")
	}
	if err != nil {
		return nil, reproRefusal("repro-output-failed", "cannot create --output "+options.output)
	}
	if _, err = file.Write(append(encoded, '\n')); err == nil {
		err = file.Close()
	} else {
		_ = file.Close()
	}
	if err != nil {
		_ = os.Remove(output)
		return nil, reproRefusal("repro-output-failed", "cannot write --output "+options.output)
	}
	record := map[string]any{
		"profile": "corvint-repro-record/0", "bundle": output, "exitCode": exitCode,
		"stdoutSha256": bundle.Observed.StdoutSHA256, "repository": bundle.Inputs.Repository,
		"inputFiles": len(files), "expectations": len(bundle.Expectations),
	}
	// RPB-V0-048: a recording whose own observed run fails its expectations
	// is still written, but says so.
	warnings := []string{}
	for _, result := range repro.Evaluate(bundle, exitCode, stdout, stderr) {
		if result.Status != "PASS" {
			warnings = append(warnings, "the observed run does not satisfy expectation "+result.ID)
		}
	}
	if len(warnings) != 0 {
		record["warnings"] = warnings
	}
	return record, nil
}

func reproRun(ctx context.Context, root string, args []string) (map[string]any, int, error) {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		return nil, 0, reproRefusal("repro-invalid-arguments", "repro run takes exactly one BUNDLE path")
	}
	data, err := reproReadBounded(args[0], repro.MaxBundleBytes)
	if err != nil {
		return nil, 0, reproRefusal("repro-bundle-unavailable", "cannot read bundle "+args[0])
	}
	bundle, err := repro.Decode(data)
	if err != nil {
		return nil, 0, reproRefusal("repro-bundle-invalid", err.Error())
	}
	_, problems, err := reproRepository(ctx, root, bundle.Inputs.Repository.Commit)
	if err != nil {
		return nil, 0, err
	}
	for _, file := range bundle.Inputs.Files {
		digest, problem := reproInputDigest(root, file.Path)
		if problem == "" && digest != file.SHA256 {
			problem = "input " + file.Path + " changed since recording"
		}
		if problem != "" {
			problems = append(problems, problem)
		}
	}
	if len(problems) > 0 {
		return nil, 0, reproRefusal("repro-inputs-unavailable", strings.Join(problems, "; "))
	}
	exitCode, stdout, stderr := reproExecute(ctx, root, bundle.Command.Arguments)
	results := repro.Evaluate(bundle, exitCode, stdout, stderr)
	outcome, exit := "PASS", 0
	for _, result := range results {
		if result.Status != "PASS" {
			outcome, exit = "FAIL", 1
		}
	}
	current := reproCurrentBuild()
	recorded := bundle.RecordedBy
	sameBuild := current.Commit != producerUnknown && current == recorded
	return map[string]any{
		"profile": "corvint-repro-run/0", "outcome": outcome, "results": results,
		"build": current, "recordedBy": recorded, "sameBuild": sameBuild,
		"repository": bundle.Inputs.Repository, "arguments": bundle.Command.Arguments,
		"observed": map[string]any{"exitCode": exitCode, "stdoutSha256": repro.Digest(stdout), "stdoutBytes": len(stdout)},
	}, exit, nil
}

// reproCwdMu serializes the working-directory change reproExecute makes: the
// process working directory is global.
var reproCwdMu sync.Mutex

// reproExecute runs one corvint invocation in this process with empty stdin.
// The recorded arguments resolve against the repository root, never the
// caller's working directory, so a record and a replay agree wherever they run
// (RPB-V0-047).
func reproExecute(ctx context.Context, root string, arguments []string) (int, []byte, []byte) {
	var stdout, stderr bytes.Buffer
	reproCwdMu.Lock()
	defer reproCwdMu.Unlock()
	if previous, err := os.Getwd(); err == nil {
		if current, _ := filepath.EvalSymlinks(previous); current != "" {
			if target, _ := filepath.EvalSymlinks(root); target != current {
				if os.Chdir(root) == nil {
					defer func() { _ = os.Chdir(previous) }()
				}
			}
		}
	}
	invocation := append([]string{"--root", root}, arguments...)
	code := runContext(ctx, invocation, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.Bytes(), stderr.Bytes()
}

func reproCurrentBuild() repro.Build {
	info, _ := debug.ReadBuildInfo()
	identity := versionIdentityFrom(version, build, sourceCommit, info)
	return repro.Build{Version: version, DisplayVersion: identity.DisplayVersion, Build: identity.Build, Commit: identity.Commit, CommitState: identity.CommitState, Release: identity.Release}
}

// reproRepository reads HEAD's commit and tree and reports why the checkout is not the pinned
// input: a dirty tracked tree, or (with want) a HEAD other than want or an absent want commit.
func reproRepository(ctx context.Context, root, want string) (repro.Repository, []string, error) {
	unavailable := func(err error) (repro.Repository, []string, error) {
		return repro.Repository{}, nil, reproRefusal("repro-repository-unavailable", "--root is not a readable Git checkout: "+err.Error())
	}
	head, err := reproGit(ctx, root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return unavailable(err)
	}
	tree, err := reproGit(ctx, root, "rev-parse", "--verify", "HEAD^{tree}")
	if err != nil {
		return unavailable(err)
	}
	status, err := reproGit(ctx, root, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return unavailable(err)
	}
	problems := []string{}
	if want != "" && head != want {
		if _, err := reproGit(ctx, root, "cat-file", "-e", want+"^{commit}"); err != nil {
			problems = append(problems, "pinned commit "+want+" is not in this repository")
		} else {
			problems = append(problems, "HEAD is "+head+", not the pinned commit "+want+" (check it out first)")
		}
	}
	if status != "" {
		problems = append(problems, "the tracked tree has uncommitted changes")
	}
	return repro.Repository{Commit: head, Tree: tree}, problems, nil
}

func reproGit(ctx context.Context, root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, reproGitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s failed", args[0])
	}
	return strings.TrimSpace(stdout.String()), nil
}

// reproInputDigest digests one regular, non-symlink repository file, or names why it cannot.
func reproInputDigest(root, name string) (string, string) {
	full := filepath.Join(root, filepath.FromSlash(name))
	info, err := os.Lstat(full)
	if err != nil {
		return "", "input " + name + " is missing"
	}
	if !info.Mode().IsRegular() {
		return "", "input " + name + " is not a regular file"
	}
	data, err := reproReadBounded(full, reproMaxInputSize)
	if err != nil {
		return "", "input " + name + " is unreadable or larger than 16 MiB"
	}
	return repro.Digest(data), ""
}

func reproReadBounded(name string, limit int64) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", name, limit)
	}
	return data, nil
}

// reproPlain converts a payload holding structs into the maps, slices, strings, booleans and
// integers the canonical encoder accepts.
func reproPlain(value any) any {
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var plain any
	if decoder.Decode(&plain) != nil {
		return value
	}
	return reproIntegers(plain)
}

func reproIntegers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, member := range typed {
			typed[key] = reproIntegers(member)
		}
	case []any:
		for index, member := range typed {
			typed[index] = reproIntegers(member)
		}
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer
		}
		return typed.String()
	}
	return value
}
