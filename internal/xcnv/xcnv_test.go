// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xcnv

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-lpc/mim/internal/eformat"
	"go-hep.org/x/hep/lcio"
)

func TestEDA2LCIO(t *testing.T) {
	tmp, err := ioutil.TempDir("", "mim-xcnv-")
	if err != nil {
		t.Fatalf("could not create tmp dir: %+v", err)
	}
	defer os.RemoveAll(tmp)

	for _, tc := range []struct {
		name string
		eda  bool
		data eformat.DIF
	}{
		{
			name: "eda_063.000",
			data: eformat.DIF{
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
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const run = 63
			msg := log.New(os.Stdout, "", 0)

			fname := filepath.Join(tmp, tc.name+".raw")
			f, err := os.Create(fname)
			if err != nil {
				t.Fatalf("could not create raw EDA file: %+v", err)
			}
			defer f.Close()

			err = eformat.NewEncoder(f).Encode(&tc.data)
			if err != nil {
				t.Fatalf("could not encode EDA: %+v", err)
			}

			err = f.Close()
			if err != nil {
				t.Fatalf("could not close EDA file: %+v", err)
			}

			edabuf, err := ioutil.ReadFile(fname)
			if err != nil {
				t.Fatalf("could not read EDA file: %+v", err)
			}

			lw, err := lcio.Create(fname + ".lcio")
			if err != nil {
				t.Fatalf("could not create LCIO file: %+v", err)
			}
			defer lw.Close()

			err = EDA2LCIO(lw, eformat.NewDecoder(tc.data.Header.ID, bytes.NewReader(edabuf)), run, msg)
			if err != nil {
				t.Fatalf("could not convert to LCIO: %+v", err)
			}
			err = lw.Close()
			if err != nil {
				t.Fatalf("could not close LCIO file: %+v", err)
			}

			ew, err := os.Create(fname)
			if err != nil {
				t.Fatalf("could not create raw EDA file: %+v", err)
			}
			defer ew.Close()

			lr, err := lcio.Open(fname + ".lcio")
			if err != nil {
				t.Fatalf("could not open LCIO file: %+v", err)
			}
			defer lr.Close()

			err = LCIO2EDA(ew, lr, 1, msg)
			if err != nil {
				t.Fatalf("could not convert to EDA: %+v", err)
			}

			err = ew.Close()
			if err != nil {
				t.Fatalf("could not close EDA file: %+v", err)
			}

			edagot, err := ioutil.ReadFile(fname)
			if err != nil {
				t.Fatalf("could not read EDA file: %+v", err)
			}

			var got eformat.DIF
			err = eformat.NewDecoder(tc.data.Header.ID, bytes.NewReader(edagot)).Decode(&got)
			if err != nil {
				t.Fatalf("could not decode EDA file: %+v", err)
			}

			if !reflect.DeepEqual(got, tc.data) {
				t.Fatalf("round-trip failed")
			}
		})
	}
}
