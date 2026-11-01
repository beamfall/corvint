package dogfoodflow

import (
	"bufio"
	"io"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

// batchBlobReads turns the batch reader on for drafts and records; a test
// turns it off to compare the per-file reads (CKC-V0-015).
var batchBlobReads = true

// blobBatch is the one long-lived `git cat-file --batch` reader through which
// a draft or record reads each blob it has not prefetched, in place of an
// ls-tree and a cat-file process a blob (CKC-V0-015).
type blobBatch struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	failed  bool
}

// batchBlob reads name at commit through the proposal's batch reader. ok is
// false when the reader cannot answer exactly as blob's ls-tree and cat-file
// would: batching is off, the name or commit is not a clean literal, the
// reader failed or never started, or the blob is past the flow's Git output
// bound. blob then reads it the per-file way, which keeps that read's refusal.
func (p *proposal) batchBlob(commit, name string) (data []byte, present, ok bool) {
	if !p.batchBlobs || commit == "" || strings.ContainsAny(commit, ": \t\n\x00") || name == "" || name == "." || path.Clean(name) != name || path.IsAbs(name) ||
		strings.HasPrefix(name, "../") || strings.ContainsAny(name, "*?[]\\:\x00\n") {
		return nil, false, false
	}
	if p.batch == nil {
		p.batch = &blobBatch{}
		p.checkpoint()
		// %(objectmode) is the tree entry's mode, so a symlink or submodule
		// reads as absent, as blob's ls-tree check has it. A Git without the
		// atom fails the first read, and every read falls back.
		command := p.gitCommand(true, "cat-file", "--batch=%(objectmode) %(objecttype) %(objectsize)")
		stdin, err := command.StdinPipe()
		if err != nil {
			p.batch.failed = true
			return nil, false, false
		}
		stdout, err := command.StdoutPipe()
		if err != nil || command.Start() != nil {
			p.batch.failed = true
			return nil, false, false
		}
		p.batch.command, p.batch.stdin, p.batch.stdout = command, stdin, bufio.NewReader(stdout)
	}
	if p.batch.failed {
		return nil, false, false
	}
	p.checkpoint()
	request := commit + ":" + name
	if _, err := io.WriteString(p.batch.stdin, request+"\n"); err != nil {
		p.stopBlobBatch()
		return nil, false, false
	}
	header, err := p.batch.stdout.ReadString('\n')
	if err != nil {
		p.stopBlobBatch()
		return nil, false, false
	}
	header = strings.TrimSuffix(header, "\n")
	if header == request+" missing" || header == request+" ambiguous" {
		return nil, false, true
	}
	fields := strings.Fields(header)
	if len(fields) != 3 {
		p.stopBlobBatch()
		return nil, false, false
	}
	size, err := strconv.Atoi(fields[2])
	limit := p.gitOutputLimit
	if limit <= 0 {
		limit = maxGitOutputBytes
	}
	regular := strings.HasPrefix(fields[0], "100") && fields[1] == "blob"
	if err != nil || size < 0 || regular && size > limit {
		// cat-file blob would refuse this read past the bound; blob makes it.
		p.stopBlobBatch()
		return nil, false, false
	}
	content := make([]byte, size+1)
	if _, err := io.ReadFull(p.batch.stdout, content); err != nil || content[size] != '\n' {
		p.stopBlobBatch()
		return nil, false, false
	}
	if !regular {
		return nil, false, true
	}
	return content[:size:size], true, true
}

// stopBlobBatch ends a failed reader; later reads fall back to blob's
// per-file reads.
func (p *proposal) stopBlobBatch() {
	if p.batch == nil || p.batch.command == nil {
		return
	}
	p.batch.failed = true
	_ = p.batch.stdin.Close()
	_ = p.batch.command.Process.Kill()
	_ = p.batch.command.Wait()
	p.batch.command = nil
}

// closeBlobBatch ends the reader at the flow's boundary, so no Git process
// outlives the command.
func (p *proposal) closeBlobBatch() {
	if p.batch == nil || p.batch.command == nil {
		return
	}
	_ = p.batch.stdin.Close()
	_ = p.batch.command.Wait()
	p.batch.command = nil
}
