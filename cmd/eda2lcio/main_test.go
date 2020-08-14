// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"compress/flate"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-lpc/mim/internal/eformat"
)

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

func TestEDA2LCIO(t *testing.T) {
	tmp, err := ioutil.TempDir("", "mim-xcnv-")
	if err != nil {
		t.Fatalf("could not create tmp dir: %+v", err)
	}
	defer os.RemoveAll(tmp)

	refdif := eformat.DIF{
		Header: eformat.GlobalHeader{
			ID:        0x42,
			DTC:       10,
			ATC:       11,
			GTC:       12,
			AbsBCID:   0x0000112233445566,
			TimeDIFTC: 0x00112233,
		},
		Frames: []eformat.Frame{
			{
				Header: 1,
				BCID:   0x001a1b1c,
				Data:   [16]uint8{0xa, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			},
			{
				Header: 2,
				BCID:   0x002a2b2c,
				Data: [16]uint8{
					0xb, 21, 22, 23, 24, 25, 26, 27, 28, 29,
					210, 211, 212, 213, 214, 215,
				},
			},
		},
	}

	fname := filepath.Join(tmp, "eda_063.000.raw")
	edaf, err := os.Create(fname)
	if err != nil {
		t.Fatalf("could not create raw EDA file: %+v", err)
	}
	defer edaf.Close()

	err = eformat.NewEncoder(edaf).Encode(&refdif)
	if err != nil {
		t.Fatalf("could not encode EDA: %+v", err)
	}

	err = edaf.Close()
	if err != nil {
		t.Fatalf("could not close EDA file: %+v", err)
	}

	err = process(fname+".lcio", flate.DefaultCompression, fname)
	if err != nil {
		t.Fatalf("could not convert EDA file: %+v", err)
	}
}
