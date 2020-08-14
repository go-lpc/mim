// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-lpc/mim/eda/internal/regs"
	"github.com/go-lpc/mim/internal/eformat"
	"github.com/go-lpc/mim/internal/mmap"
	"golang.org/x/sys/unix"
)

func (dev *Device) mmapLwH2F() error {
	data, err := unix.Mmap(
		int(dev.mem.fd.Fd()),
		regs.LW_H2F_BASE, regs.LW_H2F_SPAN,
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_SHARED,
	)
	if err != nil {
		return fmt.Errorf("eda: could not mmap lw-h2f: %w", err)
	}
	if data == nil || len(data) != regs.LW_H2F_SPAN {
		return fmt.Errorf("eda: invalid mmap'd data: %d", len(data))
	}
	dev.mem.lw = mmap.HandleFrom(data)

	err = dev.bindLwH2F()
	if err != nil {
		return fmt.Errorf("eda: could not read lw-h2f registers: %w", err)
	}

	return nil
}

func (dev *Device) mmapH2F() error {
	data, err := unix.Mmap(
		int(dev.mem.fd.Fd()),
		regs.H2F_BASE, regs.H2F_SPAN, unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_SHARED,
	)
	if err != nil {
		return fmt.Errorf("eda: could not mmap h2f: %w", err)
	}
	if data == nil || len(data) != regs.H2F_SPAN {
		return fmt.Errorf("eda: invalid mmap'd data: %d", len(data))
	}
	dev.mem.h2f = mmap.HandleFrom(data)

	err = dev.bindH2F()
	if err != nil {
		return fmt.Errorf("eda: could not read h2f registers: %w", err)
	}

	return nil
}

func (dev *Device) readThOffset(fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("eda: could not open threshold offsets file: %w", err)
	}
	defer f.Close()

	var (
		scan = bufio.NewScanner(f)
		line int
		rfm  uint32
		hr   uint32
	)
	for scan.Scan() {
		line++
		txt := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(txt, "#") || txt == "" {
			continue
		}
		toks := strings.Split(txt, ";")
		if len(toks) != 5 {
			return fmt.Errorf(
				"eda: invalid threshold offsets file:%d: line=%q",
				line, txt,
			)
		}
		v, err := strconv.ParseUint(toks[0], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse RFM id %q (line:%d): %w",
				toks[0], line, err,
			)
		}
		if uint32(v) != rfm {
			return fmt.Errorf(
				"eda: invalid RFM id=%d (line:%d), want=%d",
				v, line, rfm,
			)
		}

		v, err = strconv.ParseUint(toks[1], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse HR id %q (line:%d): %w",
				toks[1], line, err,
			)
		}
		if uint32(v) != hr {
			return fmt.Errorf(
				"eda: invalid HR id=%d (line:%d), want=%d",
				v, line, hr,
			)
		}

		v, err = strconv.ParseUint(toks[2], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse threshold value dac0 for (RFM=%d,HR=%d) (line:%d:%q): %w",
				rfm, hr, line, toks[2], err,
			)
		}
		dev.cfg.daq.floor[3*(nHR*rfm+hr)+0] = uint32(v)

		v, err = strconv.ParseUint(toks[3], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse threshold value dac1 for (RFM=%d,HR=%d) (line:%d:%q): %w",
				rfm, hr, line, toks[3], err,
			)
		}
		dev.cfg.daq.floor[3*(nHR*rfm+hr)+1] = uint32(v)

		v, err = strconv.ParseUint(toks[4], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse threshold value dac2 for (RFM=%d,HR=%d) (line:%d:%q): %w",
				rfm, hr, line, toks[4], err,
			)
		}
		dev.cfg.daq.floor[3*(nHR*rfm+hr)+2] = uint32(v)

		hr++
		if hr >= nHR {
			hr = 0
			rfm++
		}
	}

	err = scan.Err()
	if err != nil {
		return fmt.Errorf("eda: error while parsing threshold offsets: %w", err)
	}

	return nil
}

func (dev *Device) readPreAmpGain(fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("eda: could not open preamp-gain file: %w", err)
	}
	defer f.Close()

	var (
		scan = bufio.NewScanner(f)
		line int
		rfm  uint32
		hr   uint32
		ch   uint32
	)
	for scan.Scan() {
		line++
		txt := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(txt, "#") || txt == "" {
			continue
		}
		toks := strings.Split(txt, ";")
		if len(toks) != 4 {
			return fmt.Errorf(
				"eda: invalid preamp-gain file:%d: line=%q",
				line, txt,
			)
		}

		v, err := strconv.ParseUint(toks[0], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse RFM id %q (line:%d): %w",
				toks[0], line, err,
			)
		}
		if uint32(v) != rfm {
			return fmt.Errorf(
				"eda: invalid RFM id=%d (line:%d), want=%d",
				v, line, rfm,
			)
		}

		v, err = strconv.ParseUint(toks[1], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse HR id %q (line:%d): %w",
				toks[1], line, err,
			)
		}
		if uint32(v) != hr {
			return fmt.Errorf(
				"eda: invalid HR id=%d (line:%d), want=%d",
				v, line, hr,
			)
		}

		v, err = strconv.ParseUint(toks[2], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse chan %q (line:%d): %w",
				toks[2], line, err,
			)
		}
		if uint32(v) != ch {
			return fmt.Errorf(
				"eda: invalid chan id=%d (line:%d), want=%d",
				v, line, ch,
			)
		}

		v, err = strconv.ParseUint(toks[3], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse gain for (RFM=%d,HR=%d,ch=%d) (line:%d:%q): %w",
				rfm, hr, ch, line, toks[3], err,
			)
		}
		dev.cfg.preamp.gains[nChans*(nHR*rfm+hr)+ch] = uint32(v)
		ch++

		if ch >= nChans {
			ch = 0
			hr++
		}
		if hr >= nHR {
			hr = 0
			rfm++
		}
	}

	err = scan.Err()
	if err != nil {
		return fmt.Errorf("eda: error while parsing preamp-gains: %w", err)
	}

	return nil
}

func (dev *Device) readMask(fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("eda: could not open mask file: %w", err)
	}
	defer f.Close()

	var (
		scan = bufio.NewScanner(f)
		line int
		rfm  uint32
		hr   uint32
		ch   uint32
	)
	for scan.Scan() {
		line++
		txt := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(txt, "#") || txt == "" {
			continue
		}
		toks := strings.Split(txt, ";")
		if len(toks) != 4 {
			return fmt.Errorf(
				"eda: invalid mask file:%d: line=%q",
				line, txt,
			)
		}

		v, err := strconv.ParseUint(toks[0], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse RFM id %q (line:%d): %w",
				toks[0], line, err,
			)
		}
		if uint32(v) != rfm {
			return fmt.Errorf(
				"eda: invalid RFM id=%d (line:%d), want=%d",
				v, line, rfm,
			)
		}

		v, err = strconv.ParseUint(toks[1], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse HR id %q (line:%d): %w",
				toks[1], line, err,
			)
		}
		if uint32(v) != hr {
			return fmt.Errorf(
				"eda: invalid HR id=%d (line:%d), want=%d",
				v, line, hr,
			)
		}

		v, err = strconv.ParseUint(toks[2], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse chan %q (line:%d): %w",
				toks[2], line, err,
			)
		}
		if uint32(v) != ch {
			return fmt.Errorf(
				"eda: invalid chan id=%d (line:%d), want=%d",
				v, line, ch,
			)
		}

		v, err = strconv.ParseUint(toks[3], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse mask for (RFM=%d,HR=%d,ch=%d) (line:%d:%q): %w",
				rfm, hr, ch, line, toks[3], err,
			)
		}
		dev.cfg.mask.table[nChans*(nHR*rfm+hr)+ch] = uint32(v)
		ch++

		if ch >= nChans {
			ch = 0
			hr++
		}
		if hr >= nHR {
			hr = 0
			rfm++
		}
	}

	err = scan.Err()
	if err != nil {
		return fmt.Errorf("eda: error while parsing masks: %w", err)
	}

	return nil
}

func (dev *Device) bindLwH2F() error {
	dev.regs.pio.state = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_STATE_IN)
	dev.regs.pio.ctrl = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CTRL_OUT)
	dev.regs.pio.pulser = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_PULSER)

	dev.regs.ramSC[0] = newHRCfg(dev, dev.mem.lw, regs.LW_H2F_RAM_SC_RFM0)
	dev.regs.ramSC[1] = newHRCfg(dev, dev.mem.lw, regs.LW_H2F_RAM_SC_RFM1)
	dev.regs.ramSC[2] = newHRCfg(dev, dev.mem.lw, regs.LW_H2F_RAM_SC_RFM2)
	dev.regs.ramSC[3] = newHRCfg(dev, dev.mem.lw, regs.LW_H2F_RAM_SC_RFM3)

	dev.regs.pio.chkSC[0] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_SC_CHECK_RFM0)
	dev.regs.pio.chkSC[1] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_SC_CHECK_RFM1)
	dev.regs.pio.chkSC[2] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_SC_CHECK_RFM2)
	dev.regs.pio.chkSC[3] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_SC_CHECK_RFM3)

	dev.regs.pio.cntHit0[0] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT_HIT0_RFM0)
	dev.regs.pio.cntHit0[1] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT_HIT0_RFM1)
	dev.regs.pio.cntHit0[2] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT_HIT0_RFM2)
	dev.regs.pio.cntHit0[3] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT_HIT0_RFM3)

	dev.regs.pio.cntHit1[0] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT_HIT1_RFM0)
	dev.regs.pio.cntHit1[1] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT_HIT1_RFM1)
	dev.regs.pio.cntHit1[2] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT_HIT1_RFM2)
	dev.regs.pio.cntHit1[3] = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT_HIT1_RFM3)

	dev.regs.pio.cntTrig = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT_TRIG)
	dev.regs.pio.cnt48MSB = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT48_MSB)
	dev.regs.pio.cnt48LSB = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT48_LSB)
	dev.regs.pio.cnt24 = newReg32(dev, dev.mem.lw, regs.LW_H2F_PIO_CNT24)

	return dev.err
}

func (dev *Device) bindH2F() error {
	dev.regs.fifo.daq[0] = newReg32(dev, dev.mem.h2f, regs.H2F_FIFO_DAQ_RFM0)
	dev.regs.fifo.daq[1] = newReg32(dev, dev.mem.h2f, regs.H2F_FIFO_DAQ_RFM1)
	dev.regs.fifo.daq[2] = newReg32(dev, dev.mem.h2f, regs.H2F_FIFO_DAQ_RFM2)
	dev.regs.fifo.daq[3] = newReg32(dev, dev.mem.h2f, regs.H2F_FIFO_DAQ_RFM3)

	dev.regs.fifo.daqCSR[0] = newDAQFIFO(dev, dev.mem.h2f, regs.H2F_FIFO_DAQ_CSR_RFM0)
	dev.regs.fifo.daqCSR[1] = newDAQFIFO(dev, dev.mem.h2f, regs.H2F_FIFO_DAQ_CSR_RFM1)
	dev.regs.fifo.daqCSR[2] = newDAQFIFO(dev, dev.mem.h2f, regs.H2F_FIFO_DAQ_CSR_RFM2)
	dev.regs.fifo.daqCSR[3] = newDAQFIFO(dev, dev.mem.h2f, regs.H2F_FIFO_DAQ_CSR_RFM3)

	return dev.err
}

func (dev *Device) readU32(r io.ReaderAt, off int64) uint32 {
	if dev.err != nil {
		return 0
	}
	_, dev.err = r.ReadAt(dev.buf[:4], off)
	if dev.err != nil {
		dev.err = fmt.Errorf("eda: could not read register 0x%x: %w", off, dev.err)
		return 0
	}
	return binary.LittleEndian.Uint32(dev.buf[:4])
}

func (dev *Device) writeU32(w io.WriterAt, off int64, v uint32) {
	if dev.err != nil {
		return
	}
	binary.LittleEndian.PutUint32(dev.buf[:4], v)
	_, dev.err = w.WriteAt(dev.buf[:4], off)
	if dev.err != nil {
		dev.err = fmt.Errorf("eda: could not write register 0x%x: %w", off, dev.err)
		return
	}
}

func (dev *Device) rfmOn(rfm int) error {
	var mask uint32
	switch rfm {
	case 0:
		mask = regs.O_ON_OFF_RFM0
	case 1:
		mask = regs.O_ON_OFF_RFM1
	case 2:
		mask = regs.O_ON_OFF_RFM2
	case 3:
		mask = regs.O_ON_OFF_RFM3
	default:
		panic(fmt.Errorf("eda: invalid RFM id=%d", rfm))
	}
	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= mask
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not switch ON RFM=%d: %w", rfm, dev.err)
	}
	return nil
}

// func (dev *Device) rfmOff(rfm int) error {
// 	var mask uint32
// 	switch rfm {
// 	case 0:
// 		mask = regs.O_ON_OFF_RFM0
// 	case 1:
// 		mask = regs.O_ON_OFF_RFM1
// 	case 2:
// 		mask = regs.O_ON_OFF_RFM2
// 	case 3:
// 		mask = regs.O_ON_OFF_RFM3
// 	default:
// 		panic(fmt.Errorf("eda: invalid RFM id=%d", rfm))
// 	}
// 	ctrl := dev.regs.pio.ctrl.r()
// 	ctrl &= ^mask
// 	dev.regs.pio.ctrl.w(ctrl)
//
// 	if dev.err != nil {
// 		return fmt.Errorf("eda: could not switch OFF RFM=%d: %w", rfm, dev.err)
// 	}
// 	return nil
// }

func (dev *Device) rfmEnable(rfm int) error {
	var mask uint32
	switch rfm {
	case 0:
		mask = regs.O_ENA_RFM0
	case 1:
		mask = regs.O_ENA_RFM1
	case 2:
		mask = regs.O_ENA_RFM2
	case 3:
		mask = regs.O_ENA_RFM3
	default:
		panic(fmt.Errorf("eda: invalid RFM id=%d", rfm))
	}
	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= mask
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not enable RFM=%d: %w", rfm, dev.err)
	}
	return nil
}

// func (dev *Device) rfmDisable(rfm int) error {
// 	var mask uint32
// 	switch rfm {
// 	case 0:
// 		mask = regs.O_ENA_RFM0
// 	case 1:
// 		mask = regs.O_ENA_RFM1
// 	case 2:
// 		mask = regs.O_ENA_RFM2
// 	case 3:
// 		mask = regs.O_ENA_RFM3
// 	default:
// 		panic(fmt.Errorf("eda: invalid RFM id=%d", rfm))
// 	}
// 	ctrl := dev.regs.pio.ctrl.r()
// 	ctrl &= ^mask
// 	dev.regs.pio.ctrl.w(ctrl)
//
// 	if dev.err != nil {
// 		return fmt.Errorf("eda: could not disable RFM=%d: %w", rfm, dev.err)
// 	}
// 	return nil
// }

func (dev *Device) syncResetFPGA() error {
	dev.regs.pio.ctrl.w(regs.O_RESET)
	dev.msg.Printf("reset FPGA")
	time.Sleep(1 * time.Microsecond)
	dev.regs.pio.ctrl.w(0x00000000)
	time.Sleep(1 * time.Microsecond)

	if dev.err != nil {
		return fmt.Errorf("eda: could not reset FPGA: %w", dev.err)
	}
	return nil
}

func (dev *Device) syncResetHR() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= regs.O_RESET_HR
	dev.regs.pio.ctrl.w(ctrl)
	time.Sleep(1 * time.Microsecond)

	ctrl = dev.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_RESET_HR)
	dev.regs.pio.ctrl.w(ctrl)
	time.Sleep(1 * time.Microsecond)

	if dev.err != nil {
		return fmt.Errorf("eda: could not reset HR: %w", dev.err)
	}
	return nil
}

func (dev *Device) syncPLLLock() bool {
	state := dev.regs.pio.state.r()
	return state&regs.O_PLL_LCK == regs.O_PLL_LCK
}

func (dev *Device) syncState() uint32 {
	state := dev.regs.pio.state.r()
	return (state >> regs.SHIFT_SYNCHRO_STATE) & 0xF
}

// func (dev *Device) syncSelectCmdSoft() error {
// 	ctrl := dev.regs.pio.ctrl.r()
// 	ctrl |= regs.O_SEL_CMD_SOURCE
// 	dev.regs.pio.ctrl.w(ctrl)
//
// 	if dev.err != nil {
// 		return fmt.Errorf("eda: could not synchronize cmd-soft: %w", dev.err)
// 	}
// 	return nil
// }

func (dev *Device) syncSelectCmdDCC() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_SEL_CMD_SOURCE)
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not synchronize cmd-dcc: %w", dev.err)
	}
	return nil
}

// func (dev *Device) syncSetCmd(cmd uint32) error {
// 	ctrl := dev.regs.pio.ctrl.r()
// 	ctrl &= ^uint32(0xf << regs.SHIFT_CMD_CODE) // reset 4 bits
// 	ctrl |= (0xf & cmd) << regs.SHIFT_CMD_CODE  // set command
//
// 	dev.regs.pio.ctrl.w(ctrl)
// 	time.Sleep(2 * time.Microsecond)
//
// 	ctrl &= ^uint32(0xf << regs.SHIFT_CMD_CODE)  // reset 4 bits
// 	ctrl |= regs.CMD_IDLE << regs.SHIFT_CMD_CODE // set idle command
// 	dev.regs.pio.ctrl.w(ctrl)
//
// 	if dev.err != nil {
// 		return fmt.Errorf("eda: could not set command 0x%x: %w", cmd, dev.err)
// 	}
// 	return nil
// }
//
// func (dev *Device) syncResetBCID() error {
// 	return dev.syncSetCmd(regs.CMD_RESET_BCID)
// }
//
// func (dev *Device) syncStart() error {
// 	return dev.syncSetCmd(regs.CMD_START_ACQ)
// }
//
// func (dev *Device) syncStop() error {
// 	return dev.syncSetCmd(regs.CMD_STOP_ACQ)
// }
//
// func (dev *Device) syncRAMFullExt() error {
// 	return dev.syncSetCmd(regs.CMD_RAMFULL_EXT)
// }
//
// func (dev *Device) syncDigitalRO() error {
// 	return dev.syncSetCmd(regs.CMD_DIGITAL_RO)
// }

func (dev *Device) syncArmFIFO() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= regs.O_HPS_BUSY
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not arm FIFO: %w", dev.err)
	}
	return nil
}

func (dev *Device) syncAckFIFO() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_HPS_BUSY)
	dev.regs.pio.ctrl.w(ctrl) // falling edge on hps busy
	for dev.syncState() != regs.S_IDLE {
		// when FPGA ready for new acquisition
	}
	ctrl = dev.regs.pio.ctrl.r()
	dev.regs.pio.ctrl.w(ctrl | regs.O_HPS_BUSY) // re-arming

	if dev.err != nil {
		return fmt.Errorf("eda: could not ACK FIFO: %w", dev.err)
	}
	return nil
}

func (dev *Device) syncDCCCmdMem() uint32 {
	cnt := dev.regs.pio.cnt24.r()
	return (cnt >> regs.SHIFT_CMD_CODE_MEM) & 0xf
}

// func (dev *Device) syncDCCCmdNow() uint32 {
// 	cnt := dev.regs.pio.cnt24.r()
// 	return (cnt >> regs.SHIFT_CMD_CODE_NOW) & 0xf
// }
//
// func (dev *Device) syncRAMFull() bool {
// 	state := dev.syncState()
// 	return state == regs.S_RAMFULL
// }
//
// func (dev *Device) syncFPGARO() bool {
// 	state := dev.syncState()
// 	return state == regs.S_START_RO || state == regs.S_WAIT_END_RO
// }
//
// func (dev *Device) syncFIFOReady() bool {
// 	state := dev.syncState()
// 	return state == regs.S_FIFO_READY
// }
//
// func (dev *Device) syncRunStopped() bool {
// 	state := dev.syncState()
// 	return state == regs.S_STOP_RUN
// }

// func (dev *Device) syncHRTransmitOn(rfm int) bool {
// 	state := dev.regs.pio.state.r()
// 	switch rfm {
// 	case 0:
// 		return (state & regs.O_HR_TRANSMITON_0) == regs.O_HR_TRANSMITON_0
// 	case 1:
// 		return (state & regs.O_HR_TRANSMITON_1) == regs.O_HR_TRANSMITON_1
// 	case 2:
// 		return (state & regs.O_HR_TRANSMITON_2) == regs.O_HR_TRANSMITON_2
// 	case 3:
// 		return (state & regs.O_HR_TRANSMITON_3) == regs.O_HR_TRANSMITON_3
// 	}
// 	panic(fmt.Errorf("eda: invalid RFM id %d", rfm))
// }
//
// func (dev *Device) syncChipsAt(rfm int) bool {
// 	state := dev.regs.pio.state.r()
// 	switch rfm {
// 	case 0:
// 		return (state & regs.O_CHIPSAT_0) == regs.O_CHIPSAT_0
// 	case 1:
// 		return (state & regs.O_CHIPSAT_1) == regs.O_CHIPSAT_1
// 	case 2:
// 		return (state & regs.O_CHIPSAT_2) == regs.O_CHIPSAT_2
// 	case 3:
// 		return (state & regs.O_CHIPSAT_3) == regs.O_CHIPSAT_3
// 	}
// 	panic(fmt.Errorf("eda: invalid RFM id %d", rfm))
// }
//
// func (dev *Device) syncHREndRO(rfm int) bool {
// 	state := dev.regs.pio.state.r()
// 	switch rfm {
// 	case 0:
// 		return (state & regs.O_HR_END_RO_0) == regs.O_HR_END_RO_0
// 	case 1:
// 		return (state & regs.O_HR_END_RO_1) == regs.O_HR_END_RO_1
// 	case 2:
// 		return (state & regs.O_HR_END_RO_2) == regs.O_HR_END_RO_2
// 	case 3:
// 		return (state & regs.O_HR_END_RO_3) == regs.O_HR_END_RO_3
// 	}
// 	panic(fmt.Errorf("eda: invalid RFM id %d", rfm))
// }

func (dev *Device) syncEnableDCCBusy() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= regs.O_ENA_DCC_BUSY
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not enable DCC-BUSY: %w", dev.err)
	}
	return nil
}

func (dev *Device) syncEnableDCCRAMFull() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= regs.O_ENA_DCC_RAMFULL
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not enable DCC-RAMFULL: %w", dev.err)
	}
	return nil
}

// func (dev *Device) trigSelectThreshold0() error {
// 	ctrl := dev.regs.pio.ctrl.r()
// 	ctrl &= ^uint32(regs.O_SEL_TRIG_THRESH)
// 	dev.regs.pio.ctrl.w(ctrl)
//
// 	if dev.err != nil {
// 		return fmt.Errorf("eda: could not select threshold-0: %w", dev.err)
// 	}
// 	return nil
// }
//
// func (dev *Device) trigSelectThreshold1() error {
// 	ctrl := dev.regs.pio.ctrl.r()
// 	ctrl |= regs.O_SEL_TRIG_THRESH
// 	dev.regs.pio.ctrl.w(ctrl)
//
// 	if dev.err != nil {
// 		return fmt.Errorf("eda: could not select threshold-1: %w", dev.err)
// 	}
// 	return nil
// }
//
// func (dev *Device) trigEnable() error {
// 	ctrl := dev.regs.pio.ctrl.r()
// 	ctrl |= regs.O_ENA_TRIG
// 	dev.regs.pio.ctrl.w(ctrl)
//
// 	if dev.err != nil {
// 		return fmt.Errorf("eda: could not enable trigger: %w", dev.err)
// 	}
// 	return nil
// }
//
// func (dev *Device) trigDisable() error {
// 	ctrl := dev.regs.pio.ctrl.r()
// 	ctrl &= ^uint32(regs.O_ENA_TRIG)
// 	dev.regs.pio.ctrl.w(ctrl)
//
// 	if dev.err != nil {
// 		return fmt.Errorf("eda: could not disable trigger: %w", dev.err)
// 	}
// 	return nil
// }

func (dev *Device) cntReset() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= regs.O_RST_SCALERS
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not reset scalers: %w", dev.err)
	}

	time.Sleep(1 * time.Microsecond)

	ctrl &= ^uint32(regs.O_RST_SCALERS)
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not clear scalers: %w", dev.err)
	}
	return nil
}

func (dev *Device) cntStart() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= regs.O_ENA_SCALERS
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not start scalers: %w", dev.err)
	}
	return nil
}

func (dev *Device) cntStop() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_ENA_SCALERS)
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not stop scalers: %w", dev.err)
	}
	return nil
}

func (dev *Device) cntHit0(rfm int) uint32 {
	return dev.regs.pio.cntHit0[rfm].r()
}

func (dev *Device) cntHit1(rfm int) uint32 {
	return dev.regs.pio.cntHit1[rfm].r()
}

func (dev *Device) cntTrig() uint32 {
	return dev.regs.pio.cntTrig.r()
}

func (dev *Device) cntBCID24() uint32 {
	v := dev.regs.pio.cnt24.r()
	return v & 0xffffff
}

func (dev *Device) cntBCID48MSB() uint32 {
	v := dev.regs.pio.cnt48MSB.r()
	return v & 0xffff
}

func (dev *Device) cntBCID48LSB() uint32 {
	return dev.regs.pio.cnt48LSB.r()
}

// func (dev *Device) cntSave(w io.Writer, rfm int) error {
// 	var (
// 		i   = 0
// 		buf = make([]byte, 7*4)
// 		dif = difIDOffset + ((dev.id & 7) << 3) + (uint32(rfm) & 3)
// 		u32 = (dif << 24) | (dev.daq.cycleID[rfm] & 0x00ffffff)
// 	)
//
// 	binary.BigEndian.PutUint32(buf[i:], u32)
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], dev.cntBCID48MSB())
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], dev.cntBCID48LSB())
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], dev.cntBCID24())
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], dev.cntHit0(rfm))
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], dev.cntHit1(rfm))
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], dev.cntTrig())
// 	if dev.err != nil {
// 		return fmt.Errorf("eda: could not read counters: %w", dev.err)
// 	}
//
// 	n, err := w.Write(buf)
// 	if err != nil {
// 		return fmt.Errorf("eda: could not save counters: %w", err)
// 	}
//
// 	if n != len(buf) {
// 		return fmt.Errorf("eda: could not save counters: %w", io.ErrShortWrite)
// 	}
// 	return nil
// }

// hardroc slow control

func (dev *Device) hrscSelectSlowControl() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_SELECT_SC_RR)
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not select slow-control: %w", dev.err)
	}
	return nil
}

func (dev *Device) hrscSelectReadRegister() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= regs.O_SELECT_SC_RR
	dev.regs.pio.ctrl.w(ctrl)

	if dev.err != nil {
		return fmt.Errorf("eda: could not select read-register: %w", dev.err)
	}
	return nil
}

func div(num, den uint32) (quo, rem uint32) {
	quo = num / den
	rem = num % den
	return quo, rem
}

func (dev *Device) hrscSetBit(hr, addr, bit uint32) {
	// byte address 0 corresponds to the last register (addr 864 to 871)
	// of the last Hardroc (pos=nHR-1)
	var (
		quo, rem = div(addr, nHR)

		i   = (nHR-1-hr)*nBytesCfgHR + nBytesCfgHR - 1 - quo
		v   = dev.cfg.hr.data[i]
		off = rem

		// bit address increases from LSB to MSB
		mask1 = uint8(0x01 << off)
		mask2 = uint8((0x1 & bit) << off)
	)
	v &= ^mask1 // reset target bit
	v |= mask2  // set target bit = "bit" argument
	dev.cfg.hr.data[i] = v
}

func (dev *Device) hrscGetBit(hr, addr uint32) uint32 {
	// byte address 0 corresponds to the last register (addr 864 to 871)
	// of the last Hardroc (pos=nHR-1)
	var (
		quo, rem = div(addr, nHR)

		i   = (nHR-1-hr)*nBytesCfgHR + nBytesCfgHR - 1 - quo
		v   = dev.cfg.hr.data[i]
		off = rem
	)
	return uint32((v >> off) & 0x01)
}

func (dev *Device) hrscSetWord(hr, addr, nbits, v uint32) {
	for i := uint32(0); i < nbits; i++ {
		// scan LSB to MSB
		bit := (v >> i) & 0x01
		dev.hrscSetBit(hr, addr+i, bit)
	}
}

func (dev *Device) hrscSetWordMSB2LSB(hr, addr, nbits, v uint32) {
	for i := uint32(0); i < nbits; i++ {
		// scan MSB to LSB
		bit := (v >> i) & 0x01
		dev.hrscSetBit(hr, addr+nbits-1-i, bit)
	}
}

func (dev *Device) hrscReadConf(fname string, hr uint32) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("eda: could not open config file %q: %w", fname, err)
	}
	defer f.Close()

	var (
		addr uint32
		bit  uint32
		cnt  = uint32(nBitsCfgHR - 1)
		sc   = bufio.NewScanner(f)
		line int
	)

	for sc.Scan() {
		line++
		txt := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(txt, "#") {
			continue
		}
		toks := strings.Split(txt, ";")

		if len(toks) != 5 {
			return fmt.Errorf("eda: invalid config file:%d: line=%q", line, txt)
		}
		v, err := strconv.ParseUint(toks[0], 10, 32)
		if err != nil {
			return fmt.Errorf("eda: could not parse address %q in %q: %w", toks[0], txt, err)
		}
		addr = uint32(v)

		v, err = strconv.ParseUint(toks[4], 10, 32)
		if err != nil {
			return fmt.Errorf("eda: could not parse bit %q in %q: %w", toks[4], txt, err)
		}
		bit = uint32(v)

		if addr != cnt {
			return fmt.Errorf(
				"eda: invalid bit address line:%d: got=0x%x, want=0x%x",
				line, addr, cnt,
			)
		}
		cnt--

		dev.hrscSetBit(hr, addr, bit)
		if addr == 0 {
			return nil
		}
	}
	err = sc.Err()
	if err != nil && err != io.EOF {
		return fmt.Errorf("eda: could not scan config file %q: %w", fname, err)
	}

	return fmt.Errorf("eda: reached end of config file %q before last bit", fname)
}

func (dev *Device) hrscCopyConf(hrDst, hrSrc uint32) {
	var (
		isrc = (nHR - 1 - hrSrc) * nBytesCfgHR
		idst = (nHR - 1 - hrDst) * nBytesCfgHR

		dst = dev.cfg.hr.data[idst : idst+nBytesCfgHR]
		src = dev.cfg.hr.data[isrc : isrc+nBytesCfgHR]
	)
	copy(dst, src)
}

func (dev *Device) hrscReadConfHRs(fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("eda: could not open hr-sc %q: %w", fname, err)
	}
	defer f.Close()

	var (
		hr     uint32
		addr   uint32
		bit    uint32
		cntBit = int64(nBitsCfgHR - 1)
		cntHR  = int64(nHR - 1)
		sc     = bufio.NewScanner(f)
		line   int
	)

	for sc.Scan() && cntBit >= 0 && cntHR >= 0 {
		line++
		txt := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(txt, "#") {
			continue
		}
		toks := strings.Split(txt, ";")
		if len(toks) != 3 {
			return fmt.Errorf("eda: invalid HR config file:%d: line=%q", line, txt)
		}
		v, err := strconv.ParseUint(toks[0], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse HR address %q in %q: %w",
				toks[0], txt, err,
			)
		}
		hr = uint32(v)

		v, err = strconv.ParseUint(toks[1], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse bit address %q in %q: %w",
				toks[1], txt, err,
			)
		}
		addr = uint32(v)

		v, err = strconv.ParseUint(toks[2], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse bit value %q in %q: %w",
				toks[2], txt, err,
			)
		}
		bit = uint32(v)

		if int64(addr) != cntBit {
			return fmt.Errorf(
				"eda: invalid bit address line:%d: got=%d, want=%d",
				line, addr, cntBit,
			)
		}

		if int64(hr) != cntHR {
			return fmt.Errorf(
				"eda: invalid HR address line:%d: got=%d, want=%d",
				line, hr, cntHR,
			)
		}

		cntBit--
		if cntBit < 0 {
			cntBit = nBitsCfgHR - 1
			cntHR--
		}

		dev.hrscSetBit(hr, addr, bit)
		if addr == 0 && hr == 0 {
			return nil
		}
	}

	err = sc.Err()
	if err != nil && err != io.EOF {
		return fmt.Errorf("eda: could not scan config file %q: %w", fname, err)
	}

	return fmt.Errorf("eda: reached end of config file %q before last bit", fname)
}

func (dev *Device) hrscWriteConfHRs(fname string) error {
	f, err := os.Create(fname)
	if err != nil {
		return fmt.Errorf("eda: could not create hr-sc file %q: %w", fname, err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for i := 0; i < nHR; i++ {
		for j := 0; j < nBitsCfgHR; j++ {
			var (
				hr   = uint32(nHR - 1 - i)
				addr = uint32(nBitsCfgHR - 1 - j)
				v    = dev.hrscGetBit(hr, addr)
			)
			fmt.Fprintf(w, "%d;%d;%d\n", hr, addr, v)
		}
	}

	err = w.Flush()
	if err != nil {
		return fmt.Errorf("eda: could not flush hr-sc file %q: %w", fname, err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("eda: could not close hr-sc file %q: %w", fname, err)
	}
	return nil
}

// // hrscSetCtest switches the test capacitor (1=closed).
// func (dev *Device) hrscSetCtest(hr, ch, v uint32) {
// 	dev.hrscSetBit(hr, ch, v&0x01)
// }
//
// func (dev *Device) hrscSetAllCtestOff() {
// 	for hr := uint32(0); hr < nHR; hr++ {
// 		for ch := uint32(0); ch < nChans; ch++ {
// 			dev.hrscSetCtest(hr, ch, 0)
// 		}
// 	}
// }

func (dev *Device) hrscSetPreAmp(hr, ch, v uint32) {
	addr := nChans + nHR*ch
	dev.hrscSetWord(hr, addr, 8, v)
}

// func (dev *Device) hrscSetCmdFSB2(hr, ch, v uint32) {
// 	// fast shaper 2 gain
// 	addr := 587 + 4*ch
// 	dev.hrscSetWordMSB2LSB(hr, addr, 4, ^v) // "cmdb" register bits are active-low
// }
//
// func (dev *Device) hrscSetCmdFSB1(hr, ch, v uint32) {
// 	// fast shaper 2 gain
// 	addr := 595 + 4*ch
// 	dev.hrscSetWordMSB2LSB(hr, addr, 4, ^v) // "cmdb" register bits are active-low
// }

func (dev *Device) hrscSetMask(hr, ch, v uint32) {
	addr := 618 + 3*ch
	dev.hrscSetWord(hr, addr, 3, v)
}

func (dev *Device) hrscSetChipID(hr, v uint32) {
	dev.hrscSetWordMSB2LSB(hr, 810, 8, v)
}

func (dev *Device) hrscSetDAC0(hr, v uint32) {
	dev.hrscSetWord(hr, 818, 10, v)
}

func (dev *Device) hrscSetDAC1(hr, v uint32) {
	dev.hrscSetWord(hr, 828, 10, v)
}

func (dev *Device) hrscSetDAC2(hr, v uint32) {
	dev.hrscSetWord(hr, 838, 10, v)
}

// func (dev *Device) hrscSetDACCoarse(hr uint32) {
// 	dev.hrscSetWord(hr, 848, 10, 0)
// }
//
// func (dev *Device) hrscSetDACFine(hr uint32) {
// 	dev.hrscSetWord(hr, 848, 10, 1)
// }

func (dev *Device) hrscSetCShaper(hr, v uint32) {
	dev.hrscSetBit(hr, 611, v&1)      // sw_50f0 = b0
	dev.hrscSetBit(hr, 602, v&1)      // sw_50f1 = b0
	dev.hrscSetBit(hr, 594, v&1)      // sw_50f2 = b0
	dev.hrscSetBit(hr, 610, (v>>1)&1) // sw_100f0 = b1
	dev.hrscSetBit(hr, 601, (v>>1)&1) // sw_100f1 = b1
	dev.hrscSetBit(hr, 593, (v>>1)&1) // sw_100f2 = b1
}

func (dev *Device) hrscSetRShaper(hr, v uint32) {
	dev.hrscSetBit(hr, 609, v&1)      // sw_100k0 = b0
	dev.hrscSetBit(hr, 600, v&1)      // sw_100k1 = b0
	dev.hrscSetBit(hr, 592, v&1)      // sw_100k2 = b0
	dev.hrscSetBit(hr, 608, (v>>1)&1) // sw_50k0 = b1
	dev.hrscSetBit(hr, 599, (v>>1)&1) // sw_50k1 = b1
	dev.hrscSetBit(hr, 591, (v>>1)&1) // sw_50k2 = b1
}

func (dev *Device) hrscSetConfig(rfm int) error {
	ctrl := dev.regs.pio.chkSC[rfm].r()
	if dev.err != nil {
		return fmt.Errorf(
			"eda: could not read check-sc register (rfm=%d): %w",
			rfm, dev.err,
		)
	}

	switch ctrl {
	case 0xcafefade:
		ctrl = 0x36baffe5
	default:
		ctrl = 0xcafefade
	}
	dev.cfg.hr.buf[0] = byte((ctrl >> 24) & 0xff)
	dev.cfg.hr.buf[1] = byte((ctrl >> 16) & 0xff)
	dev.cfg.hr.buf[2] = byte((ctrl >> 8) & 0xff)
	dev.cfg.hr.buf[3] = byte(ctrl & 0xff)

	// reset sc
	err := dev.hrscSelectSlowControl()
	if err != nil {
		return fmt.Errorf("eda: could not select slow-control (rfm=%d): %w",
			rfm, err,
		)
	}

	err = dev.hrscResetSC()
	if err != nil {
		return fmt.Errorf("eda: could not reset slow-control (rfm=%d): %w",
			rfm, err,
		)
	}

	if dev.hrscSCDone(rfm) {
		return fmt.Errorf("eda: could not reset slow control (rfm=%d): sc-not-done", rfm)
	}

	// copy to FPGA
	_, err = dev.regs.ramSC[rfm].w(dev.cfg.hr.buf[:szCfgHR])
	if err != nil {
		return fmt.Errorf(
			"eda: could not write slow-control cfg to FPGA (rfm=%d): %w",
			rfm, err,
		)
	}

	// trigger the slow control serializer
	err = dev.hrscStartSC(rfm)
	if err != nil {
		return fmt.Errorf(
			"eda: could not start slow-control serializer (rfm=%d): %w",
			rfm, err,
		)
	}

	// check loop-back header
	time.Sleep(10 * time.Microsecond)
	for !dev.hrscSCDone(rfm) {
		time.Sleep(10 * time.Microsecond)
	}

	chk := dev.regs.pio.chkSC[rfm].r()
	if dev.err != nil {
		return fmt.Errorf(
			"eda: could not read slow-control loopback register (rfm=%d): %w",
			rfm, dev.err,
		)
	}

	if chk != ctrl {
		return fmt.Errorf(
			"eda: invalid loopback register (rfm=%d): got=0x%x, want=0x%x",
			rfm, chk, ctrl,
		)
	}

	return nil
}

func (dev *Device) hrscResetReadRegisters(rfm int) error {
	ctrl := dev.regs.pio.chkSC[rfm].r()
	if dev.err != nil {
		return fmt.Errorf(
			"eda: could not read check-sc register (rfm=%d): %w",
			rfm, dev.err,
		)
	}

	switch ctrl {
	case 0xcafefade:
		ctrl = 0x36baffe5
	default:
		ctrl = 0xcafefade
	}
	var (
		buf [szCfgHR]byte
		off = nHR*nBytesCfgHR - nChans
	)
	buf[off+0] = byte((ctrl >> 24) & 0xff)
	buf[off+1] = byte((ctrl >> 16) & 0xff)
	buf[off+2] = byte((ctrl >> 8) & 0xff)
	buf[off+3] = byte(ctrl & 0xff)

	// reset sc
	err := dev.hrscSelectReadRegister()
	if err != nil {
		return fmt.Errorf(
			"eda: could select read-register (rfm=%d): %w",
			rfm, err,
		)
	}

	err = dev.hrscResetSC()
	if err != nil {
		return fmt.Errorf("eda: could not reset slow-control (rfm=%d): %w",
			rfm, err,
		)
	}

	if dev.hrscSCDone(rfm) {
		return fmt.Errorf("eda: could not reset slow control (rfm=%d)", rfm)
	}

	// copy to FPGA
	_, err = dev.regs.ramSC[rfm].w(buf[:szCfgHR])
	if err != nil {
		return fmt.Errorf(
			"eda: could not write slow-control cfg to FPGA (rfm=%d): %w",
			rfm, err,
		)
	}

	// trigger the slow control serializer
	time.Sleep(10 * time.Microsecond)
	err = dev.hrscStartSC(rfm)
	if err != nil {
		return fmt.Errorf(
			"eda: could not start slow-control serializer (rfm=%d): %w",
			rfm, err,
		)
	}

	// check loop-back header
	time.Sleep(10 * time.Microsecond)
	for !dev.hrscSCDone(rfm) {
		time.Sleep(10 * time.Microsecond)
	}

	chk := dev.regs.pio.chkSC[rfm].r()
	if dev.err != nil {
		return fmt.Errorf(
			"eda: could not read slow-control loopback register (rfm=%d): %w",
			rfm, dev.err,
		)
	}

	if chk != ctrl {
		return fmt.Errorf(
			"eda: invalid loopback register (rfm=%d): got=0x%x, want=0x%x",
			rfm, chk, ctrl,
		)
	}

	err = dev.hrscSelectSlowControl()
	if err != nil {
		return fmt.Errorf(
			"eda: could not select slow-control (rfm=%d): %w",
			rfm, err,
		)
	}

	return nil
}

// func (dev *Device) hrscSetReadRegister(rfm, ch int) error {
// 	ctrl := dev.regs.pio.chkSC[rfm].r()
// 	if dev.err != nil {
// 		return fmt.Errorf(
// 			"eda: could not read check-sc register (rfm=%d): %w",
// 			rfm, dev.err,
// 		)
// 	}
//
// 	switch ctrl {
// 	case 0xcafefade:
// 		ctrl = 0x36baffe5
// 	default:
// 		ctrl = 0xcafefade
// 	}
// 	var (
// 		buf [szCfgHR]byte
// 		off = nHR*nBytesCfgHR - nChans
// 	)
// 	buf[off+0] = byte((ctrl >> 24) & 0xff)
// 	buf[off+1] = byte((ctrl >> 16) & 0xff)
// 	buf[off+2] = byte((ctrl >> 8) & 0xff)
// 	buf[off+3] = byte(ctrl & 0xff)
//
// 	// reset sc
// 	err := dev.hrscSelectReadRegister()
// 	if err != nil {
// 		return fmt.Errorf(
// 			"eda: could select read-register (rfm=%d): %w",
// 			rfm, err,
// 		)
// 	}
//
// 	err = dev.hrscResetSC()
// 	if err != nil {
// 		return fmt.Errorf("eda: could not reset slow-control (rfm=%d): %w",
// 			rfm, err,
// 		)
// 	}
//
// 	if !dev.hrscSCDone(rfm) {
// 		return fmt.Errorf("eda: could not reset slow control (rfm=%d)", rfm)
// 	}
//
// 	// select the same channel for all HR's
// 	var (
// 		quo, rem = div(uint32(ch), nHR) // channels: 0->64*nHR-1
// 		v        = byte(0x1 << rem)
// 	)
// 	for i := 0; i < nHR; i++ {
// 		off := nHR*nBytesCfgHR + 3 - i*nHR - int(quo) // last byte -> channels [0,8)
// 		buf[off] = v
// 	}
//
// 	// copy to FPGA
// 	_, err = dev.regs.ramSC[rfm].w(buf[:szCfgHR])
// 	if err != nil {
// 		return fmt.Errorf(
// 			"eda: could not write slow-control cfg to FPGA (rfm=%d): %w",
// 			rfm, err,
// 		)
// 	}
//
// 	// trigger the slow control serializer
// 	err = dev.hrscStartSC(rfm)
// 	if err != nil {
// 		return fmt.Errorf(
// 			"eda: could not start slow-control serializer (rfm=%d): %w",
// 			rfm, err,
// 		)
// 	}
//
// 	// check loop-back header
// 	time.Sleep(10 * time.Microsecond)
// 	for !dev.hrscSCDone(rfm) {
// 		time.Sleep(10 * time.Microsecond)
// 	}
//
// 	chk := dev.regs.pio.chkSC[rfm].r()
// 	if dev.err != nil {
// 		return fmt.Errorf(
// 			"eda: could not read slow-control loopback register (rfm=%d): %w",
// 			rfm, dev.err,
// 		)
// 	}
//
// 	if chk != ctrl {
// 		return fmt.Errorf(
// 			"eda: invalid loopback register (rfm=%d): got=0x%x, want=0x%x",
// 			rfm, chk, ctrl,
// 		)
// 	}
//
// 	err = dev.hrscSelectSlowControl()
// 	if err != nil {
// 		return fmt.Errorf(
// 			"eda: could not select slow-control (rfm=%d): %w",
// 			rfm, err,
// 		)
// 	}
//
// 	return nil
// }

func (dev *Device) hrscResetSC() error {
	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= regs.O_RESET_SC
	dev.regs.pio.ctrl.w(ctrl)
	if dev.err != nil {
		return fmt.Errorf("eda: could not reset slow-control: %w", dev.err)
	}

	time.Sleep(1 * time.Microsecond)

	ctrl = dev.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_RESET_SC)
	dev.regs.pio.ctrl.w(ctrl)
	if dev.err != nil {
		return fmt.Errorf("eda: could not reset slow-control: %w", dev.err)
	}

	time.Sleep(1 * time.Microsecond)

	return nil
}

func (dev *Device) hrscStartSC(rfm int) error {
	var (
		mask uint32
	)

	switch rfm {
	case 0:
		mask = regs.O_START_SC_0
	case 1:
		mask = regs.O_START_SC_1
	case 2:
		mask = regs.O_START_SC_2
	case 3:
		mask = regs.O_START_SC_3
	default:
		return fmt.Errorf("eda: start slow-control: invalid RFM id %d", rfm)
	}

	ctrl := dev.regs.pio.ctrl.r()
	ctrl |= mask
	dev.regs.pio.ctrl.w(ctrl)
	if dev.err != nil {
		return fmt.Errorf(
			"eda: could read/write pio ctrl mask=0x%x: %w",
			mask,
			dev.err,
		)
	}

	time.Sleep(1 * time.Microsecond)

	ctrl = dev.regs.pio.ctrl.r()
	ctrl &= ^mask
	dev.regs.pio.ctrl.w(ctrl)
	if dev.err != nil {
		return fmt.Errorf(
			"eda: could read/write pio ctrl ^mask=0x%x: %w",
			mask,
			dev.err,
		)
	}

	return nil
}

func (dev *Device) hrscSCDone(rfm int) bool {
	var mask uint32
	switch rfm {
	case 0:
		mask = regs.O_SC_DONE_0
	case 1:
		mask = regs.O_SC_DONE_1
	case 2:
		mask = regs.O_SC_DONE_2
	case 3:
		mask = regs.O_SC_DONE_3
	default:
		panic(fmt.Errorf("eda: invalid RFM ID=%d", rfm))
	}

	return (dev.regs.pio.state.r() & mask) == mask
}

func bit32(word, digit uint32) uint32 {
	return (word >> digit) & 0x1
}

func (dev *Device) daqFIFOInit(rfm int) error {
	fifo := &dev.regs.fifo.daqCSR[rfm]

	// clear event reg (write 1 to each field)
	fifo.w(regs.ALTERA_AVALON_FIFO_EVENT_REG, regs.ALTERA_AVALON_FIFO_EVENT_ALL)

	// disable interrupts
	fifo.w(regs.ALTERA_AVALON_FIFO_IENABLE_REG, 0)

	// set "almostfull" to maxsize+1
	fifo.w(regs.ALTERA_AVALON_FIFO_ALMOSTFULL_REG, 5080+1)

	// set "almostempty"
	fifo.w(regs.ALTERA_AVALON_FIFO_ALMOSTEMPTY_REG, 2)

	if dev.err != nil {
		return fmt.Errorf("eda: could not initialize DAQ FIFO: %w", dev.err)
	}
	return nil
}

// func (dev *Device) daqFIFOPrintStatus(rfm int) {
// 	reg := dev.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_STATUS_REG)
// 	switch dev.err {
// 	case nil:
// 		dev.msg.Printf("fifo/status-reg[rfm=%d]= 0x%x\n", rfm, reg)
// 	default:
// 		dev.msg.Printf("fifo/status-reg[rfm=%d]= %+v\n", rfm, dev.err)
// 	}
// }
//
// func (dev *Device) daqFIFOPrintEvent(rfm int) {
// 	reg := dev.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_EVENT_REG)
// 	switch dev.err {
// 	case nil:
// 		dev.msg.Printf("fifo/event-reg[rfm=%d]= 0x%x\n", rfm, reg)
// 	default:
// 		dev.msg.Printf("fifo/event-reg[rfm=%d]= %+v\n", rfm, dev.err)
// 	}
// }
//
// func (dev *Device) daqFIFOClearEvent(rfm int) error {
// 	// clear event register: write 1 to each field.
// 	dev.regs.fifo.daqCSR[rfm].w(
// 		regs.ALTERA_AVALON_FIFO_EVENT_REG,
// 		regs.ALTERA_AVALON_FIFO_EVENT_ALL,
// 	)
// 	if dev.err != nil {
// 		return fmt.Errorf(
// 			"eda: could not clear DAQ FIFO event register (rfm=%d): %w",
// 			rfm, dev.err,
// 		)
// 	}
// 	return nil
// }
//
// func (dev *Device) daqFIFOInValid(rfm int) bool {
// 	var mask uint32
// 	switch rfm {
// 	case 0:
// 		mask = regs.O_FIFO_IN_VALID_0
// 	case 1:
// 		mask = regs.O_FIFO_IN_VALID_1
// 	case 2:
// 		mask = regs.O_FIFO_IN_VALID_2
// 	case 3:
// 		mask = regs.O_FIFO_IN_VALID_3
// 	default:
// 		panic(fmt.Errorf("eda: invalid rfm=%d", rfm))
// 	}
//
// 	return (dev.regs.pio.state.r() & mask) == mask
// }
//
// func (dev *Device) daqFIFOFull(rfm int) bool {
// 	reg := dev.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_STATUS_REG)
// 	return bit32(reg, 0) != 0
// }

func (dev *Device) daqFIFOEmpty(rfm int) bool {
	reg := dev.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_STATUS_REG)
	return bit32(reg, 1) == 0
}

// func (dev *Device) daqFIFOFillLevel(rfm int) uint32 {
// 	return dev.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_LEVEL_REG)
// }
//
// func (dev *Device) daqFIFOData(rfm int) uint32 {
// 	return dev.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_DATA_REG)
// }
//
// func (dev *Device) daqFIFOClear(rfm int) uint32 {
// 	var cnt uint32
// 	for !dev.daqFIFOEmpty(rfm) {
// 		// consume FIFO, one word at a time.
// 		_ = dev.regs.fifo.daq[rfm].r() // FIFO_DATA_REG==0
// 		cnt++
// 	}
// 	return cnt / 5
// }
//
// // daqWriteHRData writes the hardroc data for rfm to the provided writer
// // and returns the number of events read.
// //
// // daqWriteHRData writes data following this simple binary format:
// //
// //   6 x 32-bit words per event
// //        1: dif_id;dif_trigger_counter
// //        2: hr_id;bcid
// //        3-6 : discri data
// //
// // Note: on DAQ architecture: each RFM & chamber is identified by a DIF id
// // and EDA produces a data stream for each RFM.
// // DIF id's for new electronics are under 100 (above 100 is for old DIFs)
// func (dev *Device) daqWriteHRData(w io.Writer, rfm int) uint32 {
// 	var (
// 		n     uint32
// 		buf   = bufio.NewWriter(w)
// 		difID = difIDOffset + ((dev.id & 7) << 3) + (uint32(rfm) & 3)
// 	)
// 	for !dev.daqFIFOEmpty(rfm) {
// 		v := (difID << 24) | (dev.daq.cycleID[rfm] & 0x00ffffff)
// 		binary.BigEndian.PutUint32(dev.buf[:4], v)
// 		_, _ = buf.Write(dev.buf[:4])
// 		for i := 0; i < 5; i++ {
// 			v = dev.regs.fifo.daq[rfm].r()
// 			binary.BigEndian.PutUint32(dev.buf[:4], v)
// 			_, _ = buf.Write(dev.buf[:4])
// 			n++
// 		}
// 	}
// 	dev.daq.cycleID[rfm]++
// 	_ = buf.Flush()
// 	return n / 5
// }

// func (dev *Device) daqSaveHRDataAsDIF(w io.Writer, rfm int) uint32 {
// 	var (
// 		n   uint32
// 		buf = bufio.NewWriter(w)
// 		wU8 = func(v uint8) {
// 			dev.buf[0] = v
// 			_, _ = buf.Write(dev.buf[:1])
// 		}
// 		wU16 = func(v uint16) {
// 			binary.BigEndian.PutUint16(dev.buf[:2], v)
// 			_, _ = buf.Write(dev.buf[:2])
// 		}
// 		wU32 = func(v uint32) {
// 			binary.BigEndian.PutUint32(dev.buf[:4], v)
// 			_, _ = buf.Write(dev.buf[:4])
// 		}
// 	)
// 	defer func() {
// 		_ = buf.Flush()
// 	}()
//
// 	// DIF DAQ header,
// 	wU8(0xB0)
// 	wU8(difIDOffset + byte(dev.id&7)<<3 + byte(rfm)&3)
// 	// counters
// 	wU32(dev.daq.cycleID[rfm])
// 	wU32(dev.cntHit0(rfm))
// 	wU32(dev.cntHit1(rfm))
// 	wU16(uint16(dev.cntBCID48MSB() & 0xffff))
// 	wU32(dev.cntBCID48LSB())
// 	bcid24 := dev.cntBCID24()
// 	wU8(uint8(bcid24 >> 16))
// 	wU16(uint16(bcid24 & 0xffff))
// 	// unused "nb-lines"
// 	wU8(0xff)
//
// 	// HR DAQ chunk
// 	var (
// 		lastHR = -1
// 		hrID   int
// 	)
// 	for !dev.daqFIFOEmpty(rfm) {
// 		// read HR ID
// 		id := dev.regs.fifo.daq[rfm].r()
// 		hrID = int(id >> 24)
// 		// insert trailer and header if new hardroc ID
// 		if hrID != lastHR {
// 			if lastHR >= 0 {
// 				wU8(0xA3) // HR trailer
// 			}
// 			wU8(0xB4) // HR header
// 		}
// 		wU32(id)
// 		for i := 0; i < 4; i++ {
// 			wU32(dev.regs.fifo.daq[rfm].r())
// 			n++
// 		}
// 		lastHR = hrID
// 	}
// 	wU8(0xA3)    // last HR trailer
// 	wU8(0xA0)    // DIF DAQ trailer
// 	wU16(0xC0C0) // fake CRC
//
// 	// on-line monitoring
// 	nRAMUnits := n / 5
//
// 	dev.daq.cycleID[rfm]++
// 	return nRAMUnits
// }

func difIDFrom(id uint32, rfm int) byte {
	return difIDOffset + byte(id&7)<<3 + byte(rfm)&3
}

func (dev *Device) daqWriteDIFData(w io.Writer, rfm int) {
	var (
		wU8 = func(v uint8) {
			dev.buf[0] = v
			_, _ = w.Write(dev.buf[:1])
		}
		wU16 = func(v uint16) {
			binary.BigEndian.PutUint16(dev.buf[:2], v)
			_, _ = w.Write(dev.buf[:2])
		}
		wU32 = func(v uint32) {
			binary.BigEndian.PutUint32(dev.buf[:4], v)
			_, _ = w.Write(dev.buf[:4])
		}
	)

	// offset
	if dev.daq.cycleID[rfm] == 0 {
		dev.daq.bcid48Offset = dev.cntBCID48LSB() - dev.cntBCID24()
	}

	// DIF DAQ header
	wU8(0xB0)
	wU8(difIDFrom(dev.id, rfm))
	// counters
	wU32(dev.daq.cycleID[rfm])
	wU32(dev.cntHit0(rfm))
	wU32(dev.cntHit1(rfm))
	// assemble and correct absolute BCID
	bcid48 := uint64(dev.cntBCID48MSB())
	bcid48 <<= 32
	bcid48 |= uint64(dev.cntBCID48LSB())
	bcid48 -= uint64(dev.daq.bcid48Offset)
	// copy frame
	wU16(uint16(bcid48>>32) & 0xffff)
	wU32(uint32(bcid48))
	bcid24 := dev.cntBCID24()
	wU8(uint8(bcid24 >> 16))
	wU16(uint16(bcid24 & 0xffff))
	// unused "nb-lines"
	wU8(0xff)

	// HR DAQ chunk
	var (
		lastHR = -1
		hrID   int
	)
	wU8(0xB4) // HR header
	for !dev.daqFIFOEmpty(rfm) {
		// read HR ID
		id := dev.regs.fifo.daq[rfm].r()
		hrID = int(id >> 24)
		// insert trailer and header if new hardroc ID
		if hrID != lastHR {
			if lastHR >= 0 {
				wU8(0xA3) // HR trailer
				wU8(0xB4) // HR header
			}
		}
		wU32(id)
		for i := 0; i < 4; i++ {
			wU32(dev.regs.fifo.daq[rfm].r())
		}
		lastHR = hrID
	}
	wU8(0xA3)    // last HR trailer
	wU8(0xA0)    // DIF DAQ trailer
	wU16(0xC0C0) // fake CRC

	dev.daq.cycleID[rfm]++
}

func (dev *Device) daqSendDIFData(sck net.Conn, buf []byte) error {
	defer func() {
		dev.daq.w.c = 0
	}()

	errorf := func(format string, args ...interface{}) error {
		err := fmt.Errorf(format, args...)
		dev.msg.Printf("%+v", err)
		return err
	}

	hdr := buf[:8]
	cur := dev.daq.w.c
	copy(hdr, "HDR\x00")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(cur))

	_, err := sck.Write(hdr)
	if err != nil {
		return errorf(
			"eda: could not send DIF data size header to %v: %w",
			sck.RemoteAddr(), err,
		)
	}

	if cur > 0 {
		_, err = sck.Write(dev.daq.w.p[:cur])
		if err != nil {
			return errorf(
				"eda: could not send DIF data to %v: %w",
				sck.RemoteAddr(), err,
			)
		}
		_, _ = dev.daq.f.Write(dev.daq.w.p[:cur])
		dec := eformat.NewDecoder(difIDFrom(dev.id, 0), bytes.NewReader(dev.daq.w.p[:cur]))
		dec.IsEDA = true
		var d eformat.DIF
		err = dec.Decode(&d)
		if err != nil {
			dev.msg.Printf("could not decode DIF: %+v", err)
		} else {
			wbuf := dev.msg.Writer()
			fmt.Fprintf(wbuf, "=== DIF-ID 0x%x ===\n", d.Header.ID)
			fmt.Fprintf(wbuf, "DIF trigger: % 10d\n", d.Header.DTC)
			fmt.Fprintf(wbuf, "ACQ trigger: % 10d\n", d.Header.ATC)
			fmt.Fprintf(wbuf, "Gbl trigger: % 10d\n", d.Header.GTC)
			fmt.Fprintf(wbuf, "Abs BCID:    % 10d\n", d.Header.AbsBCID)
			fmt.Fprintf(wbuf, "Time DIF:    % 10d\n", d.Header.TimeDIFTC)
			fmt.Fprintf(wbuf, "Frames:      % 10d\n", len(d.Frames))

			for _, frame := range d.Frames {
				fmt.Fprintf(wbuf, "  hroc=0x%02x BCID=% 8d %x\n",
					frame.Header, frame.BCID, frame.Data,
				)
			}
		}
	}

	// wait for ACK
	_, err = io.ReadFull(sck, hdr[:4])
	if err != nil {
		return errorf(
			"eda: could not read ACK DIF data from %v: %+v",
			sck.RemoteAddr(), err,
		)
	}
	if string(hdr[:4]) != "ACK\x00" {
		return errorf(
			"eda: invalid ACK DIF data from %v: %q",
			sck.RemoteAddr(), hdr[:4],
		)
	}

	return nil
}
