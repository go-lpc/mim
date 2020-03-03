// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dif

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ziutek/ftdi"
)

type ftdiDevice interface {
	Reset() error

	SetBitmode(iomask byte, mode ftdi.Mode) error
	SetFlowControl(flowctrl ftdi.FlowCtrl) error
	SetLatencyTimer(lt int) error
	SetWriteChunkSize(cs int) error
	SetReadChunkSize(cs int) error
	PurgeBuffers() error

	io.Writer
	io.Reader
	io.Closer
}

type device struct {
	vid uint16     // vendor ID
	pid uint16     // product ID
	ft  ftdiDevice // handle to the FTDI device
}

var (
	ftdiOpen = ftdiOpenImpl
)

func ftdiOpenImpl(vid, pid uint16) (ftdiDevice, error) {
	dev, err := ftdi.OpenFirst(int(vid), int(pid), ftdi.ChannelAny)
	return dev, err
}

func newDevice(vid, pid uint16) (*device, error) {
	ft, err := ftdiOpen(vid, pid)
	if err != nil {
		return nil, fmt.Errorf("could not open FTDI device (vid=0x%x, pid=0x%x): %w", vid, pid, err)
	}

	dev := &device{vid: vid, pid: pid, ft: ft}
	err = dev.init()
	if err != nil {
		ft.Close()
		return nil, fmt.Errorf("could not initialize FTDI device (vid=0x%x, pid=0x%x): %w", vid, pid, err)
	}

	return dev, nil
}

func (dev *device) init() error {
	var err error

	err = dev.ft.Reset()
	if err != nil {
		return fmt.Errorf("could not reset USB: %w", err)
	}

	err = dev.ft.SetBitmode(0, ftdi.ModeBitbang)
	if err != nil {
		return fmt.Errorf("could not disable bitbang: %w", err)
	}

	err = dev.ft.SetFlowControl(ftdi.FlowCtrlDisable)
	if err != nil {
		return fmt.Errorf("could not disable flow control: %w", err)
	}

	err = dev.ft.SetLatencyTimer(2)
	if err != nil {
		return fmt.Errorf("could not set latency timer to 2: %w", err)
	}

	err = dev.ft.SetWriteChunkSize(0xffff)
	if err != nil {
		return fmt.Errorf("could not set write chunk-size to 0xffff: %w", err)
	}

	err = dev.ft.SetReadChunkSize(0xffff)
	if err != nil {
		return fmt.Errorf("could not set read chunk-size to 0xffff: %w", err)
	}

	if dev.pid == 0x6014 {
		err = dev.ft.SetBitmode(0, ftdi.ModeReset)
		if err != nil {
			return fmt.Errorf("could not reset bit mode: %w", err)
		}
	}

	err = dev.ft.PurgeBuffers()
	if err != nil {
		return fmt.Errorf("could not purge USB buffers: %w", err)
	}

	return err
}

func (dev *device) close() error {
	return dev.ft.Close()
}

func (dev *device) usbRegRead(addr uint32) (uint32, error) {
	a := (addr | 0x4000) & 0x7fff
	p := []byte{uint8(a>>8) & 0xff, uint8(a>>0) & 0xff, 0, 0}

	n, err := dev.ft.Write(p[:2])
	switch {
	case err != nil:
		return 0, fmt.Errorf("could not write USB addr 0x%x: %w", addr, err)
	case n != len(p[:2]):
		return 0, fmt.Errorf("could not write USB addr 0x%x: %w", addr, io.ErrShortWrite)
	}

	_, err = io.ReadFull(dev.ft, p)
	if err != nil {
		return 0, fmt.Errorf("could not read register 0x%x: %w", addr, err)
	}

	v := binary.BigEndian.Uint32(p)
	return v, nil
}

func (dev *device) usbCmdWrite(cmd uint32) error {
	addr := cmd | 0x8000 // keep only 14 LSB, write, so bit 14=0,register mode, so bit 15=0
	buf := []byte{uint8(addr>>8) & 0xff, uint8(addr>>0) & 0xff}

	n, err := dev.ft.Write(buf)
	switch {
	case err != nil:
		return fmt.Errorf("could not write USB command 0x%x: %w", cmd, err)
	case n != len(buf):
		return fmt.Errorf("could not write USB command 0x%x: %w", cmd, io.ErrShortWrite)
	}

	return nil
}

func (dev *device) usbRegWrite(addr, v uint32) error {
	var (
		a = addr & 0x3fff
		p = make([]byte, 6)
	)

	binary.BigEndian.PutUint16(p[:2], uint16(a))
	binary.BigEndian.PutUint32(p[2:], uint32(v))
	and0xff(p)

	n, err := dev.ft.Write(p)
	switch {
	case err != nil:
		return fmt.Errorf("could not write USB register (0x%x, 0x%x): %w", addr, v, err)
	case n != len(p):
		return fmt.Errorf("could not write USB register (0x%x, 0x%x): %w", addr, v, io.ErrShortWrite)
	}
	return nil
}

func (dev *device) setChipTypeRegister(v uint32) error {
	return dev.usbRegWrite(0x00, v)
}

func (dev *device) setDIFID(v uint32) error {
	return dev.usbRegWrite(0x01, v)
}

func (dev *device) setControlRegister(v uint32) error {
	return dev.usbRegWrite(0x03, v)
}

func (dev *device) getControlRegister() (uint32, error) {
	return dev.usbRegRead(0x03)
}

func (dev *device) difCptReset() error {
	const addr = 0x03
	v, err := dev.usbRegRead(addr)
	if err != nil {
		return fmt.Errorf("could not read register 0x%x", addr)
	}

	v |= 0x2000
	err = dev.usbRegWrite(addr, v)
	if err != nil {
		return fmt.Errorf("could not write to register 0x%x", addr)
	}

	v &= 0xffffdfff
	err = dev.usbRegWrite(addr, v)
	if err != nil {
		return fmt.Errorf("could not write to register 0x%x", addr)
	}

	return nil
}

func (dev *device) setPwr2PwrARegister(v uint32) error  { return dev.usbRegWrite(0x40, v) }
func (dev *device) setPwrA2PwrDRegister(v uint32) error { return dev.usbRegWrite(0x41, v) }
func (dev *device) setPwrD2DAQRegister(v uint32) error  { return dev.usbRegWrite(0x42, v) }
func (dev *device) setDAQ2PwrDRegister(v uint32) error  { return dev.usbRegWrite(0x43, v) }
func (dev *device) setPwrD2PwrARegister(v uint32) error { return dev.usbRegWrite(0x44, v) }

func (dev *device) setEventsBetweenTemperatureReadout(v uint32) error {
	return dev.usbRegWrite(0x55, v)
}

func (dev *device) setAnalogConfigureRegister(v uint32) error {
	return dev.usbRegWrite(0x60, v)
}

func (dev *device) usbFwVersion() (uint32, error) {
	return dev.usbRegRead(0x100)
}

func (dev *device) hardrocFlushDigitalFIFO() error {
	return nil
}

func (dev *device) hardrocStopDigitalAcquisitionCommand() error {
	return dev.usbCmdWrite(0x02)
}

func (dev *device) hardrocSLCStatusRead() (uint32, error) {
	return dev.usbRegRead(0x06)
}

func (dev *device) hardrocCmdSLCWrite() error {
	return dev.usbCmdWrite(0x01)
}

func (dev *device) hardrocCmdSLCWriteCRC(v uint16) error {
	p := make([]byte, 2)
	binary.BigEndian.PutUint16(p, v)
	_, err := dev.ft.Write(p)
	return err
}

func (dev *device) cmdSLCWriteSingleSLCFrame(p []byte) error {
	_, err := dev.ft.Write(p)
	return err
}

func and0xff(p []byte) {
	for i := range p {
		p[i] &= 0xff
	}
}
