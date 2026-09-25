package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/worklistadapter"
	"github.com/Beamfall/corvint/internal/workqueue"
)

// workAdapterPath is the committed adapter `work init` writes.
const workAdapterPath = ".corvint/work-queue-adapter"

const workLegacyAdapterScript = `#!/bin/sh
# repository-work-queue-adapter/0 written by ` + "`corvint work init`" + `. Corvint runs it
# with a fixed PATH; install corvint in /opt/homebrew/bin or /usr/local/bin.
exec corvint work adapter "$@"
`

const workEmptyWorklist = `{"profile":"corvint-worklist/0","tickets":[]}
`

// workAdoptionVerbs are the two work operations outside work-command-result/0:
// they print their own output and never reach the observer.
var workAdoptionVerbs = map[string]func(context.Context, string, []string, io.Writer, io.Writer) int{
	"adapter": runWorkAdapter,
	"init":    runWorkInit,
	"rebind":  runWorkRebind,
}

func runWorkAdoption(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) (int, bool) {
	if len(arguments) == 0 {
		return 0, false
	}
	verb, found := workAdoptionVerbs[arguments[0]]
	if !found {
		return 0, false
	}
	return verb(ctx, root, arguments[1:], stdout, stderr), true
}

// runWorkAdapter is the repository-work-queue-adapter/0 producer for a committed
// worklist (WQO-V0-048). It reads only qualified source and writes one document.
func runWorkAdapter(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	if err := worklistadapter.Run(ctx, "corvint work adapter", root, arguments, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

// runWorkInit writes the three adoption files and refuses to replace any (WQO-V0-047/049).
func runWorkInit(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	options, err := parseWorkInitArguments(arguments)
	if err != nil {
		fmt.Fprintln(stderr, "corvint work init:", err)
		return 2
	}
	root, err = resolveQueryRoot(root)
	if err != nil {
		fmt.Fprintln(stderr, "corvint work init:", err)
		return 2
	}
	repositoryBoundary, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		fmt.Fprintln(stderr, "corvint work init: repository boundary is unqualified:", err)
		return 2
	}
	binding, executable, err := workBindExecutable(ctx, options.executable, []string{repositoryBoundary.Root, repositoryBoundary.GitDir, repositoryBoundary.CommonDir})
	if err != nil {
		fmt.Fprintln(stderr, "corvint work init: executable is unqualified:", err)
		return 2
	}
	defer executable.close()
	policy, err := workAdoptionPolicy(options.repository)
	if err != nil {
		fmt.Fprintln(stderr, "corvint work init:", err)
		return 2
	}
	worklistPath, _ := worklistadapter.WorklistPath(worklistadapter.RepositoryMapping)
	files := []workAdoptionFile{
		{worklistadapter.PolicyPath, policy.Canonical(), 0644},
		{worklistPath, []byte(workEmptyWorklist), 0644},
		{workAdapterPath, workBoundAdapterScript(binding), 0755},
	}
	repository, err := os.OpenRoot(root)
	if err != nil {
		fmt.Fprintln(stderr, "corvint work init:", err)
		return 2
	}
	defer repository.Close()
	for _, file := range files {
		if _, err := repository.Lstat(file.path); !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "corvint work init: %s already exists; nothing was written\n", file.path)
			return 2
		}
	}
	directoryInfo, err := repository.Lstat(".corvint")
	createdDirectory := false
	if errors.Is(err, os.ErrNotExist) {
		err = repository.Mkdir(".corvint", 0755)
		if err == nil {
			createdDirectory = true
			directoryInfo, err = repository.Lstat(".corvint")
		}
	}
	if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode()&os.ModeSymlink != 0 {
		if err == nil {
			err = errors.New(".corvint must be a repository-local directory, not a symlink")
		}
		rollbackWorkAdoptionDirectory(repository, createdDirectory)
		fmt.Fprintln(stderr, "corvint work init:", err)
		return 2
	}
	directory, err := repository.OpenRoot(".corvint")
	if err != nil {
		rollbackWorkAdoptionDirectory(repository, createdDirectory)
		fmt.Fprintln(stderr, "corvint work init:", err)
		return 2
	}
	openedInfo, err := directory.Stat(".")
	if err != nil || !os.SameFile(directoryInfo, openedInfo) {
		directory.Close()
		rollbackWorkAdoptionDirectory(repository, createdDirectory)
		if err == nil {
			err = errors.New(".corvint changed during initialization")
		}
		fmt.Fprintln(stderr, "corvint work init:", err)
		return 2
	}
	created, err := writeWorkAdoptionFiles(directory, files)
	if err == nil {
		var after os.FileInfo
		after, err = repository.Lstat(".corvint")
		if err == nil && !os.SameFile(openedInfo, after) {
			err = errors.New(".corvint changed during initialization")
		}
	}
	if err != nil {
		rollbackWorkAdoptionFiles(directory, created)
		directory.Close()
		rollbackWorkAdoptionDirectory(repository, createdDirectory)
		fmt.Fprintln(stderr, "corvint work init:", err)
		return 2
	}
	_ = directory.Close()
	for _, file := range files {
		fmt.Fprintln(stdout, file.path)
	}
	workReportResolvedExecutable(stderr, "init", options.executable, binding.Path)
	fmt.Fprintln(stderr, "corvint work init: review and commit these three files; work observe and propose-wave return SOURCE_UNQUALIFIED until they are committed")
	return 0
}

type workInitOptions struct {
	repository string
	executable string
}

func parseWorkInitArguments(arguments []string) (workInitOptions, error) {
	options := workInitOptions{}
	for index := 0; index < len(arguments); {
		name, value, inline := strings.Cut(arguments[index], "=")
		if name != "--repository" && name != "--corvint-executable" {
			return options, workInitUsage()
		}
		if !inline {
			if index+1 >= len(arguments) || strings.HasPrefix(arguments[index+1], "--") {
				return options, workInitUsage()
			}
			value, index = arguments[index+1], index+2
		} else {
			index++
		}
		if value == "" {
			return options, workInitUsage()
		}
		switch name {
		case "--repository":
			if options.repository != "" {
				return options, workInitUsage()
			}
			options.repository = value
		case "--corvint-executable":
			if options.executable != "" {
				return options, workInitUsage()
			}
			options.executable = value
		}
	}
	if options.repository == "" || options.executable == "" {
		return options, workInitUsage()
	}
	return options, nil
}

func workInitUsage() error {
	return errors.New("usage: corvint [--root PATH] work init --repository NAME --corvint-executable ABSOLUTE_FILE")
}

// runWorkRebind atomically replaces only the generated adapter binding. The
// changed tracked file remains pending for operator review and commit (WQO-V0-050).
func runWorkRebind(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	executablePath, err := parseWorkRebindArguments(arguments)
	if err != nil {
		fmt.Fprintln(stderr, "corvint work rebind:", err)
		return 2
	}
	root, err = resolveQueryRoot(root)
	if err != nil {
		fmt.Fprintln(stderr, "corvint work rebind:", err)
		return 2
	}
	source, err := acquireWorkSource(ctx, root)
	if err != nil {
		fmt.Fprintln(stderr, "corvint work rebind: existing adoption is unqualified:", workSourceReason(err))
		return 2
	}
	defer source.qualified.Close()
	policy, err := workqueue.ParsePolicy(source.policyRaw)
	if err != nil || policy.MappingVersion != worklistadapter.RepositoryMapping || policy.AdapterPath != workAdapterPath {
		fmt.Fprintln(stderr, "corvint work rebind: committed work-queue policy is not the repository adoption profile")
		return 2
	}
	binding, executable, err := workBindExecutable(ctx, executablePath, []string{source.qualified.Root, source.qualified.GitDir, source.qualified.CommonDir})
	if err != nil {
		fmt.Fprintln(stderr, "corvint work rebind: executable is unqualified:", err)
		return 2
	}
	defer executable.close()
	if err := replaceWorkExecutableBinding(root, binding); err != nil {
		fmt.Fprintln(stderr, "corvint work rebind:", err)
		return 2
	}
	workReportResolvedExecutable(stderr, "rebind", executablePath, binding.Path)
	fmt.Fprintln(stdout, workAdapterPath)
	return 0
}

// workReportResolvedExecutable tells the reviewer which real file a symlinked
// --corvint-executable bound; upgrading that link later needs a rebind.
func workReportResolvedExecutable(stderr io.Writer, verb, requested, bound string) {
	if requested != bound {
		fmt.Fprintf(stderr, "corvint work %s: %s resolves through a symlink; bound its target %s (after an upgrade, run work rebind)\n", verb, requested, bound)
	}
}

func parseWorkRebindArguments(arguments []string) (string, error) {
	if len(arguments) == 2 && arguments[0] == "--corvint-executable" && arguments[1] != "" && !strings.HasPrefix(arguments[1], "--") {
		return arguments[1], nil
	}
	if len(arguments) == 1 && strings.HasPrefix(arguments[0], "--corvint-executable=") {
		if value := strings.TrimPrefix(arguments[0], "--corvint-executable="); value != "" {
			return value, nil
		}
	}
	return "", errors.New("usage: corvint [--root PATH] work rebind --corvint-executable ABSOLUTE_FILE")
}

func replaceWorkExecutableBinding(root string, binding workCorvintExecutableBinding) error {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer repository.Close()
	directoryInfo, err := repository.Lstat(".corvint")
	if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New(".corvint must be a repository-local directory, not a symlink")
	}
	directory, err := repository.OpenRoot(".corvint")
	if err != nil {
		return err
	}
	defer directory.Close()
	openedDirectory, err := directory.Stat(".")
	if err != nil || !os.SameFile(directoryInfo, openedDirectory) || directoryInfo.Mode() != openedDirectory.Mode() {
		return errors.New(".corvint changed while opening")
	}
	policyRaw, _, err := readWorkAdoptionFile(directory, strings.TrimPrefix(worklistadapter.PolicyPath, ".corvint/"), 64<<10)
	if err != nil {
		return err
	}
	policy, err := workqueue.ParsePolicy(policyRaw)
	if err != nil || policy.MappingVersion != worklistadapter.RepositoryMapping || policy.AdapterPath != workAdapterPath {
		return errors.New("committed work-queue policy is not the repository adoption profile")
	}
	adapterName := strings.TrimPrefix(workAdapterPath, ".corvint/")
	current, before, err := readWorkAdoptionFile(directory, adapterName, 64<<10)
	if err != nil {
		return err
	}
	if before.Mode().Perm() != 0755 {
		return errors.New("existing work-queue adapter is not a regular 0755 file")
	}
	if !bytes.Equal(current, []byte(workLegacyAdapterScript)) {
		if _, err := workParseBoundAdapter(current); err != nil {
			return errors.New("existing work-queue adapter is not a generated Corvint adapter")
		}
	}
	after, err := directory.Lstat(adapterName)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		return errors.New("work-queue adapter changed during rebind")
	}
	temporary, err := writeWorkRebindTemporary(directory, workBoundAdapterScript(binding))
	if err != nil {
		return err
	}
	defer directory.Remove(temporary)
	latestRaw, latest, err := readWorkAdoptionFile(directory, adapterName, 64<<10)
	if err != nil || !bytes.Equal(current, latestRaw) || !os.SameFile(after, latest) || after.Mode() != latest.Mode() || after.Size() != latest.Size() {
		return errors.New("work-queue adapter changed during rebind")
	}
	currentDirectory, err := repository.Lstat(".corvint")
	if err != nil || !os.SameFile(directoryInfo, currentDirectory) || directoryInfo.Mode() != currentDirectory.Mode() {
		return errors.New(".corvint changed during rebind")
	}
	return directory.Rename(temporary, adapterName)
}

func writeWorkRebindTemporary(root *os.Root, raw []byte) (string, error) {
	for range 8 {
		nonce := make([]byte, 16)
		if _, err := rand.Read(nonce); err != nil {
			return "", err
		}
		name := ".work-queue-adapter.rebind-" + hex.EncodeToString(nonce)
		created, err := (workAdoptionFile{path: ".corvint/" + name, raw: raw, mode: 0755}).write(root)
		if err == nil {
			return created, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", errors.New("cannot allocate work-queue adapter replacement")
}

func readWorkAdoptionFile(root *os.Root, name string, limit int64) ([]byte, os.FileInfo, error) {
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.New("adoption file must be a regular file")
	}
	handle, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}
	defer handle.Close()
	opened, err := handle.Stat()
	if err != nil || !os.SameFile(before, opened) || before.Mode() != opened.Mode() {
		return nil, nil, errors.New("adoption file changed during read")
	}
	raw, err := io.ReadAll(io.LimitReader(handle, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, nil, errors.New("adoption file exceeds read bound")
	}
	after, err := root.Lstat(name)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		return nil, nil, errors.New("adoption file changed during read")
	}
	return raw, before, nil
}

// workAdoptionPolicy derives every authority from one owner-chosen repository name
// and returns the policy only after the closed WQO-V0-001 parser accepts it.
func workAdoptionPolicy(name string) (*workqueue.Policy, error) {
	operation := func(verb string) []string { return []string{verb} }
	policy := &workqueue.Policy{
		AccessContextID: "access:" + name + ":local", AdapterPath: workAdapterPath,
		AdapterProfile: "repository-work-queue-adapter/0", DetailLimit: "512",
		MappingVersion: worklistadapter.RepositoryMapping,
		Operations:     workqueue.PolicyOperations{Details: operation("details"), Snapshot: operation("snapshot"), Verify: operation("verify")},
		Profile:        workqueue.PolicyProfile, QueueAuthorityID: "queue:" + name + ":worklist",
		RepositoryAuthorityID: "repo:" + name, ScopeID: "scope:" + name + ":worklist",
	}
	policy.RefreshIdentity()
	parsed, err := workqueue.ParsePolicy(policy.Canonical())
	if err != nil {
		return nil, fmt.Errorf("repository name %q is not a valid authority token", name)
	}
	return parsed, nil
}

type workAdoptionFile struct {
	path string
	raw  []byte
	mode os.FileMode
}

func (file workAdoptionFile) name() (string, error) {
	const prefix = ".corvint/"
	name := strings.TrimPrefix(file.path, prefix)
	if name == file.path || name == "" || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid adoption path %q", file.path)
	}
	return name, nil
}

func (file workAdoptionFile) write(root *os.Root) (string, error) {
	name, err := file.name()
	if err != nil {
		return "", err
	}
	handle, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, file.mode)
	if err != nil {
		return "", err
	}
	written, err := handle.Write(file.raw)
	if err == nil && written != len(file.raw) {
		err = io.ErrShortWrite
	}
	if err != nil {
		handle.Close()
		return name, err
	}
	if err = handle.Chmod(file.mode); err != nil {
		handle.Close()
		return name, err
	}
	return name, handle.Close()
}

func writeWorkAdoptionFiles(root *os.Root, files []workAdoptionFile) ([]string, error) {
	created := make([]string, 0, len(files))
	for _, file := range files {
		name, err := file.write(root)
		if name != "" {
			created = append(created, name)
		}
		if err != nil {
			rollbackWorkAdoptionFiles(root, created)
			return nil, err
		}
	}
	return created, nil
}

func rollbackWorkAdoptionFiles(root *os.Root, created []string) {
	for index := len(created) - 1; index >= 0; index-- {
		_ = root.Remove(created[index])
	}
}

func rollbackWorkAdoptionDirectory(root *os.Root, created bool) {
	if created {
		_ = root.Remove(".corvint")
	}
}
