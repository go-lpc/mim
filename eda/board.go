// Copyright 2021 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-lpc/mim/eda/internal/regs"
)

type board struct {
	mem  [szCfgHR]byte
	sli  []byte
	regs pins

	msg  *log.Logger
	err  error
	xbuf [4]byte
}

type pins struct {
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

func newBoard(msg *log.Logger) board {
	var b board
	b.msg = msg
	b.sli = b.mem[4:]
	return b
}

func (brd *board) readU32(r io.ReaderAt, off int64) uint32 {
	if brd.err != nil {
		return 0
	}
	_, brd.err = r.ReadAt(brd.xbuf[:4], off)
	if brd.err != nil {
		brd.err = fmt.Errorf("eda: could not read register 0x%x: %w", off, brd.err)
		return 0
	}
	return binary.LittleEndian.Uint32(brd.xbuf[:4])
}

func (brd *board) writeU32(w io.WriterAt, off int64, v uint32) {
	if brd.err != nil {
		return
	}
	binary.LittleEndian.PutUint32(brd.xbuf[:4], v)
	_, brd.err = w.WriteAt(brd.xbuf[:4], off)
	if brd.err != nil {
		brd.err = fmt.Errorf("eda: could not write register 0x%x: %w", off, brd.err)
		return
	}
}

func (brd *board) bindH2F(h2f rwer) error {
	brd.regs.fifo.daq[0] = newReg32(brd, h2f, regs.H2F_FIFO_DAQ_RFM0)
	brd.regs.fifo.daq[1] = newReg32(brd, h2f, regs.H2F_FIFO_DAQ_RFM1)
	brd.regs.fifo.daq[2] = newReg32(brd, h2f, regs.H2F_FIFO_DAQ_RFM2)
	brd.regs.fifo.daq[3] = newReg32(brd, h2f, regs.H2F_FIFO_DAQ_RFM3)

	brd.regs.fifo.daqCSR[0] = newDAQFIFO(brd, h2f, regs.H2F_FIFO_DAQ_CSR_RFM0)
	brd.regs.fifo.daqCSR[1] = newDAQFIFO(brd, h2f, regs.H2F_FIFO_DAQ_CSR_RFM1)
	brd.regs.fifo.daqCSR[2] = newDAQFIFO(brd, h2f, regs.H2F_FIFO_DAQ_CSR_RFM2)
	brd.regs.fifo.daqCSR[3] = newDAQFIFO(brd, h2f, regs.H2F_FIFO_DAQ_CSR_RFM3)
	return nil
}

func (brd *board) bindLwH2F(lw rwer) error {
	brd.regs.pio.state = newReg32(brd, lw, regs.LW_H2F_PIO_STATE_IN)
	brd.regs.pio.ctrl = newReg32(brd, lw, regs.LW_H2F_PIO_CTRL_OUT)
	brd.regs.pio.pulser = newReg32(brd, lw, regs.LW_H2F_PIO_PULSER)

	brd.regs.ramSC[0] = newHRCfg(lw, regs.LW_H2F_RAM_SC_RFM0)
	brd.regs.ramSC[1] = newHRCfg(lw, regs.LW_H2F_RAM_SC_RFM1)
	brd.regs.ramSC[2] = newHRCfg(lw, regs.LW_H2F_RAM_SC_RFM2)
	brd.regs.ramSC[3] = newHRCfg(lw, regs.LW_H2F_RAM_SC_RFM3)

	brd.regs.pio.chkSC[0] = newReg32(brd, lw, regs.LW_H2F_PIO_SC_CHECK_RFM0)
	brd.regs.pio.chkSC[1] = newReg32(brd, lw, regs.LW_H2F_PIO_SC_CHECK_RFM1)
	brd.regs.pio.chkSC[2] = newReg32(brd, lw, regs.LW_H2F_PIO_SC_CHECK_RFM2)
	brd.regs.pio.chkSC[3] = newReg32(brd, lw, regs.LW_H2F_PIO_SC_CHECK_RFM3)

	brd.regs.pio.cntHit0[0] = newReg32(brd, lw, regs.LW_H2F_PIO_CNT_HIT0_RFM0)
	brd.regs.pio.cntHit0[1] = newReg32(brd, lw, regs.LW_H2F_PIO_CNT_HIT0_RFM1)
	brd.regs.pio.cntHit0[2] = newReg32(brd, lw, regs.LW_H2F_PIO_CNT_HIT0_RFM2)
	brd.regs.pio.cntHit0[3] = newReg32(brd, lw, regs.LW_H2F_PIO_CNT_HIT0_RFM3)

	brd.regs.pio.cntHit1[0] = newReg32(brd, lw, regs.LW_H2F_PIO_CNT_HIT1_RFM0)
	brd.regs.pio.cntHit1[1] = newReg32(brd, lw, regs.LW_H2F_PIO_CNT_HIT1_RFM1)
	brd.regs.pio.cntHit1[2] = newReg32(brd, lw, regs.LW_H2F_PIO_CNT_HIT1_RFM2)
	brd.regs.pio.cntHit1[3] = newReg32(brd, lw, regs.LW_H2F_PIO_CNT_HIT1_RFM3)

	brd.regs.pio.cntTrig = newReg32(brd, lw, regs.LW_H2F_PIO_CNT_TRIG)
	brd.regs.pio.cnt48MSB = newReg32(brd, lw, regs.LW_H2F_PIO_CNT48_MSB)
	brd.regs.pio.cnt48LSB = newReg32(brd, lw, regs.LW_H2F_PIO_CNT48_LSB)
	brd.regs.pio.cnt24 = newReg32(brd, lw, regs.LW_H2F_PIO_CNT24)

	return nil
}

func (brd *board) rfmOn(rfm int) error {
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
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= mask
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not switch ON RFM=%d: %w", rfm, brd.err)
	}
	return nil
}

// func (brd *board) rfmOff(rfm int) error {
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
// 	ctrl := brd.regs.pio.ctrl.r()
// 	ctrl &= ^mask
// 	brd.regs.pio.ctrl.w(ctrl)
//
// 	if brd.err != nil {
// 		return fmt.Errorf("eda: could not switch OFF RFM=%d: %w", rfm, brd.err)
// 	}
// 	return nil
// }

func (brd *board) rfmEnable(rfm int) error {
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
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= mask
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not enable RFM=%d: %w", rfm, brd.err)
	}
	return nil
}

// func (brd *board) rfmDisable(rfm int) error {
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
// 	ctrl := brd.regs.pio.ctrl.r()
// 	ctrl &= ^mask
// 	brd.regs.pio.ctrl.w(ctrl)
//
// 	if brd.err != nil {
// 		return fmt.Errorf("eda: could not disable RFM=%d: %w", rfm, brd.err)
// 	}
// 	return nil
// }

func (brd *board) syncResetFPGA() error {
	brd.regs.pio.ctrl.w(regs.O_RESET)
	brd.msg.Printf("reset FPGA")
	time.Sleep(1 * time.Microsecond)
	brd.regs.pio.ctrl.w(0x00000000)
	time.Sleep(1 * time.Microsecond)

	if brd.err != nil {
		return fmt.Errorf("eda: could not reset FPGA: %w", brd.err)
	}
	return nil
}

func (brd *board) syncResetHR() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= regs.O_RESET_HR
	brd.regs.pio.ctrl.w(ctrl)
	time.Sleep(1 * time.Microsecond)

	ctrl = brd.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_RESET_HR)
	brd.regs.pio.ctrl.w(ctrl)
	time.Sleep(1 * time.Microsecond)

	if brd.err != nil {
		return fmt.Errorf("eda: could not reset HR: %w", brd.err)
	}
	return nil
}

func (brd *board) syncPLLLock() bool {
	state := brd.regs.pio.state.r()
	return state&regs.O_PLL_LCK == regs.O_PLL_LCK
}

func (brd *board) syncState() uint32 {
	state := brd.regs.pio.state.r()
	return (state >> regs.SHIFT_SYNCHRO_STATE) & 0xF
}

func (brd *board) syncSelectCmdSoft() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= regs.O_SEL_CMD_SOURCE
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not synchronize cmd-soft: %w", brd.err)
	}
	return nil
}

func (brd *board) syncSelectCmdDCC() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_SEL_CMD_SOURCE)
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not synchronize cmd-dcc: %w", brd.err)
	}
	return nil
}

func (brd *board) syncSetCmd(cmd uint32) error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl &= ^uint32(0xf << regs.SHIFT_CMD_CODE) // reset 4 bits
	ctrl |= (0xf & cmd) << regs.SHIFT_CMD_CODE  // set command

	brd.regs.pio.ctrl.w(ctrl)
	time.Sleep(2 * time.Microsecond)

	ctrl &= ^uint32(0xf << regs.SHIFT_CMD_CODE)  // reset 4 bits
	ctrl |= regs.CMD_IDLE << regs.SHIFT_CMD_CODE // set idle command
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not set command 0x%x: %w", cmd, brd.err)
	}
	return nil
}

func (brd *board) syncResetBCID() error {
	return brd.syncSetCmd(regs.CMD_RESET_BCID)
}

func (brd *board) syncStart() error {
	return brd.syncSetCmd(regs.CMD_START_ACQ)
}

func (brd *board) syncStop() error {
	return brd.syncSetCmd(regs.CMD_STOP_ACQ)
}

func (brd *board) syncRAMFullExt() error {
	return brd.syncSetCmd(regs.CMD_RAMFULL_EXT)
}

// func (brd *board) syncDigitalRO() error {
// 	return brd.syncSetCmd(regs.CMD_DIGITAL_RO)
// }

func (brd *board) syncArmFIFO() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= regs.O_HPS_BUSY
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not arm FIFO: %w", brd.err)
	}
	return nil
}

func (brd *board) syncAckFIFO() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_HPS_BUSY)
	brd.regs.pio.ctrl.w(ctrl) // falling edge on hps busy
	for brd.syncState() != regs.S_IDLE {
		// when FPGA ready for new acquisition
	}
	ctrl = brd.regs.pio.ctrl.r()
	brd.regs.pio.ctrl.w(ctrl | regs.O_HPS_BUSY) // re-arming

	if brd.err != nil {
		return fmt.Errorf("eda: could not ACK FIFO: %w", brd.err)
	}
	return nil
}

func (brd *board) syncDCCCmdMem() uint32 {
	cnt := brd.regs.pio.cnt24.r()
	return (cnt >> regs.SHIFT_CMD_CODE_MEM) & 0xf
}

// func (brd *board) syncDCCCmdNow() uint32 {
// 	cnt := brd.regs.pio.cnt24.r()
// 	return (cnt >> regs.SHIFT_CMD_CODE_NOW) & 0xf
// }
//
// func (brd *board) syncRAMFull() bool {
// 	state := brd.syncState()
// 	return state == regs.S_RAMFULL
// }
//
// func (brd *board) syncFPGARO() bool {
// 	state := brd.syncState()
// 	return state == regs.S_START_RO || state == regs.S_WAIT_END_RO
// }
//
// func (brd *board) syncFIFOReady() bool {
// 	state := brd.syncState()
// 	return state == regs.S_FIFO_READY
// }
//
// func (brd *board) syncRunStopped() bool {
// 	state := brd.syncState()
// 	return state == regs.S_STOP_RUN
// }

// func (brd *board) syncHRTransmitOn(rfm int) bool {
// 	state := brd.regs.pio.state.r()
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
// func (brd *board) syncChipsAt(rfm int) bool {
// 	state := brd.regs.pio.state.r()
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
// func (brd *board) syncHREndRO(rfm int) bool {
// 	state := brd.regs.pio.state.r()
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

func (brd *board) syncEnableDCCBusy() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= regs.O_ENA_DCC_BUSY
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not enable DCC-BUSY: %w", brd.err)
	}
	return nil
}

func (brd *board) syncEnableDCCRAMFull() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= regs.O_ENA_DCC_RAMFULL
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not enable DCC-RAMFULL: %w", brd.err)
	}
	return nil
}

// func (brd *board) trigSelectThreshold0() error {
// 	ctrl := brd.regs.pio.ctrl.r()
// 	ctrl &= ^uint32(regs.O_SEL_TRIG_THRESH)
// 	brd.regs.pio.ctrl.w(ctrl)
//
// 	if brd.err != nil {
// 		return fmt.Errorf("eda: could not select threshold-0: %w", brd.err)
// 	}
// 	return nil
// }
//
// func (brd *board) trigSelectThreshold1() error {
// 	ctrl := brd.regs.pio.ctrl.r()
// 	ctrl |= regs.O_SEL_TRIG_THRESH
// 	brd.regs.pio.ctrl.w(ctrl)
//
// 	if brd.err != nil {
// 		return fmt.Errorf("eda: could not select threshold-1: %w", brd.err)
// 	}
// 	return nil
// }
//
// func (brd *board) trigEnable() error {
// 	ctrl := brd.regs.pio.ctrl.r()
// 	ctrl |= regs.O_ENA_TRIG
// 	brd.regs.pio.ctrl.w(ctrl)
//
// 	if brd.err != nil {
// 		return fmt.Errorf("eda: could not enable trigger: %w", brd.err)
// 	}
// 	return nil
// }
//
// func (brd *board) trigDisable() error {
// 	ctrl := brd.regs.pio.ctrl.r()
// 	ctrl &= ^uint32(regs.O_ENA_TRIG)
// 	brd.regs.pio.ctrl.w(ctrl)
//
// 	if brd.err != nil {
// 		return fmt.Errorf("eda: could not disable trigger: %w", brd.err)
// 	}
// 	return nil
// }

func (brd *board) cntReset() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= regs.O_RST_SCALERS
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not reset scalers: %w", brd.err)
	}

	time.Sleep(1 * time.Microsecond)

	ctrl &= ^uint32(regs.O_RST_SCALERS)
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not clear scalers: %w", brd.err)
	}
	return nil
}

func (brd *board) cntStart() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= regs.O_ENA_SCALERS
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not start scalers: %w", brd.err)
	}
	return nil
}

func (brd *board) cntStop() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_ENA_SCALERS)
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not stop scalers: %w", brd.err)
	}
	return nil
}

func (brd *board) cntHit0(rfm int) uint32 {
	return brd.regs.pio.cntHit0[rfm].r()
}

func (brd *board) cntHit1(rfm int) uint32 {
	return brd.regs.pio.cntHit1[rfm].r()
}

func (brd *board) cntTrig() uint32 {
	return brd.regs.pio.cntTrig.r()
}

func (brd *board) cntBCID24() uint32 {
	v := brd.regs.pio.cnt24.r()
	return v & 0xffffff
}

func (brd *board) cntBCID48MSB() uint32 {
	v := brd.regs.pio.cnt48MSB.r()
	return v & 0xffff
}

func (brd *board) cntBCID48LSB() uint32 {
	return brd.regs.pio.cnt48LSB.r()
}

// func (brd *board) cntSave(w io.Writer, rfm int) error {
// 	var (
// 		i   = 0
// 		buf = make([]byte, 7*4)
// 		dif = difIDOffset + ((brd.id & 7) << 3) + (uint32(rfm) & 3)
// 		u32 = (dif << 24) | (brd.daq.cycleID[rfm] & 0x00ffffff)
// 	)
//
// 	binary.BigEndian.PutUint32(buf[i:], u32)
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], brd.cntBCID48MSB())
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], brd.cntBCID48LSB())
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], brd.cntBCID24())
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], brd.cntHit0(rfm))
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], brd.cntHit1(rfm))
// 	i += 4
// 	binary.BigEndian.PutUint32(buf[i:], brd.cntTrig())
// 	if brd.err != nil {
// 		return fmt.Errorf("eda: could not read counters: %w", brd.err)
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

func (brd *board) hrscSelectSlowControl() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_SELECT_SC_RR)
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not select slow-control: %w", brd.err)
	}
	return nil
}

func (brd *board) hrscSelectReadRegister() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= regs.O_SELECT_SC_RR
	brd.regs.pio.ctrl.w(ctrl)

	if brd.err != nil {
		return fmt.Errorf("eda: could not select read-register: %w", brd.err)
	}
	return nil
}

func div(num, den uint32) (quo, rem uint32) {
	quo = num / den
	rem = num % den
	return quo, rem
}

func (brd *board) hrscSetBit(hr, addr, bit uint32) {
	// byte address 0 corresponds to the last register (addr 864 to 871)
	// of the last Hardroc (pos=nHR-1)
	var (
		quo, rem = div(addr, nHR)

		i   = (nHR-1-hr)*nBytesCfgHR + nBytesCfgHR - 1 - quo
		v   = brd.sli[i]
		off = rem

		// bit address increases from LSB to MSB
		mask1 = uint8(0x01 << off)
		mask2 = uint8((0x1 & bit) << off)
	)
	v &= ^mask1 // reset target bit
	v |= mask2  // set target bit = "bit" argument
	brd.sli[i] = v
}

func (brd *board) hrscGetBit(hr, addr uint32) uint32 {
	// byte address 0 corresponds to the last register (addr 864 to 871)
	// of the last Hardroc (pos=nHR-1)
	var (
		quo, rem = div(addr, nHR)

		i   = (nHR-1-hr)*nBytesCfgHR + nBytesCfgHR - 1 - quo
		v   = brd.sli[i]
		off = rem
	)
	return uint32((v >> off) & 0x01)
}

func (brd *board) hrscSetWord(hr, addr, nbits, v uint32) {
	for i := uint32(0); i < nbits; i++ {
		// scan LSB to MSB
		bit := (v >> i) & 0x01
		brd.hrscSetBit(hr, addr+i, bit)
	}
}

func (brd *board) hrscSetWordMSB2LSB(hr, addr, nbits, v uint32) {
	for i := uint32(0); i < nbits; i++ {
		// scan MSB to LSB
		bit := (v >> i) & 0x01
		brd.hrscSetBit(hr, addr+nbits-1-i, bit)
	}
}

func (brd *board) hrscReadConf(fname string, hr uint32) error {
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

		brd.hrscSetBit(hr, addr, bit)
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

func (brd *board) hrscCopyConf(hrDst, hrSrc uint32) {
	var (
		isrc = (nHR - 1 - hrSrc) * nBytesCfgHR
		idst = (nHR - 1 - hrDst) * nBytesCfgHR

		dst = brd.sli[idst : idst+nBytesCfgHR]
		src = brd.sli[isrc : isrc+nBytesCfgHR]
	)
	copy(dst, src)
}

func (brd *board) hrscReadConfHRs(fname string) error {
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

		brd.hrscSetBit(hr, addr, bit)
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

func (brd *board) hrscWriteConfHRs(fname string) error {
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
				v    = brd.hrscGetBit(hr, addr)
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
// func (brd *board) hrscSetCtest(hr, ch, v uint32) {
// 	brd.hrscSetBit(hr, ch, v&0x01)
// }
//
// func (brd *board) hrscSetAllCtestOff() {
// 	for hr := uint32(0); hr < nHR; hr++ {
// 		for ch := uint32(0); ch < nChans; ch++ {
// 			brd.hrscSetCtest(hr, ch, 0)
// 		}
// 	}
// }

func (brd *board) hrscSetPreAmp(hr, ch, v uint32) {
	addr := nChans + nHR*ch
	brd.hrscSetWord(hr, addr, 8, v)
}

// func (brd *board) hrscSetCmdFSB2(hr, ch, v uint32) {
// 	// fast shaper 2 gain
// 	addr := 587 + 4*ch
// 	brd.hrscSetWordMSB2LSB(hr, addr, 4, ^v) // "cmdb" register bits are active-low
// }
//
// func (brd *board) hrscSetCmdFSB1(hr, ch, v uint32) {
// 	// fast shaper 2 gain
// 	addr := 595 + 4*ch
// 	brd.hrscSetWordMSB2LSB(hr, addr, 4, ^v) // "cmdb" register bits are active-low
// }

func (brd *board) hrscSetMask(hr, ch, v uint32) {
	addr := 618 + 3*ch
	brd.hrscSetWord(hr, addr, 3, v)
}

func (brd *board) hrscSetChipID(hr, v uint32) {
	brd.hrscSetWordMSB2LSB(hr, 810, 8, v)
}

func (brd *board) hrscSetDAC0(hr, v uint32) {
	brd.hrscSetWord(hr, 818, 10, v)
}

func (brd *board) hrscSetDAC1(hr, v uint32) {
	brd.hrscSetWord(hr, 828, 10, v)
}

func (brd *board) hrscSetDAC2(hr, v uint32) {
	brd.hrscSetWord(hr, 838, 10, v)
}

// func (brd *board) hrscSetDACCoarse(hr uint32) {
// 	brd.hrscSetWord(hr, 848, 10, 0)
// }
//
// func (brd *board) hrscSetDACFine(hr uint32) {
// 	brd.hrscSetWord(hr, 848, 10, 1)
// }

func (brd *board) hrscSetCShaper(hr, v uint32) {
	brd.hrscSetBit(hr, 611, v&1)      // sw_50f0 = b0
	brd.hrscSetBit(hr, 602, v&1)      // sw_50f1 = b0
	brd.hrscSetBit(hr, 594, v&1)      // sw_50f2 = b0
	brd.hrscSetBit(hr, 610, (v>>1)&1) // sw_100f0 = b1
	brd.hrscSetBit(hr, 601, (v>>1)&1) // sw_100f1 = b1
	brd.hrscSetBit(hr, 593, (v>>1)&1) // sw_100f2 = b1
}

func (brd *board) hrscSetRShaper(hr, v uint32) {
	brd.hrscSetBit(hr, 609, v&1)      // sw_100k0 = b0
	brd.hrscSetBit(hr, 600, v&1)      // sw_100k1 = b0
	brd.hrscSetBit(hr, 592, v&1)      // sw_100k2 = b0
	brd.hrscSetBit(hr, 608, (v>>1)&1) // sw_50k0 = b1
	brd.hrscSetBit(hr, 599, (v>>1)&1) // sw_50k1 = b1
	brd.hrscSetBit(hr, 591, (v>>1)&1) // sw_50k2 = b1
}

func (brd *board) hrscSetConfig(rfm int) error {
	ctrl := brd.regs.pio.chkSC[rfm].r()
	if brd.err != nil {
		return fmt.Errorf(
			"eda: could not read check-sc register (rfm=%d): %w",
			rfm, brd.err,
		)
	}

	switch ctrl {
	case 0xcafefade:
		ctrl = 0x36baffe5
	default:
		ctrl = 0xcafefade
	}
	brd.mem[0] = byte((ctrl >> 24) & 0xff)
	brd.mem[1] = byte((ctrl >> 16) & 0xff)
	brd.mem[2] = byte((ctrl >> 8) & 0xff)
	brd.mem[3] = byte(ctrl & 0xff)

	// reset sc
	err := brd.hrscSelectSlowControl()
	if err != nil {
		return fmt.Errorf("eda: could not select slow-control (rfm=%d): %w",
			rfm, err,
		)
	}

	err = brd.hrscResetSC()
	if err != nil {
		return fmt.Errorf("eda: could not reset slow-control (rfm=%d): %w",
			rfm, err,
		)
	}

	if brd.hrscSCDone(rfm) {
		return fmt.Errorf("eda: could not reset slow control (rfm=%d): sc-not-done", rfm)
	}

	// copy to FPGA
	_, err = brd.regs.ramSC[rfm].w(brd.mem[:szCfgHR])
	if err != nil {
		return fmt.Errorf(
			"eda: could not write slow-control cfg to FPGA (rfm=%d): %w",
			rfm, err,
		)
	}

	// trigger the slow control serializer
	err = brd.hrscStartSC(rfm)
	if err != nil {
		return fmt.Errorf(
			"eda: could not start slow-control serializer (rfm=%d): %w",
			rfm, err,
		)
	}

	// check loop-back header
	time.Sleep(10 * time.Microsecond)
	for !brd.hrscSCDone(rfm) {
		time.Sleep(10 * time.Microsecond)
	}

	chk := brd.regs.pio.chkSC[rfm].r()
	if brd.err != nil {
		return fmt.Errorf(
			"eda: could not read slow-control loopback register (rfm=%d): %w",
			rfm, brd.err,
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

func (brd *board) hrscResetReadRegisters(rfm int) error {
	ctrl := brd.regs.pio.chkSC[rfm].r()
	if brd.err != nil {
		return fmt.Errorf(
			"eda: could not read check-sc register (rfm=%d): %w",
			rfm, brd.err,
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
	err := brd.hrscSelectReadRegister()
	if err != nil {
		return fmt.Errorf(
			"eda: could select read-register (rfm=%d): %w",
			rfm, err,
		)
	}

	err = brd.hrscResetSC()
	if err != nil {
		return fmt.Errorf("eda: could not reset slow-control (rfm=%d): %w",
			rfm, err,
		)
	}

	if brd.hrscSCDone(rfm) {
		return fmt.Errorf("eda: could not reset slow control (rfm=%d)", rfm)
	}

	// copy to FPGA
	_, err = brd.regs.ramSC[rfm].w(buf[:szCfgHR])
	if err != nil {
		return fmt.Errorf(
			"eda: could not write slow-control cfg to FPGA (rfm=%d): %w",
			rfm, err,
		)
	}

	// trigger the slow control serializer
	time.Sleep(10 * time.Microsecond)
	err = brd.hrscStartSC(rfm)
	if err != nil {
		return fmt.Errorf(
			"eda: could not start slow-control serializer (rfm=%d): %w",
			rfm, err,
		)
	}

	// check loop-back header
	time.Sleep(10 * time.Microsecond)
	for !brd.hrscSCDone(rfm) {
		time.Sleep(10 * time.Microsecond)
	}

	chk := brd.regs.pio.chkSC[rfm].r()
	if brd.err != nil {
		return fmt.Errorf(
			"eda: could not read slow-control loopback register (rfm=%d): %w",
			rfm, brd.err,
		)
	}

	if chk != ctrl {
		return fmt.Errorf(
			"eda: invalid loopback register (rfm=%d): got=0x%x, want=0x%x",
			rfm, chk, ctrl,
		)
	}

	err = brd.hrscSelectSlowControl()
	if err != nil {
		return fmt.Errorf(
			"eda: could not select slow-control (rfm=%d): %w",
			rfm, err,
		)
	}

	return nil
}

// func (brd *board) hrscSetReadRegister(rfm, ch int) error {
// 	ctrl := brd.regs.pio.chkSC[rfm].r()
// 	if brd.err != nil {
// 		return fmt.Errorf(
// 			"eda: could not read check-sc register (rfm=%d): %w",
// 			rfm, brd.err,
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
// 	err := brd.hrscSelectReadRegister()
// 	if err != nil {
// 		return fmt.Errorf(
// 			"eda: could select read-register (rfm=%d): %w",
// 			rfm, err,
// 		)
// 	}
//
// 	err = brd.hrscResetSC()
// 	if err != nil {
// 		return fmt.Errorf("eda: could not reset slow-control (rfm=%d): %w",
// 			rfm, err,
// 		)
// 	}
//
// 	if !brd.hrscSCDone(rfm) {
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
// 	_, err = brd.regs.ramSC[rfm].w(buf[:szCfgHR])
// 	if err != nil {
// 		return fmt.Errorf(
// 			"eda: could not write slow-control cfg to FPGA (rfm=%d): %w",
// 			rfm, err,
// 		)
// 	}
//
// 	// trigger the slow control serializer
// 	err = brd.hrscStartSC(rfm)
// 	if err != nil {
// 		return fmt.Errorf(
// 			"eda: could not start slow-control serializer (rfm=%d): %w",
// 			rfm, err,
// 		)
// 	}
//
// 	// check loop-back header
// 	time.Sleep(10 * time.Microsecond)
// 	for !brd.hrscSCDone(rfm) {
// 		time.Sleep(10 * time.Microsecond)
// 	}
//
// 	chk := brd.regs.pio.chkSC[rfm].r()
// 	if brd.err != nil {
// 		return fmt.Errorf(
// 			"eda: could not read slow-control loopback register (rfm=%d): %w",
// 			rfm, brd.err,
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
// 	err = brd.hrscSelectSlowControl()
// 	if err != nil {
// 		return fmt.Errorf(
// 			"eda: could not select slow-control (rfm=%d): %w",
// 			rfm, err,
// 		)
// 	}
//
// 	return nil
// }

func (brd *board) hrscResetSC() error {
	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= regs.O_RESET_SC
	brd.regs.pio.ctrl.w(ctrl)
	if brd.err != nil {
		return fmt.Errorf("eda: could not reset slow-control: %w", brd.err)
	}

	time.Sleep(1 * time.Microsecond)

	ctrl = brd.regs.pio.ctrl.r()
	ctrl &= ^uint32(regs.O_RESET_SC)
	brd.regs.pio.ctrl.w(ctrl)
	if brd.err != nil {
		return fmt.Errorf("eda: could not reset slow-control: %w", brd.err)
	}

	time.Sleep(1 * time.Microsecond)

	return nil
}

func (brd *board) hrscStartSC(rfm int) error {
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

	ctrl := brd.regs.pio.ctrl.r()
	ctrl |= mask
	brd.regs.pio.ctrl.w(ctrl)
	if brd.err != nil {
		return fmt.Errorf(
			"eda: could read/write pio ctrl mask=0x%x: %w",
			mask,
			brd.err,
		)
	}

	time.Sleep(1 * time.Microsecond)

	ctrl = brd.regs.pio.ctrl.r()
	ctrl &= ^mask
	brd.regs.pio.ctrl.w(ctrl)
	if brd.err != nil {
		return fmt.Errorf(
			"eda: could read/write pio ctrl ^mask=0x%x: %w",
			mask,
			brd.err,
		)
	}

	return nil
}

func (brd *board) hrscSCDone(rfm int) bool {
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

	return (brd.regs.pio.state.r() & mask) == mask
}

func bit32(word, digit uint32) uint32 {
	return (word >> digit) & 0x1
}

func bitU64(v uint64, pos uint32) uint8 {
	o := v & uint64(1<<pos)
	if o == 0 {
		return 0
	}
	return 1
}

func (brd *board) daqFIFOInit(rfm int) error {
	fifo := &brd.regs.fifo.daqCSR[rfm]

	// clear event reg (write 1 to each field)
	fifo.w(regs.ALTERA_AVALON_FIFO_EVENT_REG, regs.ALTERA_AVALON_FIFO_EVENT_ALL)

	// disable interrupts
	fifo.w(regs.ALTERA_AVALON_FIFO_IENABLE_REG, 0)

	// set "almostfull" to maxsize+1
	fifo.w(regs.ALTERA_AVALON_FIFO_ALMOSTFULL_REG, 5080+1)

	// set "almostempty"
	fifo.w(regs.ALTERA_AVALON_FIFO_ALMOSTEMPTY_REG, 2)

	if brd.err != nil {
		return fmt.Errorf("eda: could not initialize DAQ FIFO: %w", brd.err)
	}
	return nil
}

// func (brd *board) daqFIFOPrintStatus(rfm int) {
// 	reg := brd.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_STATUS_REG)
// 	switch brd.err {
// 	case nil:
// 		brd.msg.Printf("fifo/status-reg[rfm=%d]= 0x%x\n", rfm, reg)
// 	default:
// 		brd.msg.Printf("fifo/status-reg[rfm=%d]= %+v\n", rfm, brd.err)
// 	}
// }
//
// func (brd *board) daqFIFOPrintEvent(rfm int) {
// 	reg := brd.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_EVENT_REG)
// 	switch brd.err {
// 	case nil:
// 		brd.msg.Printf("fifo/event-reg[rfm=%d]= 0x%x\n", rfm, reg)
// 	default:
// 		brd.msg.Printf("fifo/event-reg[rfm=%d]= %+v\n", rfm, brd.err)
// 	}
// }
//
// func (brd *board) daqFIFOClearEvent(rfm int) error {
// 	// clear event register: write 1 to each field.
// 	brd.regs.fifo.daqCSR[rfm].w(
// 		regs.ALTERA_AVALON_FIFO_EVENT_REG,
// 		regs.ALTERA_AVALON_FIFO_EVENT_ALL,
// 	)
// 	if brd.err != nil {
// 		return fmt.Errorf(
// 			"eda: could not clear DAQ FIFO event register (rfm=%d): %w",
// 			rfm, brd.err,
// 		)
// 	}
// 	return nil
// }
//
// func (brd *board) daqFIFOInValid(rfm int) bool {
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
// 	return (brd.regs.pio.state.r() & mask) == mask
// }
//
// func (brd *board) daqFIFOFull(rfm int) bool {
// 	reg := brd.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_STATUS_REG)
// 	return bit32(reg, 0) != 0
// }
//
// func (brd *board) daqFIFOEmpty(rfm int) bool {
// 	reg := brd.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_STATUS_REG)
// 	return bit32(reg, 1) != 0
// }

func (brd *board) daqFIFOFillLevel(rfm int) uint32 {
	return brd.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_LEVEL_REG)
}

// func (brd *board) daqFIFOData(rfm int) uint32 {
// 	return brd.regs.fifo.daqCSR[rfm].r(regs.ALTERA_AVALON_FIFO_DATA_REG)
// }
//
// func (brd *board) daqFIFOClear(rfm int) uint32 {
// 	var cnt uint32
// 	for !brd.daqFIFOEmpty(rfm) {
// 		// consume FIFO, one word at a time.
// 		_ = brd.regs.fifo.daq[rfm].r() // FIFO_DATA_REG==0
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
// func (brd *board) daqWriteHRData(w io.Writer, rfm int) uint32 {
// 	var (
// 		n     uint32
// 		buf   = bufio.NewWriter(w)
// 		difID = difIDOffset + ((brd.id & 7) << 3) + (uint32(rfm) & 3)
// 	)
// 	for !brd.daqFIFOEmpty(rfm) {
// 		v := (difID << 24) | (brd.daq.cycleID[rfm] & 0x00ffffff)
// 		binary.BigEndian.PutUint32(brd.buf[:4], v)
// 		_, _ = buf.Write(brd.buf[:4])
// 		for i := 0; i < 5; i++ {
// 			v = brd.regs.fifo.daq[rfm].r()
// 			binary.BigEndian.PutUint32(brd.buf[:4], v)
// 			_, _ = buf.Write(brd.buf[:4])
// 			n++
// 		}
// 	}
// 	brd.daq.cycleID[rfm]++
// 	_ = buf.Flush()
// 	return n / 5
// }

// func (brd *board) daqSaveHRDataAsDIF(w io.Writer, rfm int) uint32 {
// 	var (
// 		n   uint32
// 		buf = bufio.NewWriter(w)
// 		wU8 = func(v uint8) {
// 			brd.buf[0] = v
// 			_, _ = buf.Write(brd.buf[:1])
// 		}
// 		wU16 = func(v uint16) {
// 			binary.BigEndian.PutUint16(brd.buf[:2], v)
// 			_, _ = buf.Write(brd.buf[:2])
// 		}
// 		wU32 = func(v uint32) {
// 			binary.BigEndian.PutUint32(brd.buf[:4], v)
// 			_, _ = buf.Write(brd.buf[:4])
// 		}
// 	)
// 	defer func() {
// 		_ = buf.Flush()
// 	}()
//
// 	// DIF DAQ header,
// 	wU8(0xB0)
// 	wU8(difIDOffset + byte(brd.id&7)<<3 + byte(rfm)&3)
// 	// counters
// 	wU32(brd.daq.cycleID[rfm])
// 	wU32(brd.cntHit0(rfm))
// 	wU32(brd.cntHit1(rfm))
// 	wU16(uint16(brd.cntBCID48MSB() & 0xffff))
// 	wU32(brd.cntBCID48LSB())
// 	bcid24 := brd.cntBCID24()
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
// 	for !brd.daqFIFOEmpty(rfm) {
// 		// read HR ID
// 		id := brd.regs.fifo.daq[rfm].r()
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
// 			wU32(brd.regs.fifo.daq[rfm].r())
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
// 	brd.daq.cycleID[rfm]++
// 	return nRAMUnits
// }
