package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Beamfall/corvint/internal/gokernel"
)

func runFlowsCLI(root string, args ...string) (int, string, string) {
	var out, diagnostic bytes.Buffer
	code := Run(context.Background(), root, args, &out, &diagnostic, testEnv)
	return code, out.String(), diagnostic.String()
}

// testEnv stands in for the root's error envelope and affected-revision refusal: the same
// {"code": ..., "error": ..., "ok": false} line and refusal code, without the root's diagnostics.
var testEnv = Env{
	EmitError: func(diagnostic io.Writer, err error) {
		code := "internal-error"
		var coded *gokernel.Error
		if errors.As(err, &coded) {
			code = coded.Code
		}
		codeJSON, _ := json.Marshal(code)
		messageJSON, _ := json.Marshal(err.Error())
		fmt.Fprintf(diagnostic, "{\"code\": %s, \"error\": %s, \"ok\": false}\n", codeJSON, messageJSON)
	},
	BaseRefusal: func(base string) error {
		return &gokernel.Error{Code: "unsupported-affected-revision", Message: "base " + base + " is not a commit in this repository"}
	},
}
