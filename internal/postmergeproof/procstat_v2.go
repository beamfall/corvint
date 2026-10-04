// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof

import (
	"bytes"
	"errors"
	"strconv"
)

// procStatV2 holds the Linux /proc/PID/stat fields the birth proof consumes.
type procStatV2 struct {
	pid        int
	state      byte
	ppid       int
	startTicks uint64
}

// procStatMinFields is the Linux 3.5+ layout ending in exit_code;
// procStatStartField is starttime, the last field the birth key needs.
const (
	procStatMinFields  = 52
	procStatStartField = 22
)

// errStatLegacyLayout marks an otherwise well-formed stat from a kernel older
// than 3.5: an unsupported host rather than malformed evidence.
var errStatLegacyLayout = errors.New("stat has the pre-3.5 kernel layout")

// parseProcStatV2 parses raw /proc/PID/stat bytes without trimming: a decimal
// PID before ` (`, comm up to the final `) `, then single-space fields ending in
// exactly one LF. Field 3 is state, 4 PPID and 22 starttime clock ticks.
func parseProcStatV2(raw []byte) (procStatV2, error) {
	if len(raw) < 2 || raw[len(raw)-1] != '\n' {
		return procStatV2{}, errors.New("stat must end in one LF")
	}
	open := bytes.Index(raw, []byte(" ("))
	if open <= 0 {
		return procStatV2{}, errors.New("stat PID delimiter missing")
	}
	pid, ok := positiveDecimal(raw[:open])
	if !ok {
		return procStatV2{}, errors.New("stat PID is not a canonical positive decimal")
	}
	closing := bytes.LastIndex(raw, []byte(") "))
	if closing < open+1 {
		return procStatV2{}, errors.New("stat comm delimiter missing")
	}
	tail := raw[closing+2 : len(raw)-1]
	if bytes.IndexByte(tail, '\n') >= 0 {
		return procStatV2{}, errors.New("stat has more than one line")
	}
	fields := bytes.Split(tail, []byte(" "))
	if len(fields)+2 < procStatStartField {
		return procStatV2{}, errors.New("stat has too few fields")
	}
	for _, field := range fields {
		if len(field) == 0 {
			return procStatV2{}, errors.New("stat has an empty field")
		}
	}
	if len(fields[0]) != 1 || bytes.IndexByte([]byte("RSDZTtXxKWPI"), fields[0][0]) < 0 {
		return procStatV2{}, errors.New("stat state is unsupported")
	}
	ppid, ok := nonNegativeDecimal(fields[1])
	if !ok {
		return procStatV2{}, errors.New("stat PPID is invalid")
	}
	start, err := strconv.ParseUint(string(fields[19]), 10, 64)
	if err != nil || (len(fields[19]) > 1 && fields[19][0] == '0') {
		return procStatV2{}, errors.New("stat starttime is invalid")
	}
	if len(fields)+2 < procStatMinFields {
		return procStatV2{}, errStatLegacyLayout
	}
	return procStatV2{pid: pid, state: fields[0][0], ppid: ppid, startTicks: start}, nil
}

func positiveDecimal(b []byte) (int, bool) {
	v, ok := nonNegativeDecimal(b)
	return v, ok && v > 0
}

func nonNegativeDecimal(b []byte) (int, bool) {
	if len(b) == 0 || len(b) > 10 || (len(b) > 1 && b[0] == '0') {
		return 0, false
	}
	v := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + int(c-'0')
	}
	return v, true
}

func (s procStatV2) zombie() bool { return s.state == 'Z' || s.state == 'X' || s.state == 'x' }
