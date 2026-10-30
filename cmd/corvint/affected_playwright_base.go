package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

const (
	playwrightBaseMaxBytes = 1 << 20
	playwrightBaseMaxPaths = 64
)

var errPlaywrightBaseUnread = errors.New("comparison base content was not read")

type playwrightBaseBlob struct {
	body   []byte
	exists bool
}

// playwrightBaseContent reads, in one bounded `git cat-file --batch`, the blob of each owned
// dirty path at the comparison base: the --base commit when given, else HEAD. The reader it
// returns errors for a path it did not read, so that path's changed symbols stay UNRESOLVED
// (TJAA-V0-033).
func playwrightBaseContent(ctx context.Context, gitExecutable, root, base string, dirty []string) typescript.PlaywrightBaseReader {
	read := map[string]playwrightBaseBlob{}
	reader := func(relative string) ([]byte, bool, error) {
		entry, ok := read[relative]
		if !ok {
			return nil, false, errPlaywrightBaseUnread
		}
		return entry.body, entry.exists, nil
	}
	language := typescript.New()
	paths := []string{}
	for _, relative := range dirty {
		if language.Owns(relative) && !strings.ContainsAny(relative, "\r\n") && len(paths) < playwrightBaseMaxPaths {
			paths = append(paths, relative)
		}
	}
	if len(paths) == 0 {
		return reader
	}
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	defer cancel()
	var request strings.Builder
	for _, relative := range paths {
		request.WriteString(base + ":" + relative + "\n")
	}
	command := hermeticGitCommand(deadline, gitExecutable, root, "cat-file", "--batch")
	command.Stdin = strings.NewReader(request.String())
	stdout, err := command.StdoutPipe()
	if err != nil || command.Start() != nil {
		return reader
	}
	parsed := map[string]playwrightBaseBlob{}
	output := bufio.NewReader(stdout)
	for _, relative := range paths {
		entry, keep, err := readPlaywrightBaseBlob(output)
		if err != nil {
			break
		}
		if keep {
			parsed[relative] = entry
		}
	}
	_, _ = io.Copy(io.Discard, stdout)
	if command.Wait() == nil {
		read = parsed
	}
	return reader
}

// readPlaywrightBaseBlob reads one `cat-file --batch` response. keep is false for an object that
// is not a blob or is over the bound; its bytes are discarded.
func readPlaywrightBaseBlob(output *bufio.Reader) (playwrightBaseBlob, bool, error) {
	var entry playwrightBaseBlob
	header, err := output.ReadString('\n')
	if err != nil {
		return entry, false, err
	}
	if strings.HasSuffix(header, " missing\n") {
		return entry, true, nil
	}
	fields := strings.Fields(header)
	if len(fields) != 3 {
		return entry, false, fmt.Errorf("unexpected cat-file header %q", header)
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 {
		return entry, false, fmt.Errorf("unexpected cat-file size %q", fields[2])
	}
	if fields[1] != "blob" || size > playwrightBaseMaxBytes {
		if _, err := io.CopyN(io.Discard, output, size+1); err != nil {
			return entry, false, err
		}
		return entry, false, nil
	}
	body := make([]byte, size+1)
	if _, err := io.ReadFull(output, body); err != nil {
		return entry, false, err
	}
	entry.body, entry.exists = body[:size], true
	return entry, true, nil
}
