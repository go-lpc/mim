// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-lpc/mim/internal/eformat"
	"github.com/go-lpc/mim/internal/xcnv"
	"go-hep.org/x/hep/lcio"
)

func TestLCIO2EDA(t *testing.T) {
	tmp, err := os.MkdirTemp("", "mim-")
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

	const run = 63
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

	edabuf, err := os.ReadFile(fname)
	if err != nil {
		t.Fatalf("could not read EDA file: %+v", err)
	}

	lw, err := lcio.Create(fname + ".lcio")
	if err != nil {
		t.Fatalf("could not create LCIO file: %+v", err)
	}
	defer lw.Close()

	err = xcnv.EDA2LCIO(lw, eformat.NewDecoder(refdif.Header.ID, bytes.NewReader(edabuf)), run, msg)
	if err != nil {
		t.Fatalf("could not convert to LCIO: %+v", err)
	}
	err = lw.Close()
	if err != nil {
		t.Fatalf("could not close LCIO file: %+v", err)
	}

	got, err := numEvents(fname + ".lcio")
	if err != nil {
		t.Fatalf("could not retrieve number of events: %+v", err)
	}
	if got, want := got, int64(1); got != want {
		t.Fatalf("invalid number of events: got=%d, want=%d", got, want)
	}

	err = process(fname, fname+".lcio")
	if err != nil {
		t.Fatalf("could not process LCIO->EDA: %+v", err)
	}
}
