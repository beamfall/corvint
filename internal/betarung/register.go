package betarung

import (
	"fmt"
	"sort"
	"strings"
)

const (
	entryHeadingPrefix = "### DR-"
	statusPrefix       = "- **Status:**"
	commandPrefix      = "- **Command:**"
)

// closedStatuses are the Status words the register uses for an entry that no
// longer blocks. Any other first word, including an emphasized `**OPEN**`, a
// misspelling or no Status bullet at all, reads as open: an unrecognized status
// must oblige disclosure rather than let an admission skip it.
var closedStatuses = map[string]bool{"LANDED": true, "CLOSED": true, "ADJUDICATED": true}

// RegisterEntry is one adjudicated entry in conformance/divergence-register.md,
// reduced to the two facts the beta rung needs: whether it is open, and the
// commands its Command line names.
type RegisterEntry struct {
	ID       string
	Status   string
	Commands []string
}

// ParseRegister reads conformance/divergence-register.md and returns one entry
// per `### DR-` heading. It reads only the entry's own Status and Command
// bullets, so prose elsewhere in the entry cannot change its verdict. The
// register wraps a long Command bullet onto indented continuation lines, and a
// command named only there obliges disclosure too, so those lines are read as
// part of the bullet.
func ParseRegister(markdown []byte) []RegisterEntry {
	var entries []RegisterEntry
	command, reading := "", false
	for _, raw := range strings.Split(string(markdown), "\n") {
		line := strings.TrimSpace(raw)
		if reading && commandContinuation(raw, line) {
			command += " " + line
			continue
		}
		if reading {
			entries[len(entries)-1].Commands = backtickedHeadWords(command)
			reading = false
		}
		if strings.HasPrefix(line, entryHeadingPrefix) {
			entries = append(entries, RegisterEntry{ID: entryHeadingIdentifier(line)})
			continue
		}
		if len(entries) == 0 {
			continue
		}
		current := &entries[len(entries)-1]
		if strings.HasPrefix(line, statusPrefix) && current.Status == "" {
			current.Status = firstWord(strings.TrimPrefix(line, statusPrefix))
		}
		if strings.HasPrefix(line, commandPrefix) && current.Commands == nil {
			command, reading = strings.TrimPrefix(line, commandPrefix), true
		}
	}
	if reading {
		entries[len(entries)-1].Commands = backtickedHeadWords(command)
	}
	return entries
}

// commandContinuation reports whether a raw line continues the bullet above
// it: indented, not blank, and not a nested bullet of its own.
func commandContinuation(raw, line string) bool {
	if line == "" {
		return false
	}
	if len(strings.TrimLeft(raw, " \t")) == len(raw) {
		return false
	}
	return !strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "* ")
}

// OpenRegisterEntries keeps every entry whose Status line does not read as
// closed. An open entry does not block beta admission (GPK-V0-042); it obliges
// disclosure.
func OpenRegisterEntries(markdown []byte) []RegisterEntry {
	var open []RegisterEntry
	for _, entry := range ParseRegister(markdown) {
		if closedStatuses[entry.Status] {
			continue
		}
		open = append(open, entry)
	}
	return open
}

// CheckRegister proves the record's open-divergence declaration still describes
// the register at this revision. It fails in both directions on purpose: an
// entry that opens after the record was written must fail until the record
// names it, and an entry that closes must fail until the record drops it.
// Without that, "no undisclosed open entry" would be a claim about a stale copy
// rather than about the register.
func (record Record) CheckRegister(markdown []byte) error {
	open := OpenRegisterEntries(markdown)
	if err := record.matchOpenIdentifiers(open); err != nil {
		return err
	}
	return record.matchOpenCommands(open)
}

func (record Record) matchOpenIdentifiers(open []RegisterEntry) error {
	declared := identifiers(record.OpenDivergences)
	actual := entryIdentifiers(open)
	if strings.Join(declared, ",") == strings.Join(actual, ",") {
		return nil
	}
	return fmt.Errorf(
		"beta admission record: open divergences [%s] do not match the register's open entries [%s]",
		strings.Join(declared, " "), strings.Join(actual, " "),
	)
}

func (record Record) matchOpenCommands(open []RegisterEntry) error {
	byIdentifier := map[string]RegisterEntry{}
	for _, entry := range open {
		byIdentifier[entry.ID] = entry
	}
	for _, declared := range record.OpenDivergences {
		entry := byIdentifier[declared.ID]
		if command, missing := firstMissing(declared.Commands, entry.Commands); missing {
			return fmt.Errorf(
				"beta admission record: %s is declared against command %q, which its register Command line does not name (register names %v)",
				declared.ID, command, entry.Commands,
			)
		}
		// The converse: a command the register names but the record omits would
		// escape validateDisclosure and could state BETA without disclosing it.
		if command, missing := firstMissing(entry.Commands, declared.Commands); missing {
			return fmt.Errorf(
				"beta admission record: %s's register Command line names command %q, which the record does not declare (record declares %v)",
				declared.ID, command, declared.Commands,
			)
		}
	}
	return nil
}

// firstMissing returns the first value of want that have does not contain.
func firstMissing(want, have []string) (string, bool) {
	for _, value := range want {
		if !contains(have, value) {
			return value, true
		}
	}
	return "", false
}

func identifiers(entries []OpenDivergence) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.ID)
	}
	sort.Strings(out)
	return out
}

func entryIdentifiers(entries []RegisterEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.ID)
	}
	sort.Strings(out)
	return out
}

func entryHeadingIdentifier(line string) string {
	return firstWord(strings.TrimPrefix(line, "### "))
}

func firstWord(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// backtickedHeadWords returns the first word of every backticked span on a
// Command line. The register writes a command as `harness event --event
// user-prompt`, so the head word is the command name.
func backtickedHeadWords(text string) []string {
	spans := strings.Split(text, "`")
	out := []string{}
	for index := 1; index < len(spans); index += 2 {
		word := firstWord(spans[index])
		if word == "" {
			continue
		}
		out = append(out, word)
	}
	return out
}
