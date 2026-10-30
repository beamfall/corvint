// Package archivepath confines archive member names to an extraction root.
//
// It is the one shared lexical containment check for code that writes archive
// members to disk (V1-3179). It does not resolve links: an extractor that also
// creates symbolic links must resolve them separately before writing.
package archivepath

import (
	"fmt"
	"path/filepath"
)

// Join returns root joined with name, a slash-separated archive member name.
// It refuses a name that filepath.IsLocal rejects for this platform: an empty
// or absolute name, one with a ".." element that leaves root, or (on Windows)
// a volume, rooted or reserved device name.
func Join(root, name string) (string, error) {
	native := filepath.FromSlash(name)
	if !filepath.IsLocal(native) {
		return "", fmt.Errorf("archive member %q is not a local path", name)
	}
	return filepath.Join(root, native), nil
}
