package dogfoodflow

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// blobBatch reads blobs through one long-lived `git cat-file --batch`, and
// commitTree holds each commit's whole `ls-tree -r` listing, so a draft reads
// a blob with no Git spawn instead of an ls-tree and a cat-file per blob: a
// 593-hunk draft ran 2639 Git commands, 2625 of them for blobs, and now 20
// (V1-2360, DCW-V0-752). Either read failing falls back to the per-blob
// commands, so the result is the same.
type blobBatch struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	broken  bool
}

// treeEntry is one regular-file or other entry of a commit's tree.
type treeEntry struct {
	mode, kind, object string
}

// commitTree returns commit's tree entries by path, or false when the listing
// cannot be read within the flow's Git output bound.
func (p *proposal) commitTree(commit string) (map[string]treeEntry, bool) {
	if entries, ok := p.trees[commit]; ok {
		return entries, entries != nil
	}
	if p.trees == nil {
		p.trees = map[string]map[string]treeEntry{}
	}
	listing, status, bounded := p.boundedGit("ls-tree", "-r", "-z", "--full-tree", commit)
	if status != 0 || !bounded {
		p.trees[commit] = nil
		return nil, false
	}
	entries := map[string]treeEntry{}
	for _, record := range strings.Split(listing, "\x00") {
		meta, name, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			continue
		}
		entries[name] = treeEntry{mode: fields[0], kind: fields[1], object: fields[2]}
	}
	p.trees[commit] = entries
	return entries, true
}

// batchBlob reads object through the long-lived cat-file process, reporting
// false when the process is unavailable or its answer is not a blob of a
// readable size; the caller then reads it the per-blob way.
func (p *proposal) batchBlob(object string) ([]byte, bool) {
	p.checkpoint()
	if p.batch == nil {
		p.batch = &blobBatch{}
		command := p.gitCommand(true, "cat-file", "--batch")
		stdin, err := command.StdinPipe()
		if err == nil {
			var stdout io.ReadCloser
			if stdout, err = command.StdoutPipe(); err == nil {
				err = command.Start()
				p.batch.stdout = bufio.NewReader(stdout)
			}
		}
		if err != nil {
			p.batch.broken = true
			return nil, false
		}
		p.batch.command, p.batch.stdin = command, stdin
	}
	batch := p.batch
	if batch.broken {
		return nil, false
	}
	if _, err := fmt.Fprintf(batch.stdin, "%s\n", object); err != nil {
		batch.broken = true
		return nil, false
	}
	header, err := batch.stdout.ReadString('\n')
	fields := strings.Fields(header)
	if err != nil || len(fields) != 3 || fields[0] != object || fields[1] != "blob" {
		// A missing object or an unexpected answer leaves the stream's
		// position unknown, so the process is not used again.
		batch.broken = true
		return nil, false
	}
	size, err := strconv.Atoi(fields[2])
	limit := p.gitOutputLimit
	if limit <= 0 {
		limit = maxGitOutputBytes
	}
	if err != nil || size < 0 {
		batch.broken = true
		return nil, false
	}
	if size > limit {
		p.refuse(gitOutputOverBound)
	}
	data := make([]byte, size+1)
	if _, err := io.ReadFull(batch.stdout, data); err != nil || data[size] != '\n' {
		batch.broken = true
		return nil, false
	}
	return data[:size], true
}

// closeBlobs ends the cat-file process, if one was started.
func (p *proposal) closeBlobs() {
	if p.batch == nil || p.batch.command == nil {
		return
	}
	_ = p.batch.stdin.Close()
	_ = p.batch.command.Wait()
	p.batch = nil
}

// blobFastPath turns fastBlob on; a test turns it off to compare the
// drafts of both read paths (DCW-V0-752).
var blobFastPath = true

// fastBlob is blob's read through the commit listing and the batch process;
// handled is false when either is unavailable.
func (p *proposal) fastBlob(commit, name string) (data []byte, found, handled bool) {
	if !blobFastPath {
		return nil, false, false
	}
	entries, ok := p.commitTree(commit)
	if !ok {
		return nil, false, false
	}
	entry, ok := entries[name]
	if !ok || !strings.HasPrefix(entry.mode, "100") || entry.kind != "blob" {
		return nil, false, true
	}
	data, ok = p.batchBlob(entry.object)
	if !ok {
		return nil, false, false
	}
	return data, true, true
}
