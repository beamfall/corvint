// Package coverprofile is the Go coverprofile grammar shared by the bounded
// runner (internal/liveverify/gorunner) and `cem cover` (TCQ-V0-051). It
// depends only on the standard library, so the CEM seams' closure stays the
// standard library and internal/cem.
package coverprofile

import (
	"bufio"
	"bytes"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MaxBytes bounds one coverprofile.
const MaxBytes = int64(256 << 20)

// Mode is a coverprofile's `mode:` header value.
type Mode string

const (
	ModeSet    Mode = "set"
	ModeCount  Mode = "count"
	ModeAtomic Mode = "atomic"
)

// Block is one parsed coverprofile block line.
type Block struct {
	ProfilePath string
	StartLine   uint64
	StartColumn uint64
	EndLine     uint64
	EndColumn   uint64
	Statements  uint64
	Count       uint64
}

// Parse parses one local coverprofile without a source mapping: the mode
// header, then every block line under the runner's block grammar. It applies
// no file mapping, duplicate, or size rule; the runner's capture path keeps
// those. TCQ-V0-051 consumes it for patch-coverage witnesses.
func Parse(raw []byte) (Mode, []Block, error) {
	if len(raw) == 0 || !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		return "", nil, errors.New("malformed coverage profile")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "mode: ") {
		return "", nil, errors.New("coverage mode is absent")
	}
	mode := Mode(strings.TrimPrefix(scanner.Text(), "mode: "))
	if mode != ModeSet && mode != ModeCount && mode != ModeAtomic {
		return "", nil, errors.New("coverage mode is invalid")
	}
	var blocks []Block
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode: ") {
			return "", nil, errors.New("mixed coverage mode")
		}
		block, err := ParseBlockLine(line, mode)
		if err != nil {
			return "", nil, err
		}
		blocks = append(blocks, block)
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	return mode, blocks, nil
}

// ParseBlockLine parses one coverprofile block line under mode with the
// grammar the runner applies to its own captured profile.
func ParseBlockLine(line string, mode Mode) (Block, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 || strings.ContainsAny(line, "\r\t") {
		return Block{}, errors.New("malformed coverage block")
	}
	colon := strings.LastIndexByte(fields[0], ':')
	if colon <= 0 {
		return Block{}, errors.New("coverage location is absent")
	}
	profilePath := fields[0][:colon]
	rangeParts := strings.Split(fields[0][colon+1:], ",")
	if len(rangeParts) != 2 {
		return Block{}, errors.New("coverage range is malformed")
	}
	startLine, startColumn, err := parsePosition(rangeParts[0])
	if err != nil {
		return Block{}, err
	}
	endLine, endColumn, err := parsePosition(rangeParts[1])
	if err != nil || endLine < startLine || endLine == startLine && endColumn <= startColumn {
		return Block{}, errors.New("coverage range is invalid")
	}
	statements, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil || statements == 0 {
		return Block{}, errors.New("coverage statement count is invalid")
	}
	count, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil || mode == ModeSet && count > 1 {
		return Block{}, errors.New("coverage counter is invalid")
	}
	return Block{ProfilePath: profilePath, StartLine: startLine, StartColumn: startColumn,
		EndLine: endLine, EndColumn: endColumn, Statements: statements, Count: count}, nil
}

func parsePosition(value string) (uint64, uint64, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return 0, 0, errors.New("coverage position is malformed")
	}
	line, lineErr := strconv.ParseUint(parts[0], 10, 32)
	column, columnErr := strconv.ParseUint(parts[1], 10, 32)
	if lineErr != nil || columnErr != nil || line == 0 || column == 0 {
		return 0, 0, errors.New("coverage position is invalid")
	}
	return line, column, nil
}
