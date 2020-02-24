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

// Readout reads (and validates) data from an underlying data source
// and publishes data to an underlying data sink.
// Readout computes CRC-16 checksums on the fly, during the
// acquisition of DIF Frames.
type Readout struct {
	w io.Writer
	r io.Reader

	dif uint8 // current DIF ID
	buf []byte
	err error
	crc crc16.Hash16
}

// NewReadout creates a new readout device that reads and validates data from r,
// and writes it out to w.
func NewReadout(difID uint8, w io.Writer, r io.Reader) *Readout {
	return &Readout{
		w:   w,
		r:   r,
		dif: difID,
		buf: make([]byte, 8),
		crc: crc16.New(nil),
	}
}

func (rdo *Readout) Readout() error {
	v := rdo.readU8()
	if rdo.err != nil {
		return xerrors.Errorf("difreadout: could not read global header marker: %w", rdo.err)
	}
	switch v {
	case gbHeader, gbHeaderB: // global header. ok
	default:
		return xerrors.Errorf("difreadout: could not read global header marker (got=0x%x)", v)
	}

	rdo.writeU8(v)
	rdo.crcU8(v)

	var hdr []byte
	switch v {
	case gbHeader:
		hdr = make([]byte, 23)
	case gbHeaderB:
		hdr = make([]byte, 32)
	}

	rdo.read(hdr)
	if rdo.err != nil {
		return xerrors.Errorf("difreadout: could not read DIF header: %w", rdo.err)
	}
	rdo.write(hdr)
	if rdo.err != nil {
		return xerrors.Errorf("difreadout: could not write DIF header: %w", rdo.err)
	}
	_, rdo.err = rdo.crc.Write(hdr)
	if rdo.err != nil {
		return xerrors.Errorf("difreadout: could not update CRC with DIF header: %w", rdo.err)
	}

	difID := hdr[0]
	if difID != rdo.dif {
		return xerrors.Errorf("difreadout: invalid DIF ID (got=0x%x, want=0x%x)", difID, rdo.dif)
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
		v := rdo.readU8()
		if rdo.err != nil {
			return xerrors.Errorf("difreadout: DIF 0x%x could not read frame header/global trailer: %w",
				rdo.dif, rdo.err,
			)
		}

		rdo.writeU8(v)
		if rdo.err != nil {
			return xerrors.Errorf("difreadout: DIF 0x%x could not write frame header/global trailer: %w",
				rdo.dif, rdo.err,
			)
		}

		switch v {
		default:
			return xerrors.Errorf("difreadout: DIF 0x%x invalid frame/global marker (got=0x%x)", rdo.dif, v)

		case anHeader:
			// analog frame header. not supported.
			return xerrors.Errorf("difreadout: DIF 0x%x contains an analog frame", rdo.dif)

		case frHeader:
			rdo.crcU8(v)
		frameLoop:
			for {
				v := rdo.readU8()
				if rdo.err != nil {
					return xerrors.Errorf(
						"difreadout: DIF 0x%x could not read frame trailer/hardroc header: %w",
						rdo.dif, rdo.err,
					)
				}

				switch v {
				default: // not a frame trailer, so a hardroc header
					rdo.writeU8(v)
					rdo.crcU8(v)
					rdo.read(hrData)
					if rdo.err != nil {
						return xerrors.Errorf(
							"difreadout: DIF 0x%x could not read hardroc frame: %w",
							rdo.dif, rdo.err,
						)
					}

					rdo.write(hrData)
					if rdo.err != nil {
						return xerrors.Errorf(
							"difreadout: DIF 0x%x could not write hardroc frame: %w",
							rdo.dif, rdo.err,
						)
					}

					_, rdo.err = rdo.crc.Write(hrData)
					if rdo.err != nil {
						return xerrors.Errorf(
							"difreadout: DIF 0x%x could not write CRC-16 hardroc frame: %w",
							rdo.dif, rdo.err,
						)
					}

				case incFrame:
					return xerrors.Errorf("difreadout: DIF 0x%x received an incomplete frame", rdo.dif)

				case frTrailer:
					rdo.writeU8(v)
					rdo.crcU8(v)
					break frameLoop
				}
			}

		case gbTrailer:
			var (
				compCRC = rdo.crc.Sum16()
				recvCRC = rdo.readU16()
			)
			if rdo.err != nil {
				return xerrors.Errorf("difreadout: DIF 0x%x could not receive CRC-16: %w",
					rdo.dif, rdo.err,
				)
			}

			rdo.writeU8(uint8(recvCRC>>8) & 0xff)
			rdo.writeU8(uint8(recvCRC>>0) & 0xff)

			if compCRC != recvCRC {
				return xerrors.Errorf("difreadout: DIF 0x%x inconsistent CRC: recv=0x%04x comp=0x%04x",
					rdo.dif, recvCRC, compCRC,
				)
			}
			break loop
		}
	}

	return rdo.err
}

func (rdo *Readout) read(p []byte) {
	if rdo.err != nil {
		return
	}
	_, rdo.err = io.ReadFull(rdo.r, p)
}

func (rdo *Readout) readU8() uint8 {
	rdo.load(1)
	return rdo.buf[0]
}

func (rdo *Readout) readU16() uint16 {
	rdo.load(2)
	return binary.BigEndian.Uint16(rdo.buf[:2])
}

// func (rdo *Readout) readU32() uint32 {
// 	rdo.load(4)
// 	return binary.BigEndian.Uint32(rdo.buf[:4])
// }

func (rdo *Readout) load(n int) {
	if rdo.err != nil {
		return
	}
	if len(rdo.buf) < n {
		rdo.buf = append(rdo.buf, make([]byte, n-len(rdo.buf))...)
	}
	_, rdo.err = io.ReadFull(rdo.r, rdo.buf[:n])
}

func (rdo *Readout) crcU8(v uint8) {
	rdo.buf[0] = v
	_, rdo.err = rdo.crc.Write(rdo.buf[:1])
}

func (rdo *Readout) write(p []byte) {
	if rdo.err != nil {
		return
	}
	_, rdo.err = rdo.w.Write(p)
}

func (rdo *Readout) writeU8(v uint8) {
	if rdo.err != nil {
		return
	}
	rdo.buf[0] = v
	_, rdo.err = rdo.w.Write(rdo.buf[:1])
}
