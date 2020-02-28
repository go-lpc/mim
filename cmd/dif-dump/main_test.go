// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-lpc/mim/dif"
	"golang.org/x/xerrors"
)

func TestDump(t *testing.T) {
	tmp, err := ioutil.TempDir("", "mim-dif-dump-")
	if err != nil {
		t.Fatalf("could not create tmp dir: %+v", err)
	}
	defer os.RemoveAll(tmp)

	for _, tc := range []struct {
		name string
		data dif.DIF
		want string
		err  error
	}{
		{
			name: "simple-dif",
			data: dif.DIF{
				Header: dif.GlobalHeader{
					ID:        0x42,
					DTC:       10,
					ATC:       11,
					GTC:       12,
					AbsBCID:   0x0000112233445566,
					TimeDIFTC: 0x00112233,
				},
				Frames: []dif.Frame{
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
			want: `=== DIF-ID 0x42 ===
DIF trigger:         10
ACQ trigger:         11
Gbl trigger:         12
Abs BCID:     18838586676582
Time DIF:       1122867
Frames:               2
  hroc=0x01 BCID= 1710876 0a0102030405060708090a0b0c0d0e0f
  hroc=0x02 BCID= 2763564 0b15161718191a1b1c1dd2d3d4d5d6d7
`,
		},
		{
			name: "invalid-dif",
			data: dif.DIF{},
			want: string([]byte{0xb0, 0x42}),
			err:  xerrors.Errorf("could not decode DIF: dif: could not read DIF header: %w", io.ErrUnexpectedEOF),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fname := filepath.Join(tmp, tc.name+".raw")
			f, err := os.Create(fname)
			if err != nil {
				t.Fatalf("could not create raw dif file: %+v", err)
			}
			defer f.Close()

			switch {
			case tc.err == nil:
				err = dif.NewEncoder(f).Encode(&tc.data)
				if err != nil {
					t.Fatalf("could not encode dif: %+v", err)
				}
			default:
				_, err = f.Write([]byte(tc.want))
				if err != nil {
					t.Fatalf("could not encode dif: %+v", err)
				}
			}

			err = f.Close()
			if err != nil {
				t.Fatalf("could not close raw dif file: %+v", err)
			}

			out := new(strings.Builder)
			err = process(out, fname)
			switch {
			case err != nil && tc.err != nil:
				if got, want := err.Error(), tc.err.Error(); got != want {
					t.Fatalf("invalid error:\ngot= %v\nwant=%v\n", got, want)
				}
			case err != nil && tc.err == nil:
				t.Fatalf("could not dif-dump: %+v", err)
			case err == nil && tc.err == nil:
				if got, want := out.String(), tc.want; got != want {
					t.Fatalf("invalid dif-dump output:\ngot:\n%s\nwant:\n%s\n", got, want)
				}
			case err == nil && tc.err != nil:
				t.Fatalf("invalid error:\ngot= %v\nwant=%v\n", err, tc.err)
			}
		})
	}
}
