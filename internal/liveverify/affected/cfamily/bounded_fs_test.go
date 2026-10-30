package cfamily

import "io/fs"

// boundedDirFS gives a directory filesystem the bounded open an immutable
// affected source requires (DLT-V0-002), as the revision filesystem does.
type boundedDirFS struct{ fs.FS }

func (b boundedDirFS) OpenBounded(name string, limit int) (fs.File, error) {
	info, err := fs.Stat(b.FS, name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, fs.ErrInvalid
	}
	return b.FS.Open(name)
}
