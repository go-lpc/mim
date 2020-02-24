// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crc16_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/go-lpc/mim/internal/crc16"
)

func TestCRC16(t *testing.T) {
	for _, tc := range []struct {
		raw  []byte
		want uint16
	}{
		{
			raw:  []byte{0x1, 0x2, 0x3, 0x4, 0x5},
			want: 0x9304,
		},
	} {
		t.Run(fmt.Sprintf("0x%x", tc.want), func(t *testing.T) {
			crc := crc16.New(nil)
			if got, want := crc.BlockSize(), 1; got != want {
				t.Fatalf("invalid crc16 block size: got=%d, want=%d", got, want)
			}

			crc.Reset()

			_, err := crc.Write(tc.raw)
			if err != nil {
				t.Fatalf("could not write crc16 hash: %+v", err)
			}

			if got, want := crc.Sum16(), tc.want; got != want {
				t.Fatalf("invalid crc16 checksum: got=0x%x, want=0x%x",
					got, want,
				)
			}

			asBytes := func(v uint16) []byte {
				buf := make([]byte, crc.Size())
				binary.BigEndian.PutUint16(buf, v)
				return buf
			}

			if got, want := crc.Sum(nil), asBytes(tc.want); !bytes.Equal(got, want) {
				t.Fatalf("invalid crc16 checksum: got=0x%x, want=0x%x",
					got, want,
				)
			}
		})
	}
}
