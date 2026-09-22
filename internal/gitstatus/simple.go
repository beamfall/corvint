package gitstatus

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"strings"
)

// simpleConfig recognizes only unquoted, single-line settings in the sections
// `git clone` and `git worktree` write: core, user, the worktreeConfig
// extension, and the fetch/push keys of remote and branch subsections. Any
// other syntax goes to Git's parser over the private copy, never a live config.
// An explicit format version zero proves the index uses SHA-1 in this subset:
// no other extension, no include and no continuation is admitted here, and a
// quote appears only as the delimiter of a remote or branch name.
func simpleConfig(data []byte) (safe, sha1Format bool) {
	if bytes.ContainsAny(data, "\\#;\r\x00") {
		return false, false
	}
	section := ""
	for line := range bytes.SplitSeq(data, []byte{'\n'}) {
		line = bytes.Trim(line, " \t")
		if len(line) == 0 {
			continue
		}
		text := strings.ToLower(string(line))
		if header, ok := simpleSection(text); ok {
			section = header
			continue
		}
		if strings.Contains(text, "\"") {
			return false, false
		}
		key, value, found := strings.Cut(text, "=")
		if !found {
			return false, false
		}
		key, value = strings.Trim(key, " \t"), strings.Trim(value, " \t")
		switch section + "." + key {
		case "[user].name", "[user].email":
			// Values have no quoting, escapes, comments or physical newlines.
		case "[remote].url", "[remote].pushurl", "[remote].fetch", "[remote].push", "[remote].tagopt",
			"[branch].remote", "[branch].merge", "[branch].rebase", "[branch].pushremote":
			// Remote and upstream settings; a porcelain status without --branch
		// never reads them, and Git reads them from the private copy anyway.
		case "[core].repositoryformatversion":
			if value != "0" {
				return false, false
			}
			sha1Format = true
		case "[core].bare":
			if !simpleFalse(value) {
				return false, false
			}
		case "[core].filemode", "[core].logallrefupdates", "[core].ignorecase", "[core].precomposeunicode",
			"[extensions].worktreeconfig":
			if !simpleFalse(value) && !simpleTrue(value) {
				return false, false
			}
		default:
			return false, false
		}
	}
	return true, sha1Format
}

// simpleSection admits the plain [core], [user] and [extensions] headers and a
// remote or branch header whose quoted name is nonempty and carries no quote.
func simpleSection(text string) (string, bool) {
	switch text {
	case "[core]", "[user]", "[extensions]":
		return text, true
	}
	for _, name := range []string{"remote", "branch"} {
		inner, found := strings.CutPrefix(text, "["+name+" \"")
		if !found {
			continue
		}
		subsection, found := strings.CutSuffix(inner, "\"]")
		if !found || subsection == "" || strings.Contains(subsection, "\"") {
			return "", false
		}
		return "[" + name + "]", true
	}
	return "", false
}

func simpleFalse(value string) bool {
	return value == "false" || value == "no" || value == "off" || value == "0"
}

func simpleTrue(value string) bool {
	return value == "true" || value == "yes" || value == "on" || value == "1"
}

// simpleIndex recognizes complete SHA-1 v2/v3 framing without gitlinks or a
// split index. Unknown versions/extensions and malformed input keep the existing
// private Git probes. Git remains the validator of the index's contents.
func simpleIndex(data []byte) bool {
	if len(data) < 12+sha1.Size || !bytes.Equal(data[:4], []byte("DIRC")) {
		return false
	}
	version := binary.BigEndian.Uint32(data[4:8])
	if version != 2 && version != 3 {
		return false
	}
	end := len(data) - sha1.Size
	digest := sha1.Sum(data[:end])
	if !bytes.Equal(data[end:], digest[:]) {
		return false
	}
	remaining := data[12:end]
	for count := binary.BigEndian.Uint32(data[8:12]); count > 0; count-- {
		length := simpleEntryLength(remaining, version)
		if length == 0 {
			return false
		}
		remaining = remaining[length:]
	}
	for len(remaining) > 0 {
		if len(remaining) < 8 {
			return false
		}
		switch string(remaining[:4]) {
		case "TREE", "REUC", "UNTR", "FSMN", "EOIE", "IEOT":
		default:
			return false
		}
		length := uint64(binary.BigEndian.Uint32(remaining[4:8])) + 8
		if length > uint64(len(remaining)) {
			return false
		}
		remaining = remaining[int(length):]
	}
	return true
}

func simpleEntryLength(data []byte, version uint32) int {
	if len(data) < 62 {
		return 0
	}
	switch binary.BigEndian.Uint32(data[24:28]) {
	case 0o100644, 0o100755, 0o120000:
	default:
		return 0
	}
	flags := binary.BigEndian.Uint16(data[60:62])
	start := 62
	if flags&0x4000 != 0 {
		if version != 3 || len(data) < 64 || binary.BigEndian.Uint16(data[62:64]) & ^uint16(0x6000) != 0 {
			return 0
		}
		start = 64
	}
	nameLength := bytes.IndexByte(data[start:], 0)
	if nameLength < 1 || int(flags&0xfff) != min(nameLength, 0xfff) {
		return 0
	}
	length := (start + nameLength + 8) & ^7
	if length > len(data) {
		return 0
	}
	for _, value := range data[start+nameLength : length] {
		if value != 0 {
			return 0
		}
	}
	return length
}
