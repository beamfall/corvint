package main

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const workCorvintPackage = "github.com/Beamfall/corvint/cmd/corvint"

const workBindingPrefix = "# corvint-executable-binding: "

type workCorvintSourceIdentity struct {
	Package       string `json:"package"`
	Module        string `json:"module"`
	ModuleVersion string `json:"moduleVersion"`
	GoVersion     string `json:"goVersion"`
	VCS           string `json:"vcs"`
	Revision      string `json:"revision"`
	Time          string `json:"time"`
	Modified      bool   `json:"modified"`
}

type workCorvintExecutableBinding struct {
	Path    string                    `json:"path"`
	SHA256  string                    `json:"sha256"`
	Version string                    `json:"version"`
	Source  workCorvintSourceIdentity `json:"source"`
}

func workBoundAdapterScript(binding workCorvintExecutableBinding) []byte {
	encoded, _ := json.Marshal(binding)
	return []byte("#!/bin/sh\n" +
		"# repository-work-queue-adapter/0 written by `corvint work init` or `corvint work rebind`.\n" +
		"# Corvint verifies this reviewed binding, then supplies its private exact-byte path as argv 1.\n" +
		workBindingPrefix + string(encoded) + "\n" +
		"corvint_executable=$1\nshift\nexec \"$corvint_executable\" work adapter \"$@\"\n")
}

func workParseBoundAdapter(raw []byte) (workCorvintExecutableBinding, error) {
	var binding workCorvintExecutableBinding
	lines := bytes.Split(raw, []byte{'\n'})
	if len(lines) != 8 || string(lines[0]) != "#!/bin/sh" || !bytes.HasPrefix(lines[3], []byte(workBindingPrefix)) || len(lines[7]) != 0 {
		return binding, errors.New("invalid generated adapter shape")
	}
	decoder := json.NewDecoder(bytes.NewReader(lines[3][len(workBindingPrefix):]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&binding); err != nil {
		return binding, errors.New("invalid generated executable binding")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return binding, errors.New("invalid generated executable binding framing")
	}
	if !bytes.Equal(raw, workBoundAdapterScript(binding)) {
		return binding, errors.New("noncanonical generated adapter")
	}
	decodedDigest, digestErr := hex.DecodeString(binding.SHA256)
	if !filepath.IsAbs(binding.Path) || filepath.Clean(binding.Path) != binding.Path || len(decodedDigest) != 32 || digestErr != nil || binding.SHA256 != strings.ToLower(binding.SHA256) {
		return binding, errors.New("invalid generated executable identity")
	}
	if !strings.HasPrefix(binding.Version, "Corvint ") || !strings.Contains(binding.Version, " (build ") || !strings.HasSuffix(binding.Version, ")") || strings.ContainsAny(binding.Version, "\r\n\x00") {
		return binding, errors.New("invalid generated version identity")
	}
	if binding.Source.Package != workCorvintPackage || binding.Source.Module == "" || binding.Source.GoVersion == "" {
		return binding, errors.New("invalid generated source identity")
	}
	return binding, nil
}

func workBindExecutable(ctx context.Context, path string, protectedRoots []string) (workCorvintExecutableBinding, *workExecutable, error) {
	object, source, err := workOpenBoundExecutable(path, protectedRoots)
	if err != nil {
		return workCorvintExecutableBinding{}, nil, err
	}
	version, err := workBoundExecutableVersion(ctx, object, []string{"LANG=C", "LC_ALL=C"}, string(filepath.Separator))
	if err != nil {
		object.close()
		return workCorvintExecutableBinding{}, nil, err
	}
	if err := object.check(); err != nil {
		object.close()
		return workCorvintExecutableBinding{}, nil, err
	}
	binding := workCorvintExecutableBinding{Path: path, SHA256: object.digest, Version: version, Source: source}
	return binding, object, nil
}

func workOpenBoundExecutable(path string, protectedRoots []string) (*workExecutable, workCorvintSourceIdentity, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, workCorvintSourceIdentity{}, errors.New("path must be canonical and absolute")
	}
	for _, root := range protectedRoots {
		if workPathWithin(path, root) {
			return nil, workCorvintSourceIdentity{}, errors.New("path is repository-controlled")
		}
	}
	object, _, err := workReadExecutable(path)
	if err != nil {
		return nil, workCorvintSourceIdentity{}, err
	}
	fail := func(message string) (*workExecutable, workCorvintSourceIdentity, error) {
		object.close()
		return nil, workCorvintSourceIdentity{}, errors.New(message)
	}
	for index, binding := range object.bindings {
		if binding.link != "" {
			return fail("path and parent components must not be symlinks")
		}
		if index+1 < len(object.bindings) {
			if !binding.info.IsDir() || workUnsafeParent(binding.info.Mode()) {
				return fail("path has an unsafe parent component")
			}
		}
	}
	if object.info.Mode().Perm()&0022 != 0 {
		return fail("executable must not be group- or world-writable")
	}
	source, err := workCorvintSource(object.file)
	if err != nil {
		return fail(err.Error())
	}
	if err := workMaterializeBoundExecutable(object); err != nil {
		return fail("executable cannot be privately materialized for exact-byte execution")
	}
	return object, source, nil
}

func workMaterializeBoundExecutable(object *workExecutable) error {
	temporaryRoot, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		return err
	}
	rootInfo, err := os.Lstat(temporaryRoot)
	if err != nil || !rootInfo.IsDir() || workUnsafeParent(rootInfo.Mode()) {
		return errors.New("unsafe private executable root")
	}
	directory, err := os.MkdirTemp(temporaryRoot, "corvint-work-executable-")
	if err != nil {
		return err
	}
	fail := func(err error) error {
		_ = os.RemoveAll(directory)
		return err
	}
	directoryInfo, err := os.Lstat(directory)
	if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode().Perm() != 0700 {
		return fail(errors.New("unsafe private executable directory"))
	}
	destination := filepath.Join(directory, "corvint")
	handle, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		return fail(err)
	}
	written, copyErr := io.Copy(handle, io.NewSectionReader(object.file, 0, object.info.Size()))
	if copyErr == nil && written != object.info.Size() {
		copyErr = io.ErrShortWrite
	}
	if copyErr == nil {
		copyErr = handle.Chmod(0500)
	}
	closeErr := handle.Close()
	if copyErr != nil {
		return fail(copyErr)
	}
	if closeErr != nil {
		return fail(closeErr)
	}
	executionFile, err := workOpenExecutable(destination)
	if err != nil {
		return fail(err)
	}
	executionInfo, err := executionFile.Stat()
	if err != nil {
		executionFile.Close()
		return fail(err)
	}
	object.executionPath = destination
	object.executionDirectory = directory
	object.executionFile = executionFile
	object.executionInfo = executionInfo
	object.executionDirectoryID = directoryInfo
	if err := object.checkExecutionMaterialization(); err != nil {
		object.close()
		return err
	}
	return nil
}

func workUnsafeParent(mode os.FileMode) bool {
	return mode.Perm()&0022 != 0 && mode&os.ModeSticky == 0
}

func workPathWithin(path, root string) bool {
	if root == "" {
		return false
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return true
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return true
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func workCorvintSource(file *os.File) (workCorvintSourceIdentity, error) {
	info, err := buildinfo.Read(file)
	if err != nil || info.Path != workCorvintPackage {
		return workCorvintSourceIdentity{}, errors.New("executable is not a Corvint Go build")
	}
	result := workCorvintSourceIdentity{Package: info.Path, Module: info.Main.Path, ModuleVersion: info.Main.Version, GoVersion: info.GoVersion}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs":
			result.VCS = setting.Value
		case "vcs.revision":
			result.Revision = setting.Value
		case "vcs.time":
			result.Time = setting.Value
		case "vcs.modified":
			result.Modified, err = strconv.ParseBool(setting.Value)
			if err != nil {
				return workCorvintSourceIdentity{}, errors.New("invalid Corvint source modification identity")
			}
		}
	}
	if result.Module != "github.com/Beamfall/corvint" {
		return workCorvintSourceIdentity{}, errors.New("unexpected Corvint module identity")
	}
	if result.ModuleVersion == "" {
		return workCorvintSourceIdentity{}, errors.New("Corvint module source identity is incomplete")
	}
	if result.Revision == "" && (result.VCS != "" || result.Time != "" || result.Modified) {
		return workCorvintSourceIdentity{}, errors.New("Corvint source identity is inconsistent")
	}
	if result.Revision != "" && result.VCS != "git" {
		return workCorvintSourceIdentity{}, errors.New("unsupported Corvint source identity")
	}
	return result, nil
}

func workBoundExecutableVersion(parent context.Context, object *workExecutable, environment []string, directory string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	stdout := &workLimitedBuffer{limit: 4096}
	stderr := &workLimitedBuffer{limit: 4096}
	command := exec.CommandContext(ctx, object.executionPath, "--version")
	command.Dir = directory
	command.Env = append([]string(nil), environment...)
	command.Stdout, command.Stderr = stdout, stderr
	workContain(command)
	command.Cancel = func() error { workKillGroup(command); return nil }
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("Corvint version identity failed: %w", err)
	}
	if stdout.exceeded || stderr.exceeded || stderr.count != 0 {
		return "", errors.New("Corvint version identity output is invalid")
	}
	version := strings.TrimSuffix(string(stdout.data), "\n")
	if version == string(stdout.data) || !strings.HasPrefix(version, "Corvint ") || !strings.Contains(version, " (build ") || !strings.HasSuffix(version, ")") || strings.ContainsAny(version, "\r\x00") {
		return "", errors.New("Corvint version identity output is invalid")
	}
	return version, nil
}

func workVerifyBoundExecutable(ctx context.Context, binding workCorvintExecutableBinding, object *workExecutable, environment []string, directory string) error {
	if object.digest != binding.SHA256 {
		return errors.New("bound Corvint executable bytes changed")
	}
	source, err := workCorvintSource(object.file)
	if err != nil || source != binding.Source {
		return errors.New("bound Corvint source identity changed")
	}
	version, err := workBoundExecutableVersion(ctx, object, environment, directory)
	if err != nil || version != binding.Version {
		return errors.New("bound Corvint version/build identity changed")
	}
	if err := object.check(); err != nil {
		return err
	}
	return nil
}
