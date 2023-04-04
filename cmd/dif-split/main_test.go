// Copyright 2021 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-lpc/mim/internal/eformat"
)

func TestSplit(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "dif-split-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	oname := filepath.Join(tmpdir, "out.raw")

	f, err := os.Create(filepath.Join(tmpdir, "dif.raw"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	dif1 := eformat.DIF{
		Header: eformat.GlobalHeader{
			ID:        0x1,
			DTC:       10,
			ATC:       11,
			GTC:       12,
			AbsBCID:   0x0000112233445566,
			TimeDIFTC: 0x00112233,
		},
		Frames: []eformat.Frame{
			{
				Header: 11,
				BCID:   0x001a1b1c,
				Data:   [16]uint8{0xa, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			},
			{
				Header: 12,
				BCID:   0x002a2b2c,
				Data: [16]uint8{
					0xb, 21, 22, 23, 24, 25, 26, 27, 28, 29,
					210, 211, 212, 213, 214, 215,
				},
			},
		},
	}

	dif2 := eformat.DIF{
		Header: eformat.GlobalHeader{
			ID:        0x2,
			DTC:       20,
			ATC:       21,
			GTC:       22,
			AbsBCID:   0x0000112233445566,
			TimeDIFTC: 0x00112233,
		},
		Frames: []eformat.Frame{
			{
				Header: 21,
				BCID:   0x001a1b1c,
				Data:   [16]uint8{0xa, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			},
			{
				Header: 22,
				BCID:   0x002a2b2c,
				Data: [16]uint8{
					0xb, 21, 22, 23, 24, 25, 26, 27, 28, 29,
					210, 211, 212, 213, 214, 215,
				},
			},
		},
	}

	for _, dif := range []eformat.DIF{dif1, dif2} {
		err = eformat.NewEncoder(f).Encode(&dif)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("could not close input file: %+v", err)
	}

	xmain([]string{"-eda", "-o", oname, f.Name()})

	for _, tc := range []struct {
		fname string
		want  eformat.DIF
	}{
		{filepath.Join(tmpdir, "out-001.raw"), dif1},
		{filepath.Join(tmpdir, "out-002.raw"), dif2},
	} {
		f, err := os.Open(tc.fname)
		if err != nil {
			t.Fatalf("could not open split file: %+v", err)
		}
		defer f.Close()

		var dif eformat.DIF
		dec := eformat.NewDecoder(0, f)
		err = dec.Decode(&dif)
		if err != nil {
			t.Fatalf("could not decode DIF from %q: %+v", tc.fname, err)
		}

		if got, want := dif, tc.want; !reflect.DeepEqual(got, want) {
			t.Fatalf("invalid split")
		}
	}

}
