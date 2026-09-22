package repository

import (
	"bytes"
	"context"
	"sort"
	"strconv"
)

const (
	maxCatIDsPerChild = 50_000
	maxObjectBytes    = 16_777_216
)

var catFileTail = []string{"cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)"}

type objectCheck uint8

const (
	objectCheckQualified objectCheck = iota
	objectCheckInvalid
	objectCheckOversize
	objectCheckUnavailable
)

type objectExpectation struct {
	id           string
	expectedType string
}

func (authority *Authority) checkObjects(ctx context.Context, expectations []objectExpectation) map[string]objectCheck {
	result := make(map[string]objectCheck, len(expectations))
	pendingByID := make(map[string]string, len(expectations))
	for _, expectation := range expectations {
		if cached, exists := authority.objectChecks[expectation.id]; exists {
			result[expectation.id] = cached
			continue
		}
		pendingByID[expectation.id] = expectation.expectedType
	}
	pending := make([]objectExpectation, 0, len(pendingByID))
	for id, expectedType := range pendingByID {
		pending = append(pending, objectExpectation{id: id, expectedType: expectedType})
	}
	sort.Slice(pending, func(left, right int) bool { return pending[left].id < pending[right].id })

	for offset := 0; offset < len(pending); {
		end := catChunkEnd(pending, offset)
		if end == offset {
			authority.objectFailed = true
			markObjectBatch(authority.objectChecks, pending[offset:], objectCheckUnavailable)
			break
		}
		batch := pending[offset:end]
		input := catInput(batch)
		if !authority.budget.addCatInput(uint64(len(input))) {
			authority.objectFailed = true
			markObjectBatch(authority.objectChecks, batch, objectCheckUnavailable)
			offset = end
			continue
		}
		commandResult, failure := authority.runWithInput(
			ctx, authority.layout.worktree.path, nil, catFileTail,
			bytes.NewReader(input), maxCatChildBytes, true,
		)
		if failure != nil || commandResult.exit != 0 {
			markObjectBatch(authority.objectChecks, batch, objectCheckUnavailable)
		} else if parsed, ok := parseObjectChecks(commandResult.stdout, batch); ok {
			for id, check := range parsed {
				authority.objectChecks[id] = check
			}
		} else {
			markObjectBatch(authority.objectChecks, batch, objectCheckUnavailable)
		}
		offset = end
	}
	for _, expectation := range expectations {
		result[expectation.id] = authority.objectChecks[expectation.id]
	}
	return result
}

func catChunkEnd(values []objectExpectation, offset int) int {
	var inputBytes uint64
	end := offset
	for end < len(values) && end-offset < maxCatIDsPerChild {
		next := uint64(len(values[end].id) + 1)
		if inputBytes > maxCatChildBytes || next > maxCatChildBytes-inputBytes {
			break
		}
		inputBytes += next
		end++
	}
	return end
}

func catInput(values []objectExpectation) []byte {
	var size int
	for _, value := range values {
		size += len(value.id) + 1
	}
	result := make([]byte, 0, size)
	for _, value := range values {
		result = append(result, value.id...)
		result = append(result, '\n')
	}
	return result
}

func parseObjectChecks(raw []byte, expected []objectExpectation) (map[string]objectCheck, bool) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' ||
		bytes.Count(raw, []byte{'\n'}) != len(expected) || bytes.ContainsAny(raw, "\r\x00") {
		return nil, false
	}
	rows := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(rows) != len(expected) {
		return nil, false
	}
	result := make(map[string]objectCheck, len(expected))
	for index, row := range rows {
		if len(row) == 0 || !asciiField(row) {
			return nil, false
		}
		fields := bytes.Split(row, []byte{' '})
		if len(fields) == 2 && bytes.Equal(fields[0], []byte(expected[index].id)) &&
			(bytes.Equal(fields[1], []byte("missing")) || bytes.Equal(fields[1], []byte("ambiguous"))) {
			return nil, false
		}
		if len(fields) != 3 || !bytes.Equal(fields[0], []byte(expected[index].id)) ||
			len(fields[1]) == 0 || len(fields[2]) == 0 {
			return nil, false
		}
		if !bytes.Equal(fields[1], []byte(expected[index].expectedType)) {
			result[expected[index].id] = objectCheckInvalid
			continue
		}
		if len(fields[2]) > 1 && fields[2][0] == '0' {
			return nil, false
		}
		size, err := strconv.ParseUint(string(fields[2]), 10, 64)
		if err != nil {
			return nil, false
		}
		if size > maxObjectBytes {
			result[expected[index].id] = objectCheckOversize
		} else {
			result[expected[index].id] = objectCheckQualified
		}
	}
	return result, true
}

func markObjectBatch(target map[string]objectCheck, values []objectExpectation, check objectCheck) {
	for _, value := range values {
		target[value.id] = check
	}
}
