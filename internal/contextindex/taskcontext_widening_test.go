package contextindex

import (
	"regexp"
)

var bm25Reason = regexp.MustCompile(`bm25 ([0-9.]+)`)
