// Copyright 2021 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

/** TO DO LIST
 *
 * change HR_id numbering (1 to 8 instead of 0 to 7) (+ processing script)
 * change trigger_id numbering (start 1) (+ processing script)
 *
 */

//#cgo CFLAGS: -g -Wall -std=c99 -D_GNU_SOURCE=1 -I.
//#cgo CFLAGS arm: -I/build/soc_eds/ip/altera/hps/altera_hps/hwlib/include
//#cgo LDFLAGS arm: -static
//
//#include <stdlib.h>
//#include <string.h>
//#include <stdint.h>
//#include "device.h"
//#include "fpga.h"
import "C"

import (
	"fmt"
	"net"
	"strconv"
	"time"
	"unsafe"

	"github.com/go-lpc/mim/conddb"
)

type cdevice struct {
	ctx  *C.Device_t
	rfms []int         // list of enabled RFMs
	difs map[int]uint8 // map of EDA-slot->DIF/RFM-id
	cfg  config

	done chan int // signal to stop DAQ
}

var _ device = (*cdevice)(nil)

func newCDevice(devmem, odir, devshm string, opts ...Option) (*cdevice, error) {
	ctx := C.new_device()
	if ctx == nil {
		return nil, fmt.Errorf("ceda: could not create EDA device")
	}

	dev := &cdevice{
		ctx: ctx,
		cfg: newConfig(),
	}
	WithResetBCID(10 * time.Second)(&dev.cfg)
	WithDevSHM(devshm)(&dev.cfg)

	for _, opt := range opts {
		opt(&dev.cfg)
	}

	// setup RFMs indices from provided mask
	dev.rfms = nil
	for i := 0; i < nRFM; i++ {
		if (dev.cfg.daq.rfm>>i)&1 == 1 {
			dev.rfms = append(dev.rfms, i)
		}
	}

	return dev, nil
}

func (dev *cdevice) Close() error {
	if dev.ctx == nil {
		return nil
	}
	C.device_free(dev.ctx)
	dev.ctx = nil
	return nil
}

func (dev *cdevice) Boot(cfg []conddb.RFM) error {
	dev.rfms = nil
	dev.difs = make(map[int]uint8, nRFM)
	dev.cfg.daq.rfm = 0
	var trig uint8
	switch dev.cfg.daq.mode {
	case "dcc":
		trig = 0
	case "noise":
		trig = 1
	default:
		return fmt.Errorf("eda: unknown trig-mode: %q", dev.cfg.mode)
	}
	for _, rfm := range cfg {
		rc := C.device_boot_rfm(
			dev.ctx,
			C.uint8_t(rfm.ID), C.int(rfm.Slot),
			C.uint32_t(rfm.DAQ.RShaper),
			C.uint32_t(trig),
		)
		if rc != 0 {
			return fmt.Errorf(
				"ceda: could not boot RFM=%d slot=%d: err=%v",
				rfm.ID, rfm.Slot, rc,
			)
		}
		dev.rfms = append(dev.rfms, rfm.Slot)
		dev.difs[rfm.Slot] = uint8(rfm.ID)
		dev.cfg.daq.rfm |= (1 << rfm.Slot)
		dev.cfg.hr.rshaper = uint32(rfm.DAQ.RShaper)
	}
	return nil
}

func (dev *cdevice) ConfigureDIF(addr string, dif uint8, asics []conddb.ASIC) error {
	// FIXME(sbinet): handle hysteresis, make sure addrs are unique.
	dev.cfg.daq.addrs = append(dev.cfg.daq.addrs, addr)

	dev.setDBConfig(dif, asics)

	err := dev.configASICs(dif)
	if err != nil {
		return fmt.Errorf("eda: could not configure DIF=%d: %w", dif, err)
	}

	host, port_, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf(
			"ceda: could not infer rfm-socket host/port from %q: %w",
			addr, err,
		)
	}

	c_addr := C.CString(host)
	defer C.free(unsafe.Pointer(c_addr))

	port, err := strconv.Atoi(port_)
	if err != nil {
		return fmt.Errorf(
			"ceda: could not infer rfm-socket port from %q: %w", addr, err,
		)
	}

	rc := C.device_configure_dif(dev.ctx, C.uint8_t(dif), c_addr, C.int(port))
	if rc != 0 {
		return fmt.Errorf("ceda: could not configure EDA-DIF=%d: rc=%d", dif, rc)
	}

	return nil
}

func (dev *cdevice) setDBConfig(dif uint8, asics []conddb.ASIC) {
	dev.cfg.mode = "db"
	dev.cfg.hr.db.asics[dif] = make([]conddb.ASIC, len(asics))
	copy(dev.cfg.hr.db.asics[dif], asics)
}

func (dev *cdevice) configASICs(dif uint8) error {
	asics, ok := dev.cfg.hr.db.asics[dif]
	if !ok {
		return fmt.Errorf("ceda: could not find conddb configuration for DIF=%d", dif)
	}

	for i, asic := range asics {
		var (
			cfg = asic.HRConfig()
			ihr = uint32(i)
			n   = len(cfg)
		)
		for i, v := range cfg {
			dev.hrscSetBit(ihr, uint32(n-1-i), uint32(v))
		}
	}
	return nil
}

func (dev *cdevice) Initialize() error {
	rc := C.device_init_mmap(dev.ctx)
	if rc != 0 {
		return fmt.Errorf("ceda: could not initialize EDA device mmap rc=%d", rc)
	}

	rc = C.device_init_fpga(dev.ctx)
	if rc != 0 {
		return fmt.Errorf("ceda: could not initialize EDA device FPGA rc=%d", rc)
	}

	if dev.cfg.mode == "db" {
		err := dev.initHRTables()
		if err != nil {
			return fmt.Errorf("ceda: could not initialize EDA HRSC tables: %w", err)
		}
	}

	rc = C.device_init_hrsc(dev.ctx)
	if rc != 0 {
		return fmt.Errorf("ceda: could not initialize EDA device HRSC rc=%d", rc)
	}

	rc = C.device_init_scks(dev.ctx)
	if rc != 0 {
		return fmt.Errorf("ceda: could not initialize EDA device SCKS rc=%d", rc)
	}

	return nil
}

func (dev *cdevice) initHRTables() error {
	// for each active RFM, tune the configuration.
	for i, slot := range dev.rfms {
		rfm := uint32(dev.rfms[i])
		dif := dev.difs[slot]
		asics := dev.cfg.hr.db.asics[dif]
		// mask unused channels
		for hr := uint32(0); hr < nHR; hr++ {
			for ch := uint32(0); ch < nChans; ch++ {
				m0 := bitU64(asics[hr].Mask0, ch)
				m1 := bitU64(asics[hr].Mask1, ch)
				m2 := bitU64(asics[hr].Mask2, ch)

				mask := uint32(m0 | m1<<1 | m2<<2)
				dev.cfg.mask.table[64*(nHR*rfm+hr)+ch] = mask
			}
		}

		// set DAC thresholds
		for hr := uint32(0); hr < nHR; hr++ {
			th0 := uint32(asics[hr].B0)
			th1 := uint32(asics[hr].B1)
			th2 := uint32(asics[hr].B2)

			dev.cfg.daq.floor[3*(nHR*rfm+hr)+0] = th0
			dev.cfg.daq.floor[3*(nHR*rfm+hr)+1] = th1
			dev.cfg.daq.floor[3*(nHR*rfm+hr)+2] = th2
		}

		// set preamplifier gain
		for hr := uint32(0); hr < nHR; hr++ {
			for ch := uint32(0); ch < nChans; ch++ {
				v, err := strconv.ParseUint(string(asics[hr].PreAmpGain[2*ch:2*ch+2]), 16, 8)
				if err != nil {
					return err
				}
				gain := uint32(v)
				dev.cfg.preamp.gains[64*(nHR*rfm+hr)+ch] = gain
			}
		}
	}

	memcpy := func(dst *C.alt_u32, src []uint32) {
		C.memcpy(
			unsafe.Pointer(dst),
			unsafe.Pointer(&src[0]),
			C.size_t(len(src)*int(unsafe.Sizeof(src[0]))),
		)
	}
	memcpy(&dev.ctx.dac_floor_table[0], dev.cfg.daq.floor[:])
	memcpy(&dev.ctx.pa_gain_table[0], dev.cfg.preamp.gains[:])
	memcpy(&dev.ctx.mask_table[0], dev.cfg.mask.table[:])
	return nil
}

func (dev *cdevice) Start(run uint32) error {
	rc := C.device_start(dev.ctx, C.uint32_t(run))
	if rc != 0 {
		return fmt.Errorf("ceda: could not start EDA device: rc=%d", rc)
	}
	go dev.loop()
	return nil
}

func (dev *cdevice) loop() {
	dev.done = make(chan int)
	defer close(dev.done)

	C.device_loop(dev.ctx)
}

func (dev *cdevice) Stop() error {
	const timeout = 10 * time.Second
	tck := time.NewTimer(timeout)
	defer tck.Stop()

	C.device_stop_loop(dev.ctx)

	select {
	case <-dev.done:
	case <-tck.C:
		return fmt.Errorf("ceda: could not stop DAQ (timeout=%v)", timeout)
	}

	rc := C.device_stop(dev.ctx)
	if rc != 0 {
		return fmt.Errorf("ceda: could not stop EDA device: rc=%d", rc)
	}
	return nil
}

func (dev *cdevice) hrscSetBit(hr, addr, bit uint32) {
	C.HRSC_set_bit(C.alt_u32(hr), C.alt_u32(addr), C.alt_u32(bit))
}
