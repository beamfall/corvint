package testsource

import (
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// The recorded-outcome join of `corvint tests teardown` (TSA-V0-117): failed attempts of a JS
// provider receipt bound to the committed test blob, anchored at a teardown delete line.

// TeardownReceiptBinding is whether one receipt binds one file, and why not.
type TeardownReceiptBinding struct {
	Receipt string `json:"receipt"`
	Path    string `json:"path"`
	Bound   bool   `json:"bound"`
	Reason  string `json:"reason,omitempty"`
	Rows    int    `json:"rows"`
	// AmbiguousAnchors counts failed attempts left unattributed because their anchor file names
	// more than one receipt file.
	AmbiguousAnchors int `json:"ambiguousAnchors,omitempty"`
}

// TeardownRecorded is one recorded failed attempt at a teardown delete line.
type TeardownRecorded struct {
	Receipt   string `json:"receipt"`
	Retry     int    `json:"retry"`
	Line      int    `json:"line"`
	Status    int    `json:"status,omitempty"`
	Dependent string `json:"dependent,omitempty"`
}

type receiptRow struct {
	receipt string
	test    string
	retry   int
	line    int
	status  int
}

var conflictStatuses = map[int]bool{409: true, 422: true}

// receiptRows returns the failed rows of every receipt that binds the file's committed blob, and
// one binding entry per receipt.
func receiptRows(receipts []TeardownReceipt, path, digest string) ([]receiptRow, []TeardownReceiptBinding) {
	rows, bindings := []receiptRow{}, []TeardownReceiptBinding{}
	for _, receipt := range receipts {
		binding := TeardownReceiptBinding{Receipt: receipt.Name, Path: path}
		key, reason := bindReceiptFile(receipt.JS, path, digest)
		if reason != "" {
			binding.Reason = reason
			bindings = append(bindings, binding)
			continue
		}
		binding.Bound = true
		for _, t := range receipt.JS.Tests {
			attempts := t.AttemptDetails
			if len(attempts) == 0 {
				attempts = []jstestprovider.AttemptDetail{{State: t.State, Retry: t.Retries, Anchor: t.Anchor}}
			}
			for _, a := range attempts {
				if a.State != jstestprovider.StateFailed || a.Anchor == nil || !anchorNames(a.Anchor.File, key, path) {
					continue
				}
				if anchorFiles(receipt.JS, a.Anchor.File) > 1 {
					binding.AmbiguousAnchors++
					continue
				}
				row := receiptRow{receipt: receipt.Name, test: t.Name, retry: a.Retry, line: a.Anchor.Line}
				if a.RequestOutcomes != nil {
					for _, request := range a.RequestOutcomes.Requests {
						if strings.EqualFold(request.Method, "DELETE") && conflictStatuses[request.Status] {
							row.status = request.Status
							break
						}
					}
				}
				rows = append(rows, row)
				binding.Rows++
			}
		}
		bindings = append(bindings, binding)
	}
	return rows, bindings
}

// bindReceiptFile returns the one testFileDigests key naming path whose digest is the blob's.
func bindReceiptFile(receipt *jstestprovider.Receipt, path, digest string) (string, string) {
	if receipt == nil {
		return "", "receipt-not-js"
	}
	match, count := "", 0
	for key := range receipt.Identity.TestFileDigests {
		slashed := filepath.ToSlash(key)
		if slashed == path || strings.HasSuffix(slashed, "/"+path) {
			match, count = key, count+1
		}
	}
	switch {
	case count == 0:
		return "", "file-not-in-receipt"
	case count > 1:
		return "", "file-ambiguous"
	case !strings.EqualFold(receipt.Identity.TestFileDigests[match], digest):
		return "", "digest-mismatch"
	}
	return filepath.ToSlash(match), ""
}

func anchorNames(file, key, path string) bool {
	file = filepath.ToSlash(file)
	return file == key || file == path || (file != "" && !strings.HasPrefix(file, "/") && strings.HasSuffix(key, "/"+file))
}

// anchorFiles counts the receipt files an anchor file names.
func anchorFiles(receipt *jstestprovider.Receipt, file string) int {
	n := 0
	for key := range receipt.Identity.TestFileDigests {
		key = filepath.ToSlash(key)
		if anchorNames(file, key, "") {
			n++
		}
	}
	return n
}

// recordedAt returns the rows of the example recorded at a delete line.
func recordedAt(rows []receiptRow, title string, line int, dependent string) []TeardownRecorded {
	var out []TeardownRecorded
	for _, row := range rows {
		if row.test != title || row.line != line {
			continue
		}
		out = append(out, TeardownRecorded{Receipt: row.receipt, Retry: row.retry, Line: row.line, Status: row.status, Dependent: dependent})
	}
	return out
}

func hasStatus(recorded []TeardownRecorded) bool {
	for _, r := range recorded {
		if r.Status != 0 {
			return true
		}
	}
	return false
}
