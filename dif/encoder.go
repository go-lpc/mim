// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dif

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/go-lpc/mim/internal/crc16"
)

// Encoder writes DIF data to an output stream.
// Encoder computes the CRC-16 checksum on the fly and appends it
// at the end of the stream.
type Encoder struct {
	w   io.Writer
	buf []byte
	err error
	crc crc16.Hash16
}

// NewEncoder returns a new Encoder that writes to w.
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

// Encode writes the DIF data to the stream, computes the corresponding
// CRC-16 checksum on the fly and appends it to the stream.
func (enc *Encoder) Encode(dif *DIF) error {
	if dif == nil {
		return nil
	}

	enc.reset()

	enc.writeU8(gbHeader)
	if enc.err != nil {
		return fmt.Errorf("dif: could not write global header marker: %w", enc.err)
	}

	enc.writeU8(dif.Header.ID)
	enc.writeU32(dif.Header.DTC)
	enc.writeU32(dif.Header.ATC)
	enc.writeU32(dif.Header.GTC)
	enc.writeU48(dif.Header.AbsBCID)
	enc.writeU24(dif.Header.TimeDIFTC)
	enc.writeU8(0) // nlines

	enc.writeU8(frHeader)
	for _, frame := range dif.Frames {
		enc.writeU8(frame.Header)
		enc.writeU24(frame.BCID)
		enc.write(frame.Data[:])
	}
	enc.writeU8(frTrailer)
	enc.writeU8(gbTrailer)

	crc := enc.crc.Sum16()
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

func (enc *Encoder) writeU24(v uint32) {
	const n = 3
	enc.reserve(n)
	enc.buf[0] = byte(v >> 16)
	enc.buf[1] = byte(v >> 8)
	enc.buf[2] = byte(v >> 0)
	enc.write(enc.buf[:n])
}

func (enc *Encoder) writeU48(v uint64) {
	const n = 6
	enc.reserve(n)
	enc.buf[0] = byte(v >> 40)
	enc.buf[1] = byte(v >> 32)
	enc.buf[2] = byte(v >> 24)
	enc.buf[3] = byte(v >> 16)
	enc.buf[4] = byte(v >> 8)
	enc.buf[5] = byte(v >> 0)
	enc.write(enc.buf[:n])
}

func (enc *Encoder) reserve(n int) {
	if cap(enc.buf) < n {
		enc.buf = append(enc.buf[:len(enc.buf)], make([]byte, n-cap(enc.buf))...)
	}
}
