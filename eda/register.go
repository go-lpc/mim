// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"fmt"
	"io"

	"github.com/go-lpc/mim/eda/internal/regs"
)

type rwer interface {
	io.ReaderAt
	io.WriterAt
}

type rw32 interface {
	readU32(r io.ReaderAt, offset int64) uint32
	writeU32(w io.WriterAt, offset int64, v uint32)
}

type reg32 struct {
	r func() uint32
	w func(v uint32)
}

func newReg32(dev rw32, rw rwer, offset int64) reg32 {
	return reg32{
		r: func() uint32 {
			return dev.readU32(rw, offset)
		},
		w: func(v uint32) {
			dev.writeU32(rw, offset, v)
		},
	}
}

type hrCfg struct {
	rw   rwer
	addr int64
}

func newHRCfg(rw rwer, offset int64) hrCfg {
	return hrCfg{
		rw:   rw,
		addr: offset,
	}
}

func (hr *hrCfg) r(i int) byte {
	buf := make([]byte, 1)
	_, err := hr.rw.ReadAt(buf, hr.addr+int64(i))
	if err != nil {
		panic(fmt.Errorf("could not read HR cfg at addr=0x%x + %d: %+v", hr.addr, i, err))
	}
	return buf[0]
}

func (hr *hrCfg) w(p []byte) (int, error) {
	if len(p) > szCfgHR {
		return 0, io.ErrShortBuffer
	}
	n, err := hr.rw.WriteAt(p, hr.addr)
	return int(n), err
}

type daqFIFO struct {
	pins [6]reg32
}

func newDAQFIFO(dev rw32, rw rwer, offset int64) daqFIFO {
	const sz = 4 // sizeof(uint32)
	return daqFIFO{
		pins: [6]reg32{
			newReg32(dev, rw, offset+sz*regs.ALTERA_AVALON_FIFO_LEVEL_REG),
			newReg32(dev, rw, offset+sz*regs.ALTERA_AVALON_FIFO_STATUS_REG),
			newReg32(dev, rw, offset+sz*regs.ALTERA_AVALON_FIFO_EVENT_REG),
			newReg32(dev, rw, offset+sz*regs.ALTERA_AVALON_FIFO_IENABLE_REG),
			newReg32(dev, rw, offset+sz*regs.ALTERA_AVALON_FIFO_ALMOSTFULL_REG),
			newReg32(dev, rw, offset+sz*regs.ALTERA_AVALON_FIFO_ALMOSTEMPTY_REG),
		},
	}
}

func (daq *daqFIFO) r(i int) uint32 {
	return daq.pins[i].r()
}

func (daq *daqFIFO) w(i int, v uint32) {
	daq.pins[i].w(v)
}
