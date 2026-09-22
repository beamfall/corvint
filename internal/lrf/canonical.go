package lrf

import (
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CanonicalBytes returns sorted-key compact UTF-8 JSON with one terminal LF.
func CanonicalBytes(result Result) []byte {
	return result.CanonicalBytes()
}

func rawCanonicalBytes(result Result) []byte {
	var out strings.Builder
	out.WriteString(`{"inputs":`)
	writeInputs(&out, result.inputs)
	out.WriteString(`,"issues":`)
	writeIssueRows(&out, result.issues)
	out.WriteString(`,"profile":"lrf/0","results":`)
	writeResultRows(&out, result.results)
	out.WriteString("}\n")
	return []byte(out.String())
}

func writeInputs(out *strings.Builder, inputs [14]any) {
	out.WriteByte('[')
	for index, value := range inputs {
		if index > 0 {
			out.WriteByte(',')
		}
		switch typed := value.(type) {
		case nil:
			out.WriteString("null")
		case string:
			out.WriteString(wire.CanonicalString(typed))
		case int64:
			out.WriteString(strconv.FormatInt(typed, 10))
		case int:
			out.WriteString(strconv.Itoa(typed))
		}
	}
	out.WriteByte(']')
}

func writeIssueRows(out *strings.Builder, rows []IssueTuple) {
	out.WriteByte('[')
	for index, row := range rows {
		if index > 0 {
			out.WriteByte(',')
		}
		writeStrings(out, row[:])
	}
	out.WriteByte(']')
}

func writeResultRows(out *strings.Builder, rows []ResultTuple) {
	out.WriteByte('[')
	for index, row := range rows {
		if index > 0 {
			out.WriteByte(',')
		}
		writeStrings(out, row[:])
	}
	out.WriteByte(']')
}

func writeStrings(out *strings.Builder, values []string) {
	out.WriteByte('[')
	for index, value := range values {
		if index > 0 {
			out.WriteByte(',')
		}
		out.WriteString(wire.CanonicalString(value))
	}
	out.WriteByte(']')
}
