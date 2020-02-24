// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package difreadout provides tools to read out data from a DIF.
package difreadout // import "github.com/go-lpc/mim/dif/difreadout"

import (
	"encoding/binary"
	"io"

	"github.com/go-lpc/mim/internal/crc16"
	"golang.org/x/xerrors"
)

const (
	//	scHeader  = 0xb1 // slow-control header marker
	//	scTrailer = 0xa1 // slow-control trailer marker

	gbHeader  = 0xb0 // global header marker
	gbHeaderB = 0xbb // global header marker (0xBB variant)
	gbTrailer = 0xa0 // global trailer marker

	frHeader  = 0xb4 // frame header marker
	frTrailer = 0xa3 // frame trailer marker

	anHeader = 0xc4 // analog frame header marker
	incFrame = 0xc3 // incomplete frame marker
)

func Encode(w io.Writer, r io.Reader, difID uint8) error {
	enc := newEncoder(w, r, difID)
	return enc.encode()
}

type encoder struct {
	w io.Writer
	r io.Reader

	dif uint8 // current DIF ID
	buf []byte
	err error
	crc crc16.Hash16
}

func newEncoder(w io.Writer, r io.Reader, difID uint8) *encoder {
	return &encoder{
		w:   w,
		r:   r,
		dif: difID,
		buf: make([]byte, 8),
		crc: crc16.New(nil),
	}
}

func (enc *encoder) encode() error {
	v := enc.readU8()
	if enc.err != nil {
		return xerrors.Errorf("difreadout: could not read global header marker: %w", enc.err)
	}
	switch v {
	case gbHeader, gbHeaderB: // global header. ok
	default:
		return xerrors.Errorf("difreadout: could not read global header marker (got=0x%x)", v)
	}

	enc.writeU8(v)
	enc.crcU8(v)

	var hdr []byte
	switch v {
	case gbHeader:
		hdr = make([]byte, 23)
	case gbHeaderB:
		hdr = make([]byte, 32)
	}

	enc.read(hdr)
	if enc.err != nil {
		return xerrors.Errorf("difreadout: could not read DIF header: %w", enc.err)
	}
	enc.write(hdr)
	if enc.err != nil {
		return xerrors.Errorf("difreadout: could not write DIF header: %w", enc.err)
	}
	_, enc.err = enc.crc.Write(hdr)
	if enc.err != nil {
		return xerrors.Errorf("difreadout: could not update CRC with DIF header: %w", enc.err)
	}

	difID := hdr[0]
	if difID != enc.dif {
		return xerrors.Errorf("difreadout: invalid DIF ID (got=0x%x, want=0x%x)", difID, enc.dif)
	}

	var (
	//	difTC   = binary.BigEndian.Uint32(hdr[1 : 1+4])
	//	acqTC   = binary.BigEndian.Uint32(hdr[5 : 5+4])
	//	gblTC   = binary.BigEndian.Uint32(hdr[9 : 9+4])
	//	bcidMSB = binary.BigEndian.Uint16(hdr[13 : 13+2])
	//	bcidLSB = binary.BigEndian.Uint32(hdr[15 : 15+4])
	//  nlines  = int(hdr[22] >> 4)
	)

	var (
		hrData = make([]byte, 19) // bcid (3 bytes) + data (16 bytes)
	)

loop:
	for {
		v := enc.readU8()
		if enc.err != nil {
			return xerrors.Errorf("difreadout: DIF 0x%x could not read frame header/global trailer: %w",
				enc.dif, enc.err,
			)
		}

		enc.writeU8(v)
		if enc.err != nil {
			return xerrors.Errorf("difreadout: DIF 0x%x could not write frame header/global trailer: %w",
				enc.dif, enc.err,
			)
		}

		switch v {
		default:
			return xerrors.Errorf("difreadout: DIF 0x%x invalid frame/global marker (got=0x%x)", enc.dif, v)

		case anHeader:
			// analog frame header. not supported.
			return xerrors.Errorf("difreadout: DIF 0x%x contains an analog frame", enc.dif)

		case frHeader:
			enc.crcU8(v)
		frameLoop:
			for {
				v := enc.readU8()
				if enc.err != nil {
					return xerrors.Errorf(
						"difreadout: DIF 0x%x could not read frame trailer/hardroc header: %w",
						enc.dif, enc.err,
					)
				}

				switch v {
				default: // not a frame trailer, so a hardroc header
					enc.writeU8(v)
					enc.crcU8(v)
					enc.read(hrData)
					if enc.err != nil {
						return xerrors.Errorf(
							"difreadout: DIF 0x%x could not read hardroc frame: %w",
							enc.dif, enc.err,
						)
					}

					enc.write(hrData)
					if enc.err != nil {
						return xerrors.Errorf(
							"difreadout: DIF 0x%x could not write hardroc frame: %w",
							enc.dif, enc.err,
						)
					}

					_, enc.err = enc.crc.Write(hrData)
					if enc.err != nil {
						return xerrors.Errorf(
							"difreadout: DIF 0x%x could not write CRC-16 hardroc frame: %w",
							enc.dif, enc.err,
						)
					}

				case incFrame:
					return xerrors.Errorf("difreadout: DIF 0x%x received an incomplete frame", enc.dif)

				case frTrailer:
					enc.writeU8(v)
					enc.crcU8(v)
					break frameLoop
				}
			}

		case gbTrailer:
			var (
				compCRC = enc.crc.Sum16()
				recvCRC = enc.readU16()
			)
			if enc.err != nil {
				return xerrors.Errorf("difreadout: DIF 0x%x could not receive CRC-16: %w",
					enc.dif, enc.err,
				)
			}

			enc.writeU8(uint8(recvCRC>>8) & 0xff)
			enc.writeU8(uint8(recvCRC>>0) & 0xff)

			if compCRC != recvCRC {
				return xerrors.Errorf("difreadout: DIF 0x%x inconsistent CRC: recv=0x%04x comp=0x%04x",
					enc.dif, recvCRC, compCRC,
				)
			}
			break loop
		}
	}

	return enc.err
}

func (enc *encoder) read(p []byte) {
	if enc.err != nil {
		return
	}
	_, enc.err = io.ReadFull(enc.r, p)
}

func (enc *encoder) readU8() uint8 {
	enc.load(1)
	return enc.buf[0]
}

func (enc *encoder) readU16() uint16 {
	enc.load(2)
	return binary.BigEndian.Uint16(enc.buf[:2])
}

// func (enc *encoder) readU32() uint32 {
// 	enc.load(4)
// 	return binary.BigEndian.Uint32(enc.buf[:4])
// }

func (enc *encoder) load(n int) {
	if enc.err != nil {
		return
	}
	if len(enc.buf) < n {
		enc.buf = append(enc.buf, make([]byte, n-len(enc.buf))...)
	}
	_, enc.err = io.ReadFull(enc.r, enc.buf[:n])
}

func (enc *encoder) crcU8(v uint8) {
	enc.buf[0] = v
	_, enc.err = enc.crc.Write(enc.buf[:1])
}

func (enc *encoder) write(p []byte) {
	if enc.err != nil {
		return
	}
	_, enc.err = enc.w.Write(p)
}

func (enc *encoder) writeU8(v uint8) {
	if enc.err != nil {
		return
	}
	enc.buf[0] = v
	_, enc.err = enc.w.Write(enc.buf[:1])
}
