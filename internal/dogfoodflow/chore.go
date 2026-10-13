package dogfoodflow

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// ChoreRole classifies the paths a BASE..HEAD diff touches (git diff
// --no-renames --name-only, one repository-relative path each). A change is a
// chore only when it touches at least one path and every path is exactly
// .corvint/change.cem.json or .corvint/changes/<NAME>.cem.json with NAME one
// non-empty path segment. It returns wire.StableRoleSealChore when any
// .corvint/changes/ path is touched, wire.StableRoleBindChore when only
// .corvint/change.cem.json is, and "" for any other change (DCW-V0-048,
// DCW-V0-049). A path Git quotes never matches, so it is never a chore.
func ChoreRole(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	role := wire.StableRoleBindChore
	for _, path := range paths {
		switch {
		case path == ".corvint/change.cem.json":
		case choreChangesPath(path):
			role = wire.StableRoleSealChore
		default:
			return ""
		}
	}
	return role
}

func choreChangesPath(path string) bool {
	name, found := strings.CutPrefix(path, ".corvint/changes/")
	stem, sealed := strings.CutSuffix(name, ".cem.json")
	return found && sealed && stem != "" && !strings.Contains(stem, "/")
}

// choreRole classifies this run's BASE..HEAD diff (DCW-V0-048, DCW-V0-049).
func (c *change) choreRole() string {
	return ChoreRole(nonEmpty(strings.Split(c.gitValue("diff", "--no-renames", "--name-only", c.base, c.target), "\n")))
}

// uniformCitation reports a plan that cites one span for every hunk of a map
// with more than one hunk (DCW-V0-048): every hunk is named, by ordinal or full
// ID, by a citation row, and every citation row carries the same PATH and
// START:END, whatever its RELATION or the decimal spelling of START and END.
// It returns the hunk count and the span as
// PATH:START:END; a plan that keeps any hunk unknown or leaves one unnamed, or
// cites two spans, is not uniform. The caller exempts a chore change.
func uniformCitation(plan []byte, hunks []map[string]string) (int, string, bool) {
	if len(hunks) < 2 {
		return 0, "", false
	}
	cited := map[string]bool{}
	span := ""
	for _, line := range textLines(plan) {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 || unknownRow(fields) {
			return 0, "", false
		}
		rowSpan, ok := canonicalSpan(fields[1], fields[2])
		if !ok {
			return 0, "", false
		}
		if span != "" && rowSpan != span {
			return 0, "", false
		}
		span = rowSpan
		cited[fields[0]] = true
	}
	for index, hunk := range hunks {
		if !cited[strconv.Itoa(index+1)] && !cited[hunk["id"]] {
			return 0, "", false
		}
	}
	return len(hunks), span, true
}

// canonicalSpan names PATH:START:END with START and END as the decimal
// integers cem cite parses, so spellings such as 01:02 or +1:2 compare equal to
// 1:2; a span cem cite would refuse is not canonical.
func canonicalSpan(path, span string) (string, bool) {
	left, right, found := strings.Cut(span, ":")
	if !found {
		return "", false
	}
	start, startErr := strconv.ParseInt(left, 10, 64)
	end, endErr := strconv.ParseInt(right, 10, 64)
	if startErr != nil || endErr != nil {
		return "", false
	}
	return path + ":" + strconv.FormatInt(start, 10) + ":" + strconv.FormatInt(end, 10), true
}

// markChoreRole writes role into the regular cem/1.0 map at mapPath and keeps
// its file mode; a map that already carries role is left byte-identical. Only
// a map whose closed wire and reference graph parse is rewritten, through the
// stable encoder, so the written map parses again. It never writes role into a
// Core 0.x map, whose wire has no such member (DCW-V0-049, CEM-V1-095).
func markChoreRole(mapPath, role string) error {
	if isSymlink(mapPath) || !isRegular(mapPath) {
		return errors.New("stable map is not a regular file")
	}
	data, err := readPrefix(mapPath, wire.MaxMapBytes+1)
	if err != nil {
		return err
	}
	if !wire.DeclaresStable(data) {
		return errors.New("map does not declare cem/1.0")
	}
	document, err := wire.ParseStable(data)
	if err != nil {
		return err
	}
	if document.Role == role {
		return nil
	}
	document.Role = role
	encoded, err := wire.EncodeStable(document)
	if err != nil {
		return err
	}
	info, err := os.Stat(mapPath)
	if err != nil {
		return err
	}
	// The sibling temporary file is created exclusively under a random name,
	// so a planted path or symbolic link is never opened or overwritten.
	file, err := os.CreateTemp(filepath.Dir(mapPath), "."+filepath.Base(mapPath)+".role.*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	_, err = file.Write(encoded)
	err = errors.Join(err, file.Chmod(info.Mode().Perm()), file.Close())
	if err == nil {
		err = os.Rename(temporary, mapPath)
	}
	if err != nil {
		_ = removeFile(temporary)
		return err
	}
	return nil
}

// markStableChoreMap gives the prepared map of a chore change its role. Core
// cem prepare writes cem/1.0 by default since V1-1276 (CEM-V1-140); a map
// that does not declare cem/1.0 is left unmarked, and a stable map it cannot
// mark is not prepared.
func (c *change) markStableChoreMap() {
	mapPath := c.path(".corvint/change.cem.json")
	if !c.cemPrepared || !declaresStableMap(mapPath) {
		return
	}
	if role := c.choreRole(); role != "" && markChoreRole(mapPath, role) != nil {
		c.cemPrepared = false
	}
}
