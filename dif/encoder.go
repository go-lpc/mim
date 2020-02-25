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

type Encoder struct {
	w   io.Writer
	buf []byte
	err error
	crc crc16.Hash16
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:   w,
		buf: make([]byte, 8),
		crc: crc16.New(nil),
	}
}

func (enc *Encoder) crcw(p []byte) {
	_, _ = enc.crc.Write(p) // can not fail.
}

func (enc *Encoder) reset() {
	enc.crc.Reset()
}

func (enc *Encoder) Encode(dif *DIF) error {
	if dif == nil {
		return nil
	}

	enc.reset()

	enc.writeU8(gbHeader)
	if enc.err != nil {
		return xerrors.Errorf("dif: could not write global header marker: %w", enc.err)
	}

	enc.writeU8(dif.Header.ID)
	enc.writeU32(dif.Header.DTC)
	enc.writeU32(dif.Header.ATC)
	enc.writeU32(dif.Header.GTC)
	enc.write(dif.Header.AbsBCID[:])
	enc.write(dif.Header.TimeDIFTC[:])
	enc.writeU8(0) // nlines

	enc.writeU8(frHeader)
	for _, frame := range dif.Frames {
		enc.writeU8(frame.Header)
		enc.write(frame.BCID[:])
		enc.write(frame.Data[:])
	}
	enc.writeU8(frTrailer)
	crc := enc.crc.Sum16() // gb-trailer not part of CRC-16
	enc.writeU8(gbTrailer)
	enc.writeU16(crc)

	return enc.err
}

func (enc *Encoder) write(p []byte) {
	if enc.err != nil {
		return
	}
	_, enc.err = enc.w.Write(p)
	enc.crcw(p)
}

func (enc *Encoder) writeU8(v uint8) {
	const n = 1
	enc.reserve(n)
	enc.buf[0] = v
	enc.write(enc.buf[:n])
}

func (enc *Encoder) writeU16(v uint16) {
	const n = 2
	enc.reserve(n)
	binary.BigEndian.PutUint16(enc.buf[:n], v)
	enc.write(enc.buf[:n])
}

func (enc *Encoder) writeU32(v uint32) {
	const n = 4
	enc.reserve(n)
	binary.BigEndian.PutUint32(enc.buf[:n], v)
	enc.write(enc.buf[:n])
}

func (enc *Encoder) reserve(n int) {
	if cap(enc.buf) < n {
		enc.buf = append(enc.buf[:len(enc.buf)], make([]byte, n-cap(enc.buf))...)
	}
}
