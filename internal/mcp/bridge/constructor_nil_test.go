// SPDX-License-Identifier: Apache-2.0
package bridge

import "testing"

// MCPV0-7407: assigning a constructor's success to error stays nil.
func TestMCPV07407ConstructorNil(t *testing.T) {
	root := makeRepository(t)
	for _, open := range []func(string) (*Registry, error){func(root string) (*Registry, error) { return NewTaskReview(root) }, func(root string) (*Registry, error) { return NewTaskReviewLSP(root) }} {
		var err error
		var registry *Registry
		registry, err = open(root)
		if err != nil || registry == nil {
			t.Fatalf("success registry=%v err=%v", registry, err)
		}
		registry, err = open(root + "/missing")
		if err == nil || registry != nil {
			t.Fatal("invalid root accepted")
		}
		if _, ok := err.(*Error); !ok {
			t.Fatalf("failure lost typed error: %T", err)
		}
	}
}
