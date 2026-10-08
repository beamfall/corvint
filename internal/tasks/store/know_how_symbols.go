package store

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// knowHowSymbolMaxBlobBytes bounds the blob a symbol anchor is resolved in.
// A larger file is UNKNOWN at read time and refused at write time, so one
// read never parses an unbounded source (KHN-V0-017).
const knowHowSymbolMaxBlobBytes = 1 << 20

// knowHowSymbols is one source's declarations as the index's own extractor
// names them (KHN-V0-016): the SHA-256 of each uniquely named declaration's
// text, with "" for a name declared more than once. refusal is non-empty
// when the file has no extractor or the extractor refused or truncated it.
type knowHowSymbols struct {
	path    string
	digests map[string]string
	refusal string
}

// knowHowSymbolTable reuses contextindex.SymbolExtents; it adds no parser.
func knowHowSymbolTable(path string, data []byte) knowHowSymbols {
	t := knowHowSymbols{path: path, digests: map[string]string{}}
	extents, ok := contextindex.SymbolExtents(path, data)
	if !ok {
		t.refusal = "no symbol extractor admits " + path
		return t
	}
	for _, e := range extents {
		if _, seen := t.digests[e.Name]; seen {
			t.digests[e.Name] = ""
			continue
		}
		t.digests[e.Name] = string(wire.Sum([]byte(e.Content)))
	}
	return t
}

// digest is name's declaration digest, or "" with the reason it does not
// resolve to exactly one declaration.
func (t knowHowSymbols) digest(name string) (string, string) {
	if t.refusal != "" {
		return "", t.refusal
	}
	d, ok := t.digests[name]
	switch {
	case !ok:
		return "", "symbol " + name + " is not declared in " + t.path
	case d == "":
		return "", "symbol " + name + " is declared more than once in " + t.path
	}
	return d, ""
}

// knowHowChangedSymbols reads, with one `git cat-file --batch`, every HEAD
// blob that a symbol anchor's path names when it differs from the anchor's
// pin, and returns the declarations of each such path. A path whose blob was
// not read (too large, or a failed read) is absent, so its anchors stay
// UNKNOWN; nothing is read when no symbol anchor's file changed.
func knowHowChangedSymbols(root, head string, notes []KnowHowNote, objs []catFileObject, index map[string]int) map[string]knowHowSymbols {
	out := map[string]knowHowSymbols{}
	if head == "" {
		return out
	}
	var oids []string
	want := map[string]string{}
	for _, n := range notes {
		for _, a := range n.Entry.Anchors {
			o := objs[index[a.Path]]
			if a.Symbol == "" || o.kind != "blob" || o.oid == a.Blob {
				continue
			}
			if _, ok := want[a.Path]; !ok {
				want[a.Path] = o.oid
				oids = append(oids, o.oid)
			}
		}
	}
	contents, err := readKnowHowBlobs(root, oids)
	if err != nil {
		return out
	}
	for path, oid := range want {
		if data, ok := contents[oid]; ok {
			out[path] = knowHowSymbolTable(path, data)
		}
	}
	return out
}

// readKnowHowBlobs returns the content of each blob oid with one
// `git cat-file --batch` in root. A blob larger than
// knowHowSymbolMaxBlobBytes, a missing object or a non-blob is absent from
// the map; a Git failure or a short answer is an error. It writes nothing:
// the Git environment disables optional locks.
func readKnowHowBlobs(root string, oids []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	if len(oids) == 0 {
		return out, nil
	}
	if root == "" {
		return nil, wire.Errorf(wire.CodeUnsupported, "git", "no checkout to observe")
	}
	c := exec.Command("git", "-c", "credential.helper=", "cat-file", "--batch")
	c.Dir = root
	c.Env = gitEnvironment()
	stdin, err := c.StdinPipe()
	if err != nil {
		return nil, gitObservationFailed(err)
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, gitObservationFailed(err)
	}
	if err := c.Start(); err != nil {
		return nil, gitObservationFailed(err)
	}
	// The oids are written from a goroutine while the answers are read, so
	// neither pipe can fill and block the other.
	written := make(chan error, 1)
	go func() {
		_, err := io.WriteString(stdin, strings.Join(oids, "\n")+"\n")
		written <- errors.Join(err, stdin.Close())
	}()
	r := bufio.NewReader(stdout)
	var readErr error
	for range oids {
		if readErr = readKnowHowBlob(r, out); readErr != nil {
			break
		}
	}
	_, _ = io.Copy(io.Discard, r)
	werr := <-written
	if err := c.Wait(); readErr == nil && err != nil {
		readErr = gitObservationFailed(err)
	}
	if readErr == nil && werr != nil && !errors.Is(werr, os.ErrClosed) {
		readErr = gitObservationFailed(werr)
	}
	if readErr != nil {
		return nil, readErr
	}
	return out, nil
}

// readKnowHowBlob reads one `--batch` answer: "<oid> <type> <size>\n" and
// size bytes and a newline, or "<name> missing\n".
func readKnowHowBlob(r *bufio.Reader, out map[string][]byte) error {
	line, err := r.ReadString('\n')
	if err != nil {
		return wire.Errorf(wire.CodeUnsupported, "git", "git answered short: %v", err)
	}
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return nil // "<name> missing" or "<name> ambiguous": absent
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 {
		return wire.Errorf(wire.CodeUnsupported, "git", "git answered an unreadable object header %q", strings.TrimSpace(line))
	}
	if fields[1] != "blob" || size > knowHowSymbolMaxBlobBytes {
		if _, err := io.CopyN(io.Discard, r, size+1); err != nil {
			return wire.Errorf(wire.CodeUnsupported, "git", "git answered short: %v", err)
		}
		return nil
	}
	data := make([]byte, size+1)
	if _, err := io.ReadFull(r, data); err != nil {
		return wire.Errorf(wire.CodeUnsupported, "git", "git answered short: %v", err)
	}
	out[fields[0]] = data[:size]
	return nil
}
