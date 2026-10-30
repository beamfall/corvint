package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/workqueue"
)

const workExecutableLimit = 128 << 20

type workPathBinding struct {
	path string
	info os.FileInfo
	link string
}

type workExecutable struct {
	path                 string
	file                 *os.File
	info                 os.FileInfo
	digest               string
	bindings             []workPathBinding
	executionPath        string
	executionDirectory   string
	executionFile        *os.File
	executionInfo        os.FileInfo
	executionDirectoryID os.FileInfo
}

func (object *workExecutable) close() {
	if object.executionFile != nil {
		_ = object.executionFile.Close()
	}
	_ = object.file.Close()
	if object.executionDirectory != "" {
		_ = os.RemoveAll(object.executionDirectory)
	}
}

// Resolve system interpreter symlinks explicitly and retain every component's
// identity. Rechecking these bindings is drift detection, never exact fd exec.
func workResolveExecutable(path string) (string, []workPathBinding, error) {
	if !filepath.IsAbs(path) {
		return "", nil, errors.New("relative executable")
	}
	pending := strings.Split(filepath.Clean(path), string(filepath.Separator))[1:]
	current := string(filepath.Separator)
	bindings := []workPathBinding{}
	links := 0
	for len(pending) > 0 {
		current = filepath.Join(current, pending[0])
		pending = pending[1:]
		info, err := os.Lstat(current)
		if err != nil {
			return "", nil, err
		}
		binding := workPathBinding{path: current, info: info}
		if info.Mode()&os.ModeSymlink != 0 {
			links++
			if links > 40 {
				return "", nil, errors.New("interpreter symlink chain limit")
			}
			binding.link, err = os.Readlink(current)
			if err != nil {
				return "", nil, err
			}
			target := binding.link
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(current), target)
			}
			pending = append(strings.Split(filepath.Clean(target), string(filepath.Separator))[1:], pending...)
			current = string(filepath.Separator)
		} else if len(pending) > 0 && !info.IsDir() {
			return "", nil, errors.New("non-directory executable component")
		}
		bindings = append(bindings, binding)
	}
	return current, bindings, nil
}

func workReadExecutable(path string) (*workExecutable, []byte, error) {
	resolved, bindings, err := workResolveExecutable(path)
	if err != nil {
		return nil, nil, err
	}
	file, err := workOpenExecutable(resolved)
	if err != nil {
		return nil, nil, err
	}
	object := &workExecutable{path: path, file: file, bindings: bindings}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || !os.SameFile(info, bindings[len(bindings)-1].info) {
		file.Close()
		return nil, nil, errors.New("executable is not an executable regular file")
	}
	object.info = info
	raw, err := io.ReadAll(io.LimitReader(file, workExecutableLimit+1))
	if err != nil || len(raw) > workExecutableLimit {
		file.Close()
		return nil, nil, errors.New("executable read limit")
	}
	object.digest = workqueue.SHA256Hex(raw)
	if err := object.check(); err != nil {
		file.Close()
		return nil, nil, err
	}
	return object, raw, nil
}

func (object *workExecutable) check() error {
	for _, binding := range object.bindings {
		info, err := os.Lstat(binding.path)
		if err != nil || !os.SameFile(info, binding.info) || info.Mode() != binding.info.Mode() {
			return errors.New("executable pathname identity changed")
		}
		if binding.link != "" {
			link, err := os.Readlink(binding.path)
			if err != nil || link != binding.link {
				return errors.New("executable symlink changed")
			}
		}
	}
	before, err := object.file.Stat()
	if err != nil || !os.SameFile(before, object.info) || before.Mode() != object.info.Mode() {
		return errors.New("executable descriptor changed")
	}
	raw, err := io.ReadAll(io.NewSectionReader(object.file, 0, workExecutableLimit+1))
	if err != nil || len(raw) > workExecutableLimit || workqueue.SHA256Hex(raw) != object.digest {
		return errors.New("executable bytes changed")
	}
	after, err := object.file.Stat()
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		return errors.New("executable changed during read")
	}
	if object.executionFile != nil {
		if err := object.checkExecutionMaterialization(); err != nil {
			return err
		}
	}
	return nil
}

func (object *workExecutable) checkExecutionMaterialization() error {
	directory, err := os.Lstat(object.executionDirectory)
	if err != nil || !directory.IsDir() || directory.Mode().Perm() != 0700 || !os.SameFile(directory, object.executionDirectoryID) {
		return errors.New("private executable materialization changed")
	}
	pathInfo, err := os.Lstat(object.executionPath)
	if err != nil || !pathInfo.Mode().IsRegular() || pathInfo.Mode().Perm() != 0500 || !os.SameFile(pathInfo, object.executionInfo) {
		return errors.New("private executable materialization changed")
	}
	fileInfo, err := object.executionFile.Stat()
	if err != nil || !os.SameFile(fileInfo, object.executionInfo) || fileInfo.Mode() != object.executionInfo.Mode() || fileInfo.Size() != object.executionInfo.Size() {
		return errors.New("private executable descriptor changed")
	}
	raw, err := io.ReadAll(io.NewSectionReader(object.executionFile, 0, workExecutableLimit+1))
	if err != nil || workqueue.SHA256Hex(raw) != object.digest {
		return errors.New("private executable bytes changed")
	}
	return nil
}

func workExecutableChain(path string, expected []byte, mode string) ([]*workExecutable, []workqueue.ExecutableIdentity, error) {
	adapter, raw, err := workReadExecutable(path)
	if err != nil {
		return nil, nil, err
	}
	fail := func(err error) ([]*workExecutable, []workqueue.ExecutableIdentity, error) {
		adapter.file.Close()
		return nil, nil, err
	}
	if !bytes.Equal(raw, expected) || mode != "100755" {
		return fail(errors.New("adapter source identity mismatch"))
	}
	objects := []*workExecutable{adapter}
	identities := []workqueue.ExecutableIdentity{}
	if !bytes.HasPrefix(raw, []byte("#!")) {
		if !workNativeExecutable(raw) {
			return fail(errors.New("unsupported executable format"))
		}
		return objects, identities, nil
	}
	line, _, found := bytes.Cut(raw, []byte{'\n'})
	if !found || len(line) > 255 || bytes.ContainsAny(line, "\r\x00") {
		return fail(errors.New("unsupported shebang"))
	}
	fields := strings.Fields(string(line[2:]))
	if len(fields) == 0 || len(fields) > 2 {
		return fail(errors.New("unsupported interpreter arguments"))
	}
	interpreterPath := fields[0]
	if !filepath.IsAbs(interpreterPath) || filepath.Base(interpreterPath) == "env" {
		return fail(errors.New("unsupported interpreter indirection"))
	}
	interpreter, interpreterRaw, err := workReadExecutable(interpreterPath)
	if err != nil {
		return fail(err)
	}
	if filepath.Base(interpreter.file.Name()) == "env" {
		interpreter.file.Close()
		return fail(errors.New("unsupported interpreter indirection"))
	}
	if !workNativeExecutable(interpreterRaw) {
		interpreter.file.Close()
		return fail(errors.New("nested or unsupported interpreter"))
	}
	objects = append(objects, interpreter)
	identities = append(identities, workqueue.ExecutableIdentity{FileSHA256: interpreter.digest, Mode: fmt.Sprintf("%04o", interpreter.info.Mode().Perm()), PathSHA256: workqueue.SHA256Hex([]byte(interpreterPath))})
	return objects, identities, nil
}

func workNativeExecutable(raw []byte) bool {
	if len(raw) < 4 {
		return false
	}
	switch string(raw[:4]) {
	case "\x7fELF", "\xfe\xed\xfa\xce", "\xce\xfa\xed\xfe", "\xfe\xed\xfa\xcf", "\xcf\xfa\xed\xfe", "\xca\xfe\xba\xbe", "\xbe\xba\xfe\xca", "\xca\xfe\xba\xbf", "\xbf\xba\xfe\xca":
		return true
	}
	return false
}
