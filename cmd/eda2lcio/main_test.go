// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestRunNbrFrom(t *testing.T) {
	for _, tc := range []struct {
		fname string
		run   int32
	}{
		{
			fname: "./eda_063.000.raw",
			run:   63,
		},
		{
			fname: "/some/dir/eda_663.000.raw",
			run:   663,
		},
		{
			fname: "../some/dir/eda_009.000.raw",
			run:   9,
		},
	} {
		t.Run(tc.fname, func(t *testing.T) {
			got, err := runNbrFrom(tc.fname)
			if err != nil {
				t.Fatalf("could not infer run-nbr: %+v", err)
			}
			if got != tc.run {
				t.Fatalf("invalid run: got=%d, want=%d", got, tc.run)
			}
		})
	}
}
