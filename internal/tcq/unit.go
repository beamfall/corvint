package tcq

import (
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Association kinds (TCQ-V0-006).
const (
	kindPythonTestFunction      = "PYTHON_TEST_FUNCTION"
	kindPythonDocstringFunction = "PYTHON_DOCSTRING_FUNCTION"
	kindGoTestFunction          = "GO_TEST_FUNCTION"
	kindGoTableCase             = "GO_TABLE_CASE"
)

// Anchor profiles the dispatch table of TCQ-V0-005 admits, and the unit profiles
// they dispatch to.
const (
	anchorPythonTestName  = "python-test-name/1"
	anchorPythonDocstring = "python-docstring/1"
	anchorGoTestName      = "go-test-name/1"
	anchorGoTableCase     = "go-table-case/1"

	unitProfilePython = "python-ast/1"
	unitProfileGo     = "go-lexical/1"
)

// testUnit is one safely delimited test unit bound to the target blob. Per
// TCQ-V0-020 raw source, runtime names, and anchor text never reach TCQ output:
// runtimeName lives here only long enough to derive the execution key.
type testUnit struct {
	path             string
	blobOID          string
	bodyStart        int64
	bodyEnd          int64
	bodySHA256       string
	executionKey     string
	extractorProfile string
	runtimeName      string
	associationKind  string
	empty            bool
	skipped          bool
}

// identityPreimage is the exact TCQ-V0-022 body. Claim identity, association
// kind, hygiene, report state, and relation deliberately do not participate, so
// two claims resolving to one unit share one unit row.
func (unit testUnit) identityPreimage() wire.Value {
	return jsonObject(
		member{"blobOid", jsonString(unit.blobOID)},
		member{"bodySha256", jsonString(unit.bodySHA256)},
		member{"bodySpan", jsonObject(
			member{"end", jsonInt(unit.bodyEnd)},
			member{"start", jsonInt(unit.bodyStart)},
		)},
		member{"executionKeySha256", jsonString(unit.executionKey)},
		member{"extractorProfile", jsonString(unit.extractorProfile)},
		member{"path", jsonString(unit.path)},
	)
}

func (unit testUnit) identity() string {
	return unitPrefix + domainHash(domainUnit, canonicalValue(unit.identityPreimage()))
}

// wireValue is the TCQ-V0-038 unit row: exactly the identity preimage plus `id`.
func (unit testUnit) wireValue() wire.Value {
	row := unit.identityPreimage()
	row.Obj.Keys = append(row.Obj.Keys, "id")
	row.Obj.Values["id"] = jsonString(unit.identity())
	return row
}
