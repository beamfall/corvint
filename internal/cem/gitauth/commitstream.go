package gitauth

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

type streamedObject struct {
	oid, kind, tree string
	missing         bool
}

// commitStream hashes leading batch records without retaining their bodies.
// Only the original trailing tree batch consumes the existing byte allowance.
type commitStream struct {
	count     int
	requests  []string
	width     int
	objects   []streamedObject
	header    []byte
	firstLine []byte
	current   streamedObject
	sum       hash.Hash
	remaining int64
	phase     byte
	tail      []byte
}

func (s *commitStream) Write(data []byte) (int, error) {
	total := len(data)
	for len(data) > 0 {
		if len(s.objects) == s.count {
			if len(data) > MaxTreeBytes-len(s.tail) {
				return total - len(data), cemcode.New(cemcode.GitOutputExceeded, "Git stdout exceeded its byte bound")
			}
			s.tail = append(s.tail, data...)
			return total, nil
		}
		var err error
		data, err = s.consume(data)
		if err != nil {
			return total - len(data), err
		}
	}
	return total, nil
}

func (s *commitStream) consume(data []byte) ([]byte, error) {
	switch s.phase {
	case 0:
		end := bytes.IndexByte(data, '\n')
		n := len(data)
		if end >= 0 {
			n = end + 1
		}
		// A missing record echoes a revision (up to 256 bytes) plus its
		// peel suffix; the framing bound must accommodate that echo.
		if len(s.header)+n > 512 {
			return data, unavailable("cat-file batch header exceeds its byte bound")
		}
		s.header = append(s.header, data[:n]...)
		if end < 0 {
			return data[n:], nil
		}
		return data[n:], s.beginObject()
	case 1:
		n := int64(len(data))
		if n > s.remaining {
			n = s.remaining
		}
		part := data[:int(n)]
		s.sum.Write(part)
		if err := s.keepTreeHeader(part); err != nil {
			return data, err
		}
		s.remaining -= n
		if s.remaining == 0 {
			s.phase = 2
		}
		return data[int(n):], nil
	default:
		if data[0] != '\n' {
			return data, unavailable("cat-file batch body separator is malformed")
		}
		return data[1:], s.endObject()
	}
}

func (s *commitStream) beginObject() error {
	header := s.header[:len(s.header)-1]
	s.header = s.header[:0]
	if bytes.HasSuffix(header, []byte(" missing")) {
		if string(header) != s.requests[len(s.objects)]+" missing" {
			return unavailable("cat-file missing record differs from its request")
		}
		s.objects = append(s.objects, streamedObject{missing: true})
		return nil
	}
	fields := strings.Fields(string(header))
	if len(fields) != 3 || !wire.IsGitOid(fields[0]) {
		return unavailable("cat-file batch header is malformed")
	}
	if fields[1] != "commit" && fields[1] != "tree" {
		return unavailable("cat-file batch object is not a commit or tree")
	}
	if s.width != 0 && len(fields[0]) != s.width {
		return unavailable("cat-file batch object format differs")
	}
	s.width = len(fields[0])
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 || strconv.FormatInt(size, 10) != fields[2] {
		return unavailable("cat-file batch size is malformed")
	}
	s.current = streamedObject{oid: fields[0], kind: fields[1]}
	s.sum = sha1.New()
	if s.width == 64 {
		s.sum = sha256.New()
	}
	s.sum.Write([]byte(fields[1] + " " + fields[2] + "\x00"))
	s.firstLine = s.firstLine[:0]
	s.remaining = size
	s.phase = 1
	if size == 0 {
		s.phase = 2
	}
	return nil
}

func (s *commitStream) keepTreeHeader(part []byte) error {
	if s.current.kind != "commit" || bytes.HasSuffix(s.firstLine, []byte{'\n'}) {
		return nil
	}
	if end := bytes.IndexByte(part, '\n'); end >= 0 {
		part = part[:end+1]
	}
	if len(s.firstLine)+len(part) > s.width+6 {
		return unavailable("commit tree header is malformed")
	}
	s.firstLine = append(s.firstLine, part...)
	return nil
}

func (s *commitStream) endObject() error {
	if hex.EncodeToString(s.sum.Sum(nil)) != s.current.oid {
		return unavailable("%s content does not match its object identity", s.current.kind)
	}
	if s.current.kind == "commit" {
		line := string(s.firstLine)
		if len(line) != s.width+6 || !strings.HasPrefix(line, "tree ") || !strings.HasSuffix(line, "\n") {
			return unavailable("commit tree header is malformed")
		}
		s.current.tree = line[5 : len(line)-1]
		if !wire.IsGitOid(s.current.tree) {
			return unavailable("commit tree identity is malformed")
		}
	}
	s.objects = append(s.objects, s.current)
	s.phase = 0
	return nil
}

func (s *commitStream) finish() error {
	if len(s.objects) != s.count || s.phase != 0 || len(s.header) != 0 {
		return unavailable("cat-file batch output is truncated")
	}
	return nil
}

func (r *Repository) streamCommitBatch(ctx context.Context, prefixes, trees []string) (*commitStream, error) {
	requests := append(append([]string{}, prefixes...), trees...)
	stdin := []byte(strings.Join(requests, "\x00") + "\x00")
	stream := &commitStream{count: len(prefixes), requests: prefixes}
	if r.ObjectFormat == "sha1" {
		stream.width = 40
	}
	if r.ObjectFormat == "sha256" {
		stream.width = 64
	}
	env := scrubbedEnv()
	if r.objectView != nil {
		env = append(env, "GIT_OBJECT_DIRECTORY="+filepath.Join(r.objectView.CommonDir, "objects"), "GIT_ALTERNATE_OBJECT_DIRECTORIES=")
	}
	options := gitrun.Options{Dir: r.Root, Env: env, Stdin: stdin}
	args := append(r.pinnedArgs(), "cat-file", "--batch", "-z")
	if err := gitrun.RunStream(ctx, r.budget, options, stream, args...); err != nil {
		return nil, err
	}
	if err := stream.finish(); err != nil {
		return nil, err
	}
	return stream, nil
}

func requireCommitTree(commit streamedObject, rootOID string) error {
	// A missing commit peel preserves the existing direct-tree profile.
	// Ref/tag selection remains Git-owned; a present commit never falls back.
	if commit.missing {
		return nil
	}
	if commit.kind != "commit" || commit.tree != rootOID {
		return unavailable("root tree differs from its verified commit")
	}
	return nil
}
