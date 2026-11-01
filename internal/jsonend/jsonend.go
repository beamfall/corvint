// Package jsonend checks that a strict single-value JSON decode consumed the
// whole input.
//
// json.Decoder.More reports false before a trailing closing delimiter, so a
// More check accepts `{"a":1}}` or `[1]]` as one value (V1-3358). The next
// token of a fully consumed input is io.EOF; anything else, a stray delimiter
// included, is trailing content.
package jsonend

import (
	"encoding/json"
	"errors"
	"io"
)

// Done reports whether decoder has no token left after the value it decoded.
// Trailing whitespace is accepted; a trailing value, a stray closing
// delimiter or a syntax error is not.
func Done(decoder *json.Decoder) bool {
	_, err := decoder.Token()
	return errors.Is(err, io.EOF)
}
