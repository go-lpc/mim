// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dif

import (
	"bytes"
	"io"
	"reflect"
	"testing"

	"golang.org/x/xerrors"
)

func TestCodec(t *testing.T) {
	const (
		difID = 0x42
	)

	for _, tc := range []struct {
		name string
		dif  DIF
	}{
		{
			name: "normal",
			dif: DIF{
				Header: GlobalHeader{
					ID:        difID,
					DTC:       10,
					ATC:       11,
					GTC:       12,
					AbsBCID:   0x0000112233445566,
					TimeDIFTC: 0x00112233,
				},
				Frames: []Frame{
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
		{
			name: "no-frame",
			dif: DIF{
				Header: GlobalHeader{
					ID:        difID,
					DTC:       10,
					ATC:       11,
					GTC:       12,
					AbsBCID:   0x00001234aabbccdd,
					TimeDIFTC: 0x00112233,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			enc := NewEncoder(buf)
			err := enc.Encode(&tc.dif)
			if err != nil {
				t.Fatalf("could not encode dif frames: %+v", err)
			}

			dec := NewDecoder(difID, buf)
			var got DIF
			err = dec.Decode(&got)
			if err != nil {
				t.Fatalf("could not decode dif frames: %+v", err)
			}

			if got, want := got, tc.dif; !reflect.DeepEqual(got, want) {
				t.Fatalf("invalid r/w round-trip:\ngot= %#v\nwant=%#v", got, want)
			}
		})
	}
}

func TestEncoder(t *testing.T) {
	{
		buf := new(bytes.Buffer)
		enc := NewEncoder(buf)

		if got, want := enc.Encode(nil), error(nil); got != want {
			t.Fatalf("invalid nil-dif encoding: got=%v, want=%v", got, want)
		}
	}
	{
		buf := failingWriter{n: 0}
		enc := NewEncoder(&buf)
		if got, want := enc.Encode(&DIF{}), xerrors.Errorf("dif: could not write global header marker: %w", io.ErrUnexpectedEOF); got.Error() != want.Error() {
			t.Fatalf("invalid error:\ngot= %+v\nwant=%+v", got, want)
		}
	}

}

type failingWriter struct {
	n   int
	cur int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.cur += len(p)
	if w.cur >= w.n {
		return 0, io.ErrUnexpectedEOF
	}
	return len(p), nil
}

func TestDecoder(t *testing.T) {
	const (
		difID = 0x42
	)
	for _, tc := range []struct {
		name string
		n    int
		raw  []byte
		want error
	}{
		{
			name: "no data",
			n:    1,
			raw:  nil,
			want: xerrors.Errorf("dif: could not read global header marker: %w", io.EOF),
		},
		{
			name: "normal-global-header",
			n:    1,
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
				0x26, 0xa2, // CRC-16
			},
		},
		{
			name: "multiple-globals",
			n:    2,
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
				0x26, 0xa2, // CRC-16

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
				40, 41, 42, // bcid
				50, 51, 52, 53, 54, 55, 56, 57, // data-1
				60, 61, 62, 63, 64, 65, 66, 67, // data-2
				frTrailer,

				gbTrailer,
				0x9c, 0xbf, // CRC-16
			},
		},
		{
			name: "normal-global-header-0xbb-version",
			n:    1,
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
				0xf0, 0x5d, // CRC-16
			},
		},
		{
			name: "invalid-header",
			n:    1,
			raw: []byte{
				gbHeader + 1,
			},
			want: xerrors.Errorf("dif: could not read global header marker (got=0x%x)", gbHeader+1),
		},
		{
			name: "invalid-dif-header-eof",
			n:    1,
			raw: []byte{
				gbHeader,
			},
			want: xerrors.Errorf("dif: could not read DIF header: %w", io.EOF),
		},
		{
			name: "invalid-dif-header-unexpected-eof",
			n:    1,
			raw: []byte{
				gbHeader, 1, 2,
			},
			want: xerrors.Errorf("dif: could not read DIF header: %w", io.ErrUnexpectedEOF),
		},
		{
			name: "invalid-dif-id",
			n:    1,
			raw: []byte{
				gbHeader,
				difID + 1,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2
			},
			want: xerrors.Errorf("dif: invalid DIF ID (got=0x%x, want=0x%x)", difID+1, difID),
		},
		{
			name: "short-frame-header",
			n:    1,
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2
			},
			want: xerrors.Errorf("dif: DIF 0x%x could not read frame header/global trailer: %w", difID, io.EOF),
		},
		{
			name: "analog-frame-header",
			n:    1,
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				anHeader, // analog frame header
			},
			want: xerrors.Errorf("dif: DIF 0x%x contains an analog frame", difID),
		},
		{
			name: "invalid-frame-header",
			n:    1,
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
			want: xerrors.Errorf("dif: DIF 0x%x invalid frame/global marker (got=0x%x)", difID, frHeader+1),
		},
		{
			name: "missing-hardroc-header",
			n:    1,
			raw: []byte{
				gbHeader,
				difID,
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-0
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // hdr-1
				0, 1, // hdr-2

				frHeader,
			},
			want: xerrors.Errorf("dif: DIF 0x%x could not read frame trailer/hardroc header: %w", difID, io.ErrUnexpectedEOF),
		},
		{
			name: "short-frame-header",
			n:    1,
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
			want: xerrors.Errorf("dif: DIF 0x%x could not read hardroc frame: %w", difID, io.ErrUnexpectedEOF),
		},
		{
			name: "incomplete-frame",
			n:    1,
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
			want: xerrors.Errorf("dif: DIF 0x%x received an incomplete frame", difID),
		},
		{
			name: "missing-global-trailer",
			n:    1,
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
			want: xerrors.Errorf("dif: DIF 0x%x could not read frame header/global trailer: %w", difID, io.EOF),
		},
		{
			name: "invalid-global-trailer",
			n:    1,
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
			want: xerrors.Errorf("dif: DIF 0x%x invalid frame/global marker (got=0x%x)", difID, gbTrailer+1),
		},
		{
			name: "missing-crc-16",
			n:    1,
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
			want: xerrors.Errorf("dif: DIF 0x%x could not receive CRC-16: %w", difID, io.EOF),
		},
		{
			name: "short-crc-16",
			n:    1,
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
			want: xerrors.Errorf("dif: DIF 0x%x could not receive CRC-16: %w", difID, io.ErrUnexpectedEOF),
		},
		{
			name: "invalid-crc-16",
			n:    1,
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
			want: xerrors.Errorf("dif: DIF 0x%x inconsistent CRC: recv=0xb5ff comp=0x26a2", difID),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := NewDecoder(difID, bytes.NewReader(tc.raw))
			for i := 0; i < tc.n; i++ {
				if i > 0 {
					dec.buf = dec.buf[:0:0] // test cap-load
				}
				var data DIF
				err := dec.Decode(&data)
				switch {
				case err != nil && tc.want == nil:
					t.Fatalf("i=%d: got=%v, want=%v", i, err, tc.want)
				case err == nil && tc.want == nil:
					// ok.
				case err != nil && tc.want != nil:
					if got, want := err.Error(), tc.want.Error(); got != want {
						t.Fatalf("i=%d: invalid error:\ngot: %+v\nwant:%+v\n", i, got, want)
					}
				case err == nil && tc.want != nil:
					t.Fatalf("i=%d: expected an error: %+v", i, tc.want)
				}
			}
		})
	}
}
