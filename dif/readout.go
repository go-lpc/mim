// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dif

import (
	"fmt"
	"io"
	"time"

	"github.com/go-daq/tdaq/log"
	"github.com/go-lpc/mim/internal/crc16"
	"github.com/go-lpc/mim/internal/eformat"
)

type asicKind uint32

const (
	microrocASIC asicKind = 11
	hardrocASIC  asicKind = 2
)

// Readout reads data out of a digital interface board (DIF).
type Readout struct {
	msg    log.MsgStream
	dev    *device
	name   string
	difID  uint32
	asic   asicKind // asic type
	nasics int      // number of asics
	ctlreg uint32   // control register
	curSC  uint32   // current slow-control status
	reg    struct {
		p2pa   uint32 // power to power A
		pa2pd  uint32 // power A to power D
		pd2daq uint32 // power D to DAQ
		daq2pd uint32 // DAQ to power D
		pd2pa  uint32 // power D to power A
	}
	// temp [2]float32 // temperatures
}

// NewReadout creates a new DIF readout.
func NewReadout(name string, prodID uint32, msg log.MsgStream) (*Readout, error) {
	dev, err := newDevice(0x0403, uint16(prodID))
	if err != nil {
		return nil, fmt.Errorf("could not find DIF driver (%s, 0x%x): %w", name, prodID, err)
	}

	rdo := &Readout{
		msg:    msg,
		dev:    dev,
		name:   name,
		asic:   hardrocASIC,
		nasics: MaxNumASICs,
		ctlreg: 0x80181b00, // ILC CCC
	}
	rdo.reg.p2pa = 0x3e8
	rdo.reg.pa2pd = 0x3e6
	rdo.reg.pd2daq = 0x4e
	rdo.reg.daq2pd = 0x4e
	rdo.reg.pd2pa = 0x4e
	_, err = fmt.Sscanf(name, "FT101%03d", &rdo.difID)
	if err != nil {
		_ = dev.close()
		return nil, fmt.Errorf("could not find DIF-id from %q: %w", name, err)
	}

	return rdo, nil
}

func (rdo *Readout) close() error {
	err := rdo.dev.close()
	if err != nil {
		return fmt.Errorf("could not close DIF driver: %w", err)
	}
	return nil
}

func (rdo *Readout) start() error {
	return rdo.dev.hardrocFlushDigitalFIFO()
}

func (rdo *Readout) stop() error {
	var err error

	err = rdo.dev.hardrocFlushDigitalFIFO()
	if err != nil {
		return fmt.Errorf("could not flush digital FIFO: %w", err)
	}

	err = rdo.dev.hardrocStopDigitalAcquisitionCommand()
	if err != nil {
		return fmt.Errorf("could not stop digital acquisition: %w", err)
	}

	err = rdo.dev.hardrocFlushDigitalFIFO()
	if err != nil {
		return fmt.Errorf("could not flush digital FIFO: %w", err)
	}

	return nil
}

func (rdo *Readout) configureRegisters() error {
	var err error
	err = rdo.dev.setDIFID(rdo.difID)
	if err != nil {
		return fmt.Errorf("could not set DIF ID to 0x%x: %w", rdo.difID, err)
	}

	err = rdo.doRefreshNumASICs()
	if err != nil {
		return fmt.Errorf("could not refresh #ASICs: %w", err)
	}

	err = rdo.dev.setEventsBetweenTemperatureReadout(5)
	if err != nil {
		return fmt.Errorf("could not set #events Temp readout: %w", err)
	}

	err = rdo.dev.setAnalogConfigureRegister(0xc0054000)
	if err != nil {
		return fmt.Errorf("could not configure analog register: %w", err)
	}

	err = rdo.dev.hardrocFlushDigitalFIFO()
	if err != nil {
		return fmt.Errorf("could not flush digital FIFO: %w", err)
	}

	fw, err := rdo.dev.usbFwVersion()
	if err != nil {
		return fmt.Errorf("could not get firmware version: %w", err)
	}
	rdo.msg.Infof("dif %s fw: 0x%x", rdo.name, fw)

	err = rdo.dev.difCptReset()
	if err != nil {
		return fmt.Errorf("could not reset DIF cpt: %w", err)
	}

	err = rdo.dev.setChipTypeRegister(map[asicKind]uint32{
		hardrocASIC:  0x100,
		microrocASIC: 0x1000,
	}[rdo.asic])
	if err != nil {
		return fmt.Errorf("could not set chip type: %w", err)
	}

	err = rdo.dev.setControlRegister(rdo.ctlreg)
	if err != nil {
		return fmt.Errorf("could not set control register: %w", err)
	}

	ctlreg, err := rdo.dev.getControlRegister()
	if err != nil {
		return fmt.Errorf("could not get control register: %w", err)
	}
	rdo.msg.Infof("ctl reg: 0x%x", ctlreg)

	err = rdo.dev.setPwr2PwrARegister(rdo.reg.p2pa)
	if err != nil {
		return fmt.Errorf("could not set pwr to A register: %w", err)
	}

	err = rdo.dev.setPwrA2PwrDRegister(rdo.reg.pa2pd)
	if err != nil {
		return fmt.Errorf("could not set A to D register: %w", err)
	}

	err = rdo.dev.setPwrD2DAQRegister(rdo.reg.pd2daq)
	if err != nil {
		return fmt.Errorf("could not set D to DAQ register: %w", err)
	}

	err = rdo.dev.setDAQ2PwrDRegister(rdo.reg.daq2pd)
	if err != nil {
		return fmt.Errorf("could not set DAQ to D register: %w", err)
	}

	err = rdo.dev.setPwrD2PwrARegister(rdo.reg.pd2pa)
	if err != nil {
		return fmt.Errorf("could not set D to A register: %w", err)
	}

	return err
}

func (rdo *Readout) configureChips(scFrame [][]byte) (uint32, error) {
	var frame []byte
	switch rdo.asic {
	case hardrocASIC:
		frame = make([]byte, hardrocV2SLCFrameSize)
	case microrocASIC:
		frame = make([]byte, microrocSLCFrameSize)
	default:
		return 0, fmt.Errorf("unknown ASIC kind %v", rdo.asic)
	}

	crc := crc16.New(nil)
	err := rdo.dev.hardrocCmdSLCWrite()
	if err != nil {
		return 0, fmt.Errorf("%s could not send start SLC command to DIF: %w",
			rdo.name, err,
		)
	}

	for i := rdo.nasics; i > 0; i-- {
		copy(frame, scFrame[i-1])
		_, err = crc.Write(frame)
		if err != nil {
			return 0, fmt.Errorf("%s could not update CRC-16: %w", rdo.name, err)
		}

		err = rdo.dev.cmdSLCWriteSingleSLCFrame(frame)
		if err != nil {
			return 0, fmt.Errorf("%s could not send SLC frame to DIF: %w",
				rdo.name, err,
			)
		}
	}

	crc16 := crc.Sum16()
	err = rdo.dev.hardrocCmdSLCWriteCRC(crc16)
	if err != nil {
		return 0, fmt.Errorf("%s could not send CRC 0x%x to SLC: %w",
			rdo.name, crc16, err,
		)
	}

	time.Sleep(400 * time.Millisecond) // was 500ms

	st, err := rdo.doReadSLCStatus()
	if err != nil {
		return 0, fmt.Errorf("%s could not read SLC status: %w",
			rdo.name, err,
		)
	}
	rdo.curSC = st

	return st, nil
}

func (rdo *Readout) Readout(p []byte) (int, error) {
	var (
		dif eformat.DIF
		w   = bwriter{p: p}
		dec = eformat.NewDecoder(uint8(rdo.difID), io.TeeReader(rdo.dev.ft, &w))
	)
	err := dec.Decode(&dif)
	if err != nil {
		return w.c, fmt.Errorf("%s could not decode DIF data: %w",
			rdo.name, err,
		)
	}

	return w.c, nil
}

func (rdo *Readout) doRefreshNumASICs() error {
	var (
		v  uint32
		l1 = uint8(rdo.nasics>>0) & 0xff
		l2 = uint8(rdo.nasics>>8) & 0xff
		l3 = uint8(rdo.nasics>>16) & 0xff
		l4 = uint8(rdo.nasics>>24) & 0xff
		n  = l1 + l2 + l3 + l4
	)

	f := func(n, l1, l2, l3, l4 uint8) uint32 {
		return uint32(n) + uint32(l1)<<8 + uint32(l2)<<14 + uint32(l3)<<20 + uint32(l4)<<26
	}
	switch rdo.asic {
	case microrocASIC:
		v = f(n, l1, l2, l3, l4)
	default:
		v = f(n, n, 0, 0, 0)
	}

	err := rdo.dev.usbRegWrite(0x05, v)
	if err != nil {
		return fmt.Errorf("could not refresh num-asics: %w", err)
	}

	return nil
}

func (rdo *Readout) doReadSLCStatus() (uint32, error) {
	st, err := rdo.dev.hardrocSLCStatusRead()
	if err != nil {
		return 0, fmt.Errorf("could not read SLC status: %w", err)
	}
	rdo.curSC = st
	return st, nil
}

type bwriter struct {
	p []byte
	c int
}

func (w *bwriter) Write(p []byte) (int, error) {
	if w.c >= len(w.p) {
		return 0, io.EOF
	}
	n := copy(w.p[w.c:], p)
	w.c += n
	return n, nil
}
