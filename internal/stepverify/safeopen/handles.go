// SPDX-License-Identifier: AGPL-3.0-or-later

package safeopen

import "os"

// Directory retains a pinned root without exposing filesystem mutators.
type Directory struct {
	root *os.Root
}

func (d *Directory) Close() error                          { return d.root.Close() }
func (d *Directory) Stat(name string) (os.FileInfo, error) { return d.root.Stat(name) }
func (d *Directory) Lstat(name string) (os.FileInfo, error) {
	return d.root.Lstat(name)
}
func (d *Directory) Readlink(name string) (string, error) { return d.root.Readlink(name) }

// File exposes only observation operations on a descriptor opened O_RDONLY.
type File struct {
	file *os.File
}

func (f *File) Close() error                         { return f.file.Close() }
func (f *File) Stat() (os.FileInfo, error)           { return f.file.Stat() }
func (f *File) Read(p []byte) (int, error)           { return f.file.Read(p) }
func (f *File) ReadDir(n int) ([]os.DirEntry, error) { return f.file.ReadDir(n) }
