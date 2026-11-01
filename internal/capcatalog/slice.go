package capcatalog

import (
	"bytes"
	"encoding/json"
	"slices"
)

const (
	// CommandSliceSchema is the one-command slice of the catalog that
	// `corvint help --json COMMAND` and corvint.help's command topic return (CAP-V0-950).
	CommandSliceSchema = "corvint-capability-command/0"
	// MaxCommandSliceBytes bounds a slice's encoding, newline included, below
	// 5 KiB; the docs entry's flag tables alone take about 5,000 bytes.
	MaxCommandSliceBytes = 5119
)

// CommandSlice is one catalog command entry. WriteStatements keeps the leading
// sentences that fit MaxCommandSliceBytes; WriteStatementsOmitted counts the
// rest, which `corvint help COMMAND` and the full catalog still carry.
type CommandSlice struct {
	Schema                 string  `json:"schema"`
	CatalogSchema          string  `json:"catalogSchema"`
	Command                Command `json:"command"`
	WriteStatementsOmitted int     `json:"writeStatementsOmitted"`
	FullCatalog            string  `json:"fullCatalog"`
}

// Slice returns the compact, newline-terminated slice of one command of the
// encoded catalog (the `corvint help --json` bytes), or false with every
// command name when catalog names no such command.
func Slice(catalog []byte, name string) ([]byte, []string, bool) {
	var document Catalog
	if err := json.Unmarshal(catalog, &document); err != nil {
		return nil, nil, false
	}
	names := make([]string, 0, len(document.Commands))
	for _, command := range document.Commands {
		names = append(names, command.Name)
	}
	index := slices.Index(names, name)
	if index < 0 {
		return nil, names, false
	}
	command := document.Commands[index]
	statements := command.WriteStatements
	for kept := len(statements); kept >= 0; kept-- {
		command.WriteStatements = slices.Clone(statements[:kept])
		encoded, err := encodeCompact(CommandSlice{
			Schema: CommandSliceSchema, CatalogSchema: document.Schema, Command: command,
			WriteStatementsOmitted: len(statements) - kept, FullCatalog: "corvint help --json",
		})
		if err != nil {
			return nil, names, false
		}
		if len(encoded) <= MaxCommandSliceBytes || kept == 0 {
			return encoded, names, true
		}
	}
	return nil, names, false
}

func encodeCompact(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
