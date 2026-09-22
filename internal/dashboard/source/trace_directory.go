package source

import "os"

type traceDirectoryBinding struct {
	parent     *os.Root
	parentFile *os.File
	name       string
	identity   FileIdentity
}

type heldTraceDirectory struct {
	file       *os.File
	root       *os.Root
	bindings   []traceDirectoryBinding
	ownedFiles []*os.File
	ownedRoots []*os.Root
}

func (directory *heldTraceDirectory) close() {
	if directory == nil {
		return
	}
	for index := len(directory.ownedRoots) - 1; index >= 0; index-- {
		_ = directory.ownedRoots[index].Close()
	}
	for index := len(directory.ownedFiles) - 1; index >= 0; index-- {
		_ = directory.ownedFiles[index].Close()
	}
	if directory.file != nil {
		_ = directory.file.Close()
	}
}

func (directory *heldTraceDirectory) bindingsStable() bool {
	if directory == nil {
		return false
	}
	for _, binding := range directory.bindings {
		info, identity, reparse, err := traceDirectoryBindingLstat(binding)
		if err != nil || reparse || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || identity != binding.identity {
			return false
		}
	}
	return true
}

func (directory *heldTraceDirectory) lstat(name string) (os.FileInfo, FileIdentity, bool, error) {
	return traceDirectoryChildLstat(directory, name)
}
