package worksource

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type treeChild struct{ name, mode, oid string }

func (source *Source) cacheObjects(ctx context.Context) error {
	directories := map[string][]treeChild{"": {}}
	for _, entry := range source.Entries {
		source.objects[entry.BlobOID] = object{"blob", entry.Raw}
		directory := filepath.ToSlash(filepath.Dir(entry.Path))
		if directory == "." {
			directory = ""
		}
		directories[directory] = append(directories[directory], treeChild{filepath.Base(entry.Path), entry.Mode, entry.BlobOID})
		for parent := directory; parent != ""; {
			next := filepath.ToSlash(filepath.Dir(parent))
			if next == "." {
				next = ""
			}
			if _, ok := directories[next]; !ok {
				directories[next] = nil
			}
			parent = next
		}
	}
	names := make([]string, 0, len(directories))
	for name := range directories {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, directory := range names {
		children := directories[directory]
		sort.Slice(children, func(i, j int) bool {
			a, b := children[i].name, children[j].name
			if children[i].mode == "40000" {
				a += "/"
			}
			if children[j].mode == "40000" {
				b += "/"
			}
			return a < b
		})
		var raw bytes.Buffer
		for _, child := range children {
			fmt.Fprintf(&raw, "%s %s%c", child.mode, child.name, 0)
			oid, _ := hex.DecodeString(child.oid)
			raw.Write(oid)
		}
		oid := objectID(source.Identity.ObjectFormat, "tree", raw.Bytes())
		source.objects[oid] = object{"tree", bytes.Clone(raw.Bytes())}
		if directory == "" {
			if oid != source.Identity.Tree {
				return errors.New("reconstructed target tree differs")
			}
			continue
		}
		parent := filepath.ToSlash(filepath.Dir(directory))
		if parent == "." {
			parent = ""
		}
		directories[parent] = append(directories[parent], treeChild{filepath.Base(directory), "40000", oid})
	}
	commit, err := source.Git(ctx, 4<<20, "cat-file", "commit", source.Identity.Commit)
	if err != nil {
		return err
	}
	if objectID(source.Identity.ObjectFormat, "commit", commit) != source.Identity.Commit || !bytes.HasPrefix(commit, []byte("tree "+source.Identity.Tree+"\n")) {
		return errors.New("invalid pinned commit")
	}
	source.objects[source.Identity.Commit] = object{"commit", commit}
	return nil
}

// Object returns only content cached and hash-verified during acquisition.
func (source *Source) Object(ctx context.Context, oid string) (string, []byte, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	value, ok := source.objects[oid]
	if !ok {
		return "", nil, errors.New("object outside acquired target closure")
	}
	return value.kind, bytes.Clone(value.raw), nil
}

// ExportObjects writes standalone loose objects to private destination metadata.
// It exports the exact target closure, not parent history or caller configuration.
func (source *Source) ExportObjects(ctx context.Context, destinationGitDir string) error {
	destination, err := filepath.Abs(destinationGitDir)
	if err != nil {
		return err
	}
	for _, caller := range []string{source.Root, source.GitDir, source.CommonDir} {
		if destination == caller || strings.HasPrefix(destination, caller+string(os.PathSeparator)) {
			return errors.New("object export cannot write caller")
		}
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll("objects", 0700); err != nil {
		return err
	}
	for oid, value := range source.objects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if objectID(source.Identity.ObjectFormat, value.kind, value.raw) != oid {
			return errors.New("cached object drift")
		}
		directory := "objects/" + oid[:2]
		if err := root.MkdirAll(directory, 0700); err != nil {
			return err
		}
		output, err := root.OpenFile(directory+"/"+oid[2:], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0444)
		if err != nil {
			return err
		}
		compressor := zlib.NewWriter(output)
		_, headerErr := fmt.Fprintf(compressor, "%s %d%c", value.kind, len(value.raw), 0)
		_, writeErr := compressor.Write(value.raw)
		closeErr := compressor.Close()
		fileErr := output.Close()
		if err := errors.Join(headerErr, writeErr, closeErr, fileErr); err != nil {
			return err
		}
	}
	return nil
}
