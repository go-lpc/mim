// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dif

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

// Decoder reads (and validates) data from an underlying data source.
// Decoder computes CRC-16 checksums on the fly, during the
// acquisition of DIF Frames.
type Decoder struct {
	r io.Reader

	dif uint8 // current DIF ID
	buf []byte
	err error
	crc crc16.Hash16
}

// NewDecoder creates a decoder that reads and validates data from r.
func NewDecoder(difID uint8, r io.Reader) *Decoder {
	return &Decoder{
		r:   r,
		dif: difID,
		buf: make([]byte, 8),
		crc: crc16.New(nil),
	}
}

func (dec *Decoder) crcw(p []byte) {
	_, _ = dec.crc.Write(p) // can not fail.
}

func (dec *Decoder) reset() {
	dec.crc.Reset()
}

func (dec *Decoder) Decode(dif *DIF) error {
	dec.reset()

	v := dec.readU8()
	if dec.err != nil {
		return xerrors.Errorf("dif: could not read global header marker: %w", dec.err)
	}
	switch v {
	case gbHeader, gbHeaderB: // global header. ok
	default:
		return xerrors.Errorf("dif: could not read global header marker (got=0x%x)", v)
	}

	dec.crcU8(v)

	var hdr []byte
	switch v {
	case gbHeader:
		hdr = make([]byte, 23)
	case gbHeaderB:
		hdr = make([]byte, 32)
	}

	dec.read(hdr)
	if dec.err != nil {
		return xerrors.Errorf("dif: could not read DIF header: %w", dec.err)
	}
	dec.crcw(hdr)

	difID := hdr[0]
	if difID != dec.dif {
		return xerrors.Errorf("dif: invalid DIF ID (got=0x%x, want=0x%x)", difID, dec.dif)
	}

	dif.Header.ID = hdr[0]
	dif.Header.DTC = binary.BigEndian.Uint32(hdr[1 : 1+4])
	dif.Header.ATC = binary.BigEndian.Uint32(hdr[5 : 5+4])
	dif.Header.GTC = binary.BigEndian.Uint32(hdr[9 : 9+4])
	copy(dif.Header.AbsBCID[:], hdr[13:13+6])
	copy(dif.Header.TimeDIFTC[:], hdr[19:19+3])
	dif.Frames = dif.Frames[:0]

	//	var (
	//		nlines  = int(hdr[22] >> 4)
	//	)

	var (
		hrData = make([]byte, 19) // bcid (3 bytes) + data (16 bytes)
	)

loop:
	for {
		v := dec.readU8()
		if dec.err != nil {
			return xerrors.Errorf(
				"dif: DIF 0x%x could not read frame header/global trailer: %w",
				dec.dif, dec.err,
			)
		}

		switch v {
		default:
			return xerrors.Errorf("dif: DIF 0x%x invalid frame/global marker (got=0x%x)", dec.dif, v)

		case anHeader:
			// analog frame header. not supported.
			return xerrors.Errorf("dif: DIF 0x%x contains an analog frame", dec.dif)

		case frHeader:
			dec.crcU8(v)
		frameLoop:
			for {
				v := dec.readU8()
				if dec.err != nil {
					if xerrors.Is(dec.err, io.EOF) {
						dec.err = io.ErrUnexpectedEOF
					}
					return xerrors.Errorf(
						"dif: DIF 0x%x could not read frame trailer/hardroc header: %w",
						dec.dif, dec.err,
					)
				}

				switch v {
				default: // not a frame trailer, so a hardroc header
					dec.crcU8(v)
					dec.read(hrData)
					if dec.err != nil {
						return xerrors.Errorf(
							"dif: DIF 0x%x could not read hardroc frame: %w",
							dec.dif, dec.err,
						)
					}
					dec.crcw(hrData)
					frame := Frame{Header: v}
					copy(frame.BCID[:], hrData[0:3])
					copy(frame.Data[:], hrData[3:3+16])
					dif.Frames = append(dif.Frames, frame)

				case incFrame:
					return xerrors.Errorf("dif: DIF 0x%x received an incomplete frame", dec.dif)

				case frTrailer:
					dec.crcU8(v)
					break frameLoop
				}
			}

		case gbTrailer:
			var (
				compCRC = dec.crc.Sum16()
				recvCRC = dec.readU16()
			)
			if dec.err != nil {
				return xerrors.Errorf(
					"dif: DIF 0x%x could not receive CRC-16: %w",
					dec.dif, dec.err,
				)
			}

			if compCRC != recvCRC {
				return xerrors.Errorf(
					"dif: DIF 0x%x inconsistent CRC: recv=0x%04x comp=0x%04x",
					dec.dif, recvCRC, compCRC,
				)
			}
			break loop
		}
	}

	return dec.err
}

func (dec *Decoder) read(p []byte) {
	if dec.err != nil {
		return
	}
	_, dec.err = io.ReadFull(dec.r, p)
}

func (dec *Decoder) readU8() uint8 {
	dec.load(1)
	return dec.buf[0]
}

func (dec *Decoder) readU16() uint16 {
	const n = 2
	dec.load(n)
	return binary.BigEndian.Uint16(dec.buf[:n])
}

func (dec *Decoder) load(n int) {
	if dec.err != nil {
		return
	}
	if cap(dec.buf) < n {
		dec.buf = append(dec.buf[:len(dec.buf)], make([]byte, n-cap(dec.buf))...)
	}
	dec.buf = dec.buf[:n]
	_, dec.err = io.ReadFull(dec.r, dec.buf[:n])
}

func (dec *Decoder) crcU8(v uint8) {
	dec.buf[0] = v
	dec.crcw(dec.buf[:1])
}
