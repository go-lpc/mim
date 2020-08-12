// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/go-lpc/mim/eda/internal/regs"
	"github.com/go-lpc/mim/internal/mmap"
)

// TODO:
//  - send file to eda-srv
//  - send beat to eda-ctl

const (
	nRFM        = 4
	difIDOffset = 0x00

	nHR         = 8
	nBitsCfgHR  = 872
	nBytesCfgHR = 109
	szCfgHR     = 4 + nHR*nBytesCfgHR
	nChans      = 64

	daqBufferSize = nRFM * (26 + nHR*(2+128*20))
)

const (
	verbose = false
)

// Device represents an EDA board device.
type Device struct {
	msg  *log.Logger
	id   uint32 // [0,8)
	rfms []int  // list of enabled RFMs
	mem  struct {
		fd  *os.File
		lw  *mmap.Handle
		h2f *mmap.Handle
	}

	dir string
	run uint32

	err  error
	buf  []byte
	regs struct {
		pio struct {
			state  reg32
			ctrl   reg32
			pulser reg32

			chkSC [nRFM]reg32

			cntHit0  [nRFM]reg32
			cntHit1  [nRFM]reg32
			cntTrig  reg32
			cnt48MSB reg32
			cnt48LSB reg32
			cnt24    reg32
		}
		ramSC [nRFM]hrCfg

		fifo struct {
			daq    [nRFM]reg32
			daqCSR [nRFM]daqFIFO
		}
	}

	cfg struct {
		hr struct {
			fname   string
			rshaper uint32 // resistance shaper
			cshaper uint32 // capacity shaper

			buf  [szCfgHR]byte
			data []byte
		}

		daq struct {
			addr  string
			fname string
			floor [nRFM * nHR * 3]uint32
			delta uint32 // delta threshold
			rfm   uint32 // RFM ON mask
		}

		preamp struct {
			fname string
			gains [nRFM * nHR * nChans]uint32
		}

		mask struct {
			fname string
			table [nRFM * nHR * nChans]uint32
		}

		run struct {
			dir string
		}
	}

	daq struct {
		cycleID      [nRFM]uint32
		bcid48Offset uint32

		w   *wbuf    // DIF data buffer
		sck net.Conn // DIF data sink

		done chan int // signal to stop daq
	}
}

type Option func(*Device)

func WithThreshold(v uint32) Option {
	return func(dev *Device) {
		dev.cfg.daq.delta = v
	}
}

func WithRFMMask(v uint32) Option {
	return func(dev *Device) {
		dev.cfg.daq.rfm = v
	}
}

func WithRShaper(v uint32) Option {
	return func(dev *Device) {
		dev.cfg.hr.rshaper = v
	}
}

func WithCShaper(v uint32) Option {
	return func(dev *Device) {
		dev.cfg.hr.cshaper = v
	}
}

func WithDevSHM(dir string) Option {
	return func(dev *Device) {
		dev.cfg.run.dir = dir
	}
}

func WithDAQAddr(addr string) Option {
	return func(dev *Device) {
		dev.cfg.daq.addr = addr
	}
}

func WithConfigDir(dir string) Option {
	return func(dev *Device) {
		dev.cfg.hr.fname = filepath.Join(dir, "conf_base.csv")
		dev.cfg.daq.fname = filepath.Join(dir, "dac_floor_4rfm.csv")
		dev.cfg.preamp.fname = filepath.Join(dir, "pa_gain_4rfm.csv")
		dev.cfg.mask.fname = filepath.Join(dir, "mask_4rfm.csv")
	}
}

func NewDevice(fname string, runnbr uint32, odir string, opts ...Option) (*Device, error) {
	mem, err := os.OpenFile(fname, os.O_RDWR|os.O_SYNC, 0666)
	if err != nil {
		return nil, fmt.Errorf("eda: could not open %q: %w", fname, err)
	}
	defer func() {
		if err != nil {
			_ = mem.Close()
		}
	}()

	dev := &Device{
		msg: log.New(os.Stdout, "eda: ", 0),
		dir: odir,
		run: runnbr,
		buf: make([]byte, 4),
	}
	dev.mem.fd = mem
	dev.cfg.hr.cshaper = 3
	WithConfigDir("/dev/shm/config_base")(dev)
	WithDevSHM("/dev/shm")(dev)

	dev.cfg.hr.data = dev.cfg.hr.buf[4:]

	for _, opt := range opts {
		opt(dev)
	}

	// setup RFMs indices from provided mask
	dev.rfms = nil
	for i := 0; i < nRFM; i++ {
		if (dev.cfg.daq.rfm>>i)&1 == 1 {
			dev.rfms = append(dev.rfms, i)
		}
	}

	err = dev.mmapLwH2F()
	if err != nil {
		return nil, fmt.Errorf("eda: could not initialize lightweight HPS-to-FPGA bus: %w", err)
	}
	defer func() {
		if err != nil {
			_ = dev.mem.lw.Close()
		}
	}()

	err = dev.mmapH2F()
	if err != nil {
		return nil, fmt.Errorf("eda: could not initialize HPS-to-FPGA bus: %w", err)
	}
	defer func() {
		if err != nil {
			_ = dev.mem.h2f.Close()
		}
	}()

	if dev.cfg.daq.addr != "" {
		conn, err := net.Dial("tcp", dev.cfg.daq.addr)
		if err != nil {
			return nil, fmt.Errorf(
				"eda: could not dial DAQ data sink %q: %w",
				dev.cfg.daq.addr, err,
			)
		}
		dev.daq.sck = conn
	}

	return dev, nil
}

func (dev *Device) Configure() error {
	err := dev.hrscReadConf(dev.cfg.hr.fname, 0)
	if err != nil {
		return fmt.Errorf("eda: could load single-HR configuration file: %w", err)
	}

	err = dev.readThOffset(dev.cfg.daq.fname)
	if err != nil {
		return fmt.Errorf("eda: could not read floor thresholds: %w", err)
	}

	err = dev.readPreAmpGain(dev.cfg.preamp.fname)
	if err != nil {
		return fmt.Errorf("eda: could not read preamplifier gains: %w", err)
	}

	err = dev.readMask(dev.cfg.mask.fname)
	if err != nil {
		return fmt.Errorf("eda: could not read masks: %w", err)
	}

	return nil
}

func (dev *Device) Initialize() error {
	err := dev.initFPGA()
	if err != nil {
		return fmt.Errorf("eda: could not initialize FPGA: %w", err)
	}

	err = dev.initHR()
	if err != nil {
		return fmt.Errorf("eda: could not initialize HardRoc: %w", err)
	}

	return nil
}

func (dev *Device) initFPGA() error {
	// reset FPGA and set clock.
	err := dev.syncResetFPGA()
	if err != nil {
		return fmt.Errorf("eda: could not reset FPGA: %w", err)
	}
	time.Sleep(2 * time.Microsecond)
	cnt := 0
	max := 100
	for !dev.syncPLLLock() && cnt < max {
		time.Sleep(10 * time.Millisecond)
		cnt++
	}
	if cnt >= max {
		return fmt.Errorf("eda: could not lock PLL")
	}

	dev.msg.Printf("pll lock=%v\n", dev.syncPLLLock())

	// activate RFMs
	for _, rfm := range dev.rfms {
		err = dev.rfmOn(rfm)
		if err != nil {
			return fmt.Errorf("eda: could not activate RFM=%d: %w", rfm, err)
		}
		err = dev.rfmEnable(rfm)
		if err != nil {
			return fmt.Errorf("eda: could not enable RFM=%d: %w", rfm, err)
		}
	}
	time.Sleep(1 * time.Millisecond)

	ctrl := dev.regs.pio.ctrl.r()
	if dev.err != nil {
		return fmt.Errorf("eda: could not read control pio: %w", dev.err)
	}
	dev.msg.Printf("control pio=0x%x\n", ctrl)

	err = dev.syncSelectCmdDCC()
	if err != nil {
		return fmt.Errorf("eda: could not select DCC cmd: %w", err)
	}
	err = dev.syncEnableDCCBusy()
	if err != nil {
		return fmt.Errorf("eda: could not enable DCC busy: %w", err)
	}
	err = dev.syncEnableDCCRAMFull()
	if err != nil {
		return fmt.Errorf("eda: could not enable DCC RAM-full: %w", err)
	}

	return nil
}

func (dev *Device) initHR() error {
	// disable trig_out output pin (RFM v1 coupling problem)
	dev.hrscSetBit(0, 854, 0)

	dev.hrscSetRShaper(0, dev.cfg.hr.rshaper)
	dev.hrscSetCShaper(0, dev.cfg.hr.cshaper)

	dev.hrscCopyConf(1, 0)
	dev.hrscCopyConf(2, 0)
	dev.hrscCopyConf(3, 0)
	dev.hrscCopyConf(4, 0)
	dev.hrscCopyConf(5, 0)
	dev.hrscCopyConf(6, 0)
	dev.hrscCopyConf(7, 0)

	// set chip IDs
	for hr := uint32(0); hr < nHR; hr++ {
		dev.hrscSetChipID(hr, hr)
	}

	// for each active RFM, tune the configuration and send it.
	for _, rfm := range dev.rfms {
		// mask unused channels
		for hr := uint32(0); hr < nHR; hr++ {
			for ch := uint32(0); ch < nChans; ch++ {
				mask := dev.cfg.mask.table[nChans*(nHR*uint32(rfm)+hr)+ch]
				if verbose {
					dev.msg.Printf("%d      %d      %d\n", hr, ch, mask)
				}
				dev.hrscSetMask(hr, ch, mask)
			}
		}

		// set DAC thresholds
		if verbose {
			dev.msg.Printf("HR      thresh0     thresh1     thresh2\n")
		}
		for hr := uint32(0); hr < nHR; hr++ {
			th0 := dev.cfg.daq.floor[3*(nHR*uint32(rfm)+hr)+0] + dev.cfg.daq.delta
			th1 := dev.cfg.daq.floor[3*(nHR*uint32(rfm)+hr)+1] + dev.cfg.daq.delta
			th2 := dev.cfg.daq.floor[3*(nHR*uint32(rfm)+hr)+2] + dev.cfg.daq.delta
			if verbose {
				dev.msg.Printf("%d      %d      %d      %d\n", hr, th0, th1, th2)
			}
			dev.hrscSetDAC0(hr, th0)
			dev.hrscSetDAC1(hr, th1)
			dev.hrscSetDAC2(hr, th2)
		}

		// set preamplifier gain
		if verbose {
			dev.msg.Printf("HR      chan        pa_gain\n")
		}
		for hr := uint32(0); hr < nHR; hr++ {
			for ch := uint32(0); ch < nChans; ch++ {
				gain := dev.cfg.preamp.gains[nChans*hr+ch]
				if verbose {
					dev.msg.Printf("%d      %d      %d\n", hr, ch, gain)
				}
				dev.hrscSetPreAmp(hr, ch, gain)
			}
		}

		// send to HRs
		err := dev.hrscSetConfig(rfm)
		if err != nil {
			return fmt.Errorf(
				"eda: could not send configuration to HR (RFM=%d): %w",
				rfm, err,
			)
		}
		dev.msg.Printf("Hardroc configuration (RFM=%d): [done]\n", rfm)

		err = dev.hrscResetReadRegisters(rfm)
		if err != nil {
			return fmt.Errorf(
				"eda: could not reset read-registers for RFM=%d: %w",
				rfm, err,
			)
		}
		dev.msg.Printf("read-registers reset (RFM=%d): [done]\n", rfm)
	}

	// let DACs stabilize
	time.Sleep(1 * time.Second)

	return nil
}

func (dev *Device) initRun() error {
	// save run-dependant settings
	dev.msg.Printf(
		"thresh_delta=%d, Rshaper=%d, RFM=%d\n",
		dev.cfg.daq.delta,
		dev.cfg.hr.rshaper,
		dev.cfg.daq.rfm,
	)

	fname := path.Join(dev.dir, fmt.Sprintf("settings_%03d.csv", dev.run))
	f, err := os.Create(fname)
	if err != nil {
		return fmt.Errorf(
			"eda: could not create settings file %q: %w",
			fname, err,
		)
	}
	defer f.Close()

	fmt.Fprintf(f,
		"thresh_delta=%d; Rshaper=%d; RFM=%d; ip_addr=%s; run_id=%d\n",
		dev.cfg.daq.delta,
		dev.cfg.hr.rshaper,
		dev.cfg.daq.rfm,
		dev.cfg.daq.addr,
		dev.run,
	)
	err = f.Close()
	if err != nil {
		return fmt.Errorf(
			"eda: could not close settings file %q: %w",
			fname, err,
		)
	}

	dev.msg.Printf("-----------------RUN NB %d-----------------\n", dev.run)
	fname = path.Join(dev.dir, fmt.Sprintf("hr_sc_%03d.csv", dev.run))
	err = dev.hrscWriteConfHRs(fname)
	if err != nil {
		return fmt.Errorf(
			"eda: could not write HR config file %q: %w",
			fname, err,
		)
	}

	err = dev.syncResetHR()
	if err != nil {
		return fmt.Errorf("eda: could not reset hardroc: %w", err)
	}

	return nil
}

func (dev *Device) Start() error {
	err := dev.initRun()
	if err != nil {
		return fmt.Errorf("eda: could not init run: %w", err)
	}

	// FIXME(sbinet): reset-BCID
	var dccCmd uint32 = 0xe
	for dccCmd != regs.CMD_RESET_BCID {
		dccCmd = dev.syncDCCCmdMem()
	}

	dev.msg.Printf("sync-state: %[1]d 0x%[1]x\n", dev.syncState())
	for _, rfm := range dev.rfms {
		err = dev.DumpCounters(dev.msg.Writer(), rfm)
		if err != nil {
			return fmt.Errorf("eda: could not dump-counters: %w", err)
		}
	}

	err = dev.cntReset()
	if err != nil {
		return fmt.Errorf("eda: could not reset counters: %w", err)
	}

	err = dev.cntStart()
	if err != nil {
		return fmt.Errorf("eda: could not start counters: %w", err)
	}

	for _, rfm := range dev.rfms {
		err = dev.daqFIFOInit(rfm)
		if err != nil {
			return fmt.Errorf("eda: could not initialize DAQ FIFO (RFM=%d): %w", rfm, err)
		}
	}

	err = dev.syncArmFIFO()
	if err != nil {
		return fmt.Errorf("eda: could not arm FIFO: %w", err)
	}

	dev.daq.done = make(chan int)

	go dev.loop()
	return nil
}

func (dev *Device) loop() {
	var (
		w      = dev.msg.Writer()
		printf = fmt.Fprintf
		errorf = func(format string, args ...interface{}) {
			dev.err = fmt.Errorf(format, args...)
			dev.msg.Printf("%+v", dev.err)
		}
		cycle int
		err   error
	)

	if dev.daq.sck != nil {
		defer dev.daq.sck.Close()
	}

	dev.daq.w = &wbuf{
		p: make([]byte, daqBufferSize),
	}

	for {
		// wait until new readout is started
		printf(w, "trigger %07d, state: acq-", cycle)
		for !dev.syncFPGARO() {
			select {
			case <-dev.daq.done:
				dev.daq.done <- 1
				return
			default:
			}
		}

		printf(w, "ro-") // readout of HR

		// wait until readout is done.
		for !dev.syncFIFOReady() {
			select {
			case <-dev.daq.done:
				dev.daq.done <- 1
				return
			default:
			}
		}
		printf(w, "cp-") // copy

		// read hardroc data
		for _, rfm := range dev.rfms {
			dev.daqWriteDIFData(dev.daq.w, rfm)
		}
		err = dev.syncAckFIFO()
		if err != nil {
			errorf("eda: could not ACK FIFO: %w", err)
			return
		}
		printf(w, "tx-")
		err = dev.daqSendDIFData()
		if err != nil {
			errorf("eda: could not send DIF data: %w", err)
			return
		}
		printf(w, "\n")
		cycle++

		select {
		case <-dev.daq.done:
			dev.daq.done <- 1
			return
		default:
		}
	}
}

func (dev *Device) Stop() error {
	const timeout = 10 * time.Second
	tck := time.NewTimer(timeout)
	defer tck.Stop()

	select {
	case dev.daq.done <- 1:
		<-dev.daq.done
	case <-tck.C:
		return fmt.Errorf("eda: could not stop DAQ (timeout=%v)", timeout)
	}

	if dev.err != nil {
		return fmt.Errorf("eda: error during DAQ: %w", dev.err)
	}

	err := dev.cntStop()
	if err != nil {
		return fmt.Errorf("eda: could not stop counters: %w", err)
	}

	return nil
}

func (dev *Device) Close() error {
	if dev.mem.fd == nil {
		return nil
	}

	var (
		errLW  = dev.mem.lw.Close()
		errH2F = dev.mem.h2f.Close()
		errMem = dev.mem.fd.Close()
	)

	dev.mem.fd = nil
	dev.mem.h2f = nil
	dev.mem.lw = nil

	if errMem != nil {
		return fmt.Errorf("eda: could not close device mem file: %w", errMem)
	}

	if errLW != nil {
		return fmt.Errorf("eda: could not close mmap lw-h2f: %w", errLW)
	}

	if errH2F != nil {
		return fmt.Errorf("eda: could not close mmap h2f: %w", errH2F)
	}

	return nil
}

func (dev *Device) DumpFIFOStatus(w io.Writer, rfm int) error {
	var (
		fifo   = &dev.regs.fifo.daqCSR[rfm]
		buf    = bufio.NewWriter(w)
		err    error
		printf = func(format string, args ...interface{}) {
			_, e := fmt.Fprintf(buf, format, args...)
			if err == nil {
				err = e
			}
		}
	)
	defer buf.Flush()

	printf("---- FIFO status -------\n")
	printf("fill level:\t\t%d\n", fifo.r(regs.ALTERA_AVALON_FIFO_LEVEL_REG))

	reg := fifo.r(regs.ALTERA_AVALON_FIFO_STATUS_REG)
	printf("istatus:")
	printf("\t full:\t %d", bit32(reg, 0))
	printf("\t empty:\t %d", bit32(reg, 1))
	printf("\t almost full:\t %d", bit32(reg, 2))
	printf("\t almost empty:\t %d", bit32(reg, 3))
	printf("\t overflow:\t %d", bit32(reg, 4))
	printf("\t underflow:\t %d\n", bit32(reg, 5))

	reg = fifo.r(regs.ALTERA_AVALON_FIFO_EVENT_REG)
	printf("event:  ")
	printf("\t full:\t %d", bit32(reg, 0))
	printf("\t empty:\t %d", bit32(reg, 1))
	printf("\t almost full:\t %d", bit32(reg, 2))
	printf("\t almost empty:\t %d", bit32(reg, 3))
	printf("\t overflow:\t %d", bit32(reg, 4))
	printf("\t underflow:\t %d\n", bit32(reg, 5))

	reg = fifo.r(regs.ALTERA_AVALON_FIFO_IENABLE_REG)
	printf("ienable:")
	printf("\t full:\t %d", bit32(reg, 0))
	printf("\t empty:\t %d", bit32(reg, 1))
	printf("\t almost full:\t %d", bit32(reg, 2))
	printf("\t almost empty:\t %d", bit32(reg, 3))
	printf("\t overflow:\t %d", bit32(reg, 4))
	printf("\t underflow:\t %d\n", bit32(reg, 5))

	printf("almostfull:\t\t%d\n", fifo.r(regs.ALTERA_AVALON_FIFO_ALMOSTFULL_REG))
	printf("almostempty:\t\t%d\n", fifo.r(regs.ALTERA_AVALON_FIFO_ALMOSTEMPTY_REG))
	printf("\n\n")

	if err != nil {
		return fmt.Errorf("eda: could not dump FIFO status: %w", err)
	}

	err = buf.Flush()
	if err != nil {
		return fmt.Errorf("eda: could not dump FIFO status: %w", err)
	}

	return nil
}

func (dev *Device) DumpCounters(w io.Writer, rfm int) error {
	var (
		buf    = bufio.NewWriter(w)
		err    error
		printf = func(format string, args ...interface{}) {
			_, e := fmt.Fprintf(buf, format, args...)
			if err == nil {
				err = e
			}
		}
	)
	defer buf.Flush()

	printf("<counters rfm=%d>\n", rfm)
	printf("#cycle_id;cnt_hit0;cnt_hit1;trig;")
	printf("cnt48_msb;cnt48_lsb;cnt24\n")
	printf("%d;%d;%d;%d;",
		dev.daq.cycleID[rfm],
		dev.cntHit0(rfm),
		dev.cntHit1(rfm),
		dev.cntTrig(),
	)
	printf("%d;%d;%d\n",
		dev.cntBCID48MSB(), dev.cntBCID48LSB(), dev.cntBCID24(),
	)

	if err != nil {
		return fmt.Errorf("eda: could not dump counters: %w", err)
	}

	err = buf.Flush()
	if err != nil {
		return fmt.Errorf("eda: could not dump counters: %w", err)
	}
	return nil
}

func (dev *Device) DumpConfig(w io.Writer, rfm int) error {
	buf := bufio.NewWriter(w)
	defer buf.Flush()

	ram := &dev.regs.ramSC[rfm]
	for i := 0; i < szCfgHR; i++ {
		j := 8 * (nHR*nBytesCfgHR - i - 1)
		v := ram.r(i)
		_, err := fmt.Fprintf(buf, "%d\t%x\n", j, v)
		if err != nil {
			return fmt.Errorf("eda: could not dump config: %w", err)
		}
	}
	return nil
}

func (dev *Device) DumpRegisters(w io.Writer) error {
	regs := &dev.regs

	fmt.Fprintf(w, "pio.state=       0x%08x\n", regs.pio.state.r())
	fmt.Fprintf(w, "pio.ctrl=        0x%08x\n", regs.pio.ctrl.r())
	fmt.Fprintf(w, "pio.pulser=      0x%08x\n", regs.pio.pulser.r())

	fmt.Fprintf(w, "pio.cnt.hit0[0]= 0x%08x\n", regs.pio.cntHit0[0].r())
	fmt.Fprintf(w, "pio.cnt.hit0[1]= 0x%08x\n", regs.pio.cntHit0[1].r())
	fmt.Fprintf(w, "pio.cnt.hit0[2]= 0x%08x\n", regs.pio.cntHit0[2].r())
	fmt.Fprintf(w, "pio.cnt.hit0[3]= 0x%08x\n", regs.pio.cntHit0[3].r())

	fmt.Fprintf(w, "pio.cnt.hit1[0]= 0x%08x\n", regs.pio.cntHit1[0].r())
	fmt.Fprintf(w, "pio.cnt.hit1[1]= 0x%08x\n", regs.pio.cntHit1[1].r())
	fmt.Fprintf(w, "pio.cnt.hit1[2]= 0x%08x\n", regs.pio.cntHit1[2].r())
	fmt.Fprintf(w, "pio.cnt.hit1[3]= 0x%08x\n", regs.pio.cntHit1[3].r())

	fmt.Fprintf(w, "pio.cnt.trig=    0x%08x\n", regs.pio.cntTrig.r())
	fmt.Fprintf(w, "pio.cnt48MSB=    0x%08x\n", regs.pio.cnt48MSB.r())
	fmt.Fprintf(w, "pio.cnt48LSB=    0x%08x\n", regs.pio.cnt48LSB.r())

	fmt.Fprintf(w, "fifo.daqCSR[0]=  0x%08x\n", regs.fifo.daqCSR[0].r(0))
	fmt.Fprintf(w, "fifo.daqCSR[1]=  0x%08x\n", regs.fifo.daqCSR[1].r(0))
	fmt.Fprintf(w, "fifo.daqCSR[2]=  0x%08x\n", regs.fifo.daqCSR[2].r(0))
	fmt.Fprintf(w, "fifo.daqCSR[3]=  0x%08x\n", regs.fifo.daqCSR[3].r(0))

	names := [...]string{
		0: "idle",
		1: "reset_cnt48",
		2: "reset_cnt24",
		3: "acquiring",
		4: "ramfull",
		5: "start readout",
		6: "wait end_readout",
		7: "fifo ready",
		8: "stop run",
	}
	state := dev.syncState()
	fmt.Fprintf(w, "synchro FSM state= %d (%s)\n", state, names[state])
	return dev.err
}
