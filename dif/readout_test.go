// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dif

import (
	"bytes"
	"io"
	"testing"

	"golang.org/x/xerrors"
)

func TestReadout(t *testing.T) {
	const (
		difID = 0x42
	)
	for _, tc := range []struct {
		name string
		raw  []byte
		want error
	}{
		{
			name: "no data",
			raw:  nil,
			want: xerrors.Errorf("difreadout: could not read global header marker: %w", io.EOF),
		},
		{
			name: "normal-global-header",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				frHeader,
				1,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				frHeader,
				2,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				gbTrailer,
				0x4c, 0x1a, // CRC-16
			},
		},
		{
			name: "normal-global-header-0xbb-version",
			raw: []byte{
				gbHeaderB,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-2
				0, // hdr-3

				frHeader,
				1,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				frHeader,
				2,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				gbTrailer,
				0x52, 0x3f, // CRC-16
			},
		},
		{
			name: "invalid-header",
			raw: []byte{
				gbHeader + 1,
			},
			want: xerrors.Errorf("difreadout: could not read global header marker (got=0x%x)", gbHeader+1),
		},
		{
			name: "invalid-dif-header-eof",
			raw: []byte{
				gbHeader,
			},
			want: xerrors.Errorf("difreadout: could not read DIF header: %w", io.EOF),
		},
		{
			name: "invalid-dif-header-unexpected-eof",
			raw: []byte{
				gbHeader, 1, 2,
			},
			want: xerrors.Errorf("difreadout: could not read DIF header: %w", io.ErrUnexpectedEOF),
		},
		{
			name: "invalid-dif-id",
			raw: []byte{
				gbHeader,
				difID + 1,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2
			},
			want: xerrors.Errorf("difreadout: invalid DIF ID (got=0x%x, want=0x%x)", difID+1, difID),
		},
		{
			name: "short-frame-header",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2
			},
			want: xerrors.Errorf("difreadout: DIF 0x%x could not read frame header/global trailer: %w", difID, io.EOF),
		},
		{
			name: "analog-frame-header",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				anHeader, // analog frame header
			},
			want: xerrors.Errorf("difreadout: DIF 0x%x contains an analog frame", difID),
		},
		{
			name: "invalid-frame-header",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				frHeader + 1,
				1,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,
			},
			want: xerrors.Errorf("difreadout: DIF 0x%x invalid frame/global marker (got=0x%x)", difID, frHeader+1),
		},
		{
			name: "short-frame-header",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				frHeader,
				1, // hardroc header
				frTrailer,
			},
			want: xerrors.Errorf("difreadout: DIF 0x%x could not read hardroc frame: %w", difID, io.ErrUnexpectedEOF),
		},
		{
			name: "incomplete-frame",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				frHeader,
				incFrame,
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,
			},
			want: xerrors.Errorf("difreadout: DIF 0x%x received an incomplete frame", difID),
		},
		{
			name: "missing-global-trailer",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				frHeader,
				1,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				frHeader,
				2,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,
			},
			want: xerrors.Errorf("difreadout: DIF 0x%x could not read frame header/global trailer: %w", difID, io.EOF),
		},
		{
			name: "invalid-global-trailer",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				frHeader,
				1,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				frHeader,
				2,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				gbTrailer + 1,
			},
			want: xerrors.Errorf("difreadout: DIF 0x%x invalid frame/global marker (got=0x%x)", difID, gbTrailer+1),
		},
		{
			name: "missing-crc-16",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				frHeader,
				1,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				frHeader,
				2,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				gbTrailer,
			},
			want: xerrors.Errorf("difreadout: DIF 0x%x could not receive CRC-16: %w", difID, io.EOF),
		},
		{
			name: "short-crc-16",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				frHeader,
				1,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				frHeader,
				2,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				gbTrailer,
				0xb5, // CRC-16
			},
			want: xerrors.Errorf("difreadout: DIF 0x%x could not receive CRC-16: %w", difID, io.ErrUnexpectedEOF),
		},
		{
			name: "invalid-crc-16",
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				frHeader,
				1,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				frHeader,
				2,          // hardroc header
				10, 11, 12, // bcid
				20, 21, 22, 23, 24, 25, 26, 27, // data-1
				30, 31, 32, 33, 34, 35, 36, 37, // data-2
				frTrailer,

				gbTrailer,
				0xb5, 0xff, // CRC-16
			},
			want: xerrors.Errorf("difreadout: DIF 0x%x inconsistent CRC: recv=0xb5ff comp=0x4c1a", difID),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.raw)
			w := new(bytes.Buffer)
			err := NewReadout(difID, w, r).Readout()
			switch {
			case err != nil && tc.want == nil:
				t.Fatalf("got=%v, want=%v", err, tc.want)
			case err == nil && tc.want == nil:
				if got, want := w.Bytes(), tc.raw; !bytes.Equal(got, want) {
					t.Fatalf("invalid output readout data:\ngot= %v\nwant=%v\n", got, want)
				}
			case err != nil && tc.want != nil:
				if got, want := err.Error(), tc.want.Error(); got != want {
					t.Fatalf("invalid error:\ngot: %+v\nwant:%+v\n", got, want)
				}
			case err == nil && tc.want != nil:
				t.Fatalf("expected an error: %+v", tc.want)
			}
		})
	}
}
