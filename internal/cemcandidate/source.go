package cemcandidate

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/publish"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// Read uses the existing bounded no-follow CEM reader. Parent components are
// checked against a publication root; the selected filename cannot be Git metadata.
func Read(name string) ([]byte, error) {
	absolute, e := filepath.Abs(name)
	if e != nil {
		return nil, e
	}
	volume := filepath.VolumeName(absolute)
	relative := strings.TrimPrefix(filepath.ToSlash(absolute[len(volume):]), "/")
	if !literal(relative) {
		return nil, fmt.Errorf("unsafe input path")
	}
	root, e := publish.OpenRoot(volume + string(filepath.Separator))
	if e != nil {
		return nil, e
	}
	return root.ReadBounded(relative, MaxDocument, "candidate-input")
}
func readPinned(ctx context.Context, p FileRef) ([]byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	raw, e := Read(p.Path)
	if e != nil {
		return nil, e
	}
	if tr.Digest(raw) != p.Sha256 {
		return nil, fmt.Errorf("input digest changed: %s", p.Path)
	}
	return raw, nil
}
