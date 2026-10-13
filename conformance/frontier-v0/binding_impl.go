// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// This is the single binding file adapter.go describes. It binds only the
// CF-V0-019 codec: the shared WP3 codec outlived frontier/0 (decision 0525 §1,
// CEM-V1-217), so the independently authored codec vectors still run against it.

import "github.com/Beamfall/corvint/internal/wp3codec"

func init() { RegisterCodecer(implCodec{}) }

// implCodec routes the CF-V0-019 vectors through the shared WP3 codec. Parse
// then Encode is exactly the clause's own "parse, reserialize, and require byte
// equality before hashing" discipline, so an input the clause names invalid must
// fail in Parse rather than survive into a digest.
type implCodec struct{}

func (implCodec) CodecCanonicalize(raw []byte) ([]byte, error) {
	value, err := wp3codec.Parse(raw)
	if err != nil {
		return nil, err
	}
	return wp3codec.Encode(value)
}
