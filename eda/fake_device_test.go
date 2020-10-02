// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"sync"

	"github.com/go-lpc/mim/eda/internal/regs"
)

func fakeFPGA(dev *Device, rfmID int, rfmDone uint32) {
	var (
		fakeCtrl   []uint32
		fakeState  []uint32
		fakeChkSC  []uint32
		fakeCnt24  []uint32
		fakeDaqCSR []uint32
	)

	fakeCtrl = append(fakeCtrl, []uint32{
		0:  0x0,
		1:  0x2,
		2:  0x22,
		3:  0x22,
		4:  0x22,
		5:  0x8000022,
		6:  0x18000022,
		7:  0x18000022,
		8:  0x18000822,
		9:  0x18000822,
		10: 0x18001822,
		11: 0x18001822,
		12: 0x18001c22, // hrscSelectReadRegister
		13: 0x18001c22, // hrscResetSC
		14: 0x18001422,
		15: 0x18001422, // hrscStartSC
		16: 0x18000422,
		// Start
		17: 0x18000022, // syncResetHR
		18: 0x18000022, // syncResetHR
		19: 0x18000022, // DumpRegisters (from Start)
		20: 0x18000022, // cntStart
		21: 0x18400022, // syncArmFIFO
	}...)

	fakeState = append(fakeState, []uint32{
		0: 0,
		1: regs.O_PLL_LCK,
		2: regs.O_PLL_LCK,
		3: regs.O_PLL_LCK,
		4: regs.O_PLL_LCK | rfmDone,
		5: regs.O_PLL_LCK,
		6: regs.O_PLL_LCK | rfmDone,
		7: regs.O_PLL_LCK | rfmDone,
	}...)

	fakeChkSC = append(fakeChkSC, []uint32{
		0: 0xcafefade,
		1: 0x36baffe5, // loopback register rfm
		2: 0x36baffe5, // hrscResetReadRegisters
		3: 0xcafefade, // hrscResetReadRegisters
	}...)

	fakeCnt24 = append(fakeCnt24, []uint32{
		0: 0x0, // first iter in Start: reset-BCID
		1: regs.CMD_RESET_BCID << regs.SHIFT_CMD_CODE_MEM,
		2: regs.CMD_RESET_BCID << regs.SHIFT_CMD_CODE_MEM,
	}...)

	// loop data
	for i := 0; i < 3*1000; i++ {
		fakeCtrl = append(fakeCtrl, []uint32{
			0: 0x1c400022, // syncAckFIFO
		}...)

		fakeState = append(fakeState, []uint32{
			// trigger
			0: regs.O_PLL_LCK | rfmDone | regs.S_START_RO<<regs.SHIFT_SYNCHRO_STATE,
			1: regs.O_PLL_LCK | rfmDone | regs.S_FIFO_READY<<regs.SHIFT_SYNCHRO_STATE,
			2: regs.S_IDLE << regs.SHIFT_SYNCHRO_STATE,
		}...)

		fakeCnt24 = append(fakeCnt24, []uint32{
			0: 0x42, // daqSaveHRDataAsDIF
			1: 0x42, // loop
		}...)

		fakeDaqCSR = append(fakeDaqCSR, []uint32{
			0: (0x1 << 1) | 0x1,
			1: 0xd9003f00,
		}...)
	}

	// exit loop
	{
		fakeCtrl = append(fakeCtrl, []uint32{
			0: 0x1c400022, // syncAckFIFO
		}...)

		fakeState = append(fakeState, []uint32{
			// trigger
			0: regs.O_PLL_LCK | rfmDone | regs.S_START_RO<<regs.SHIFT_SYNCHRO_STATE,
			1: regs.O_PLL_LCK | rfmDone | ^uint32(regs.S_FIFO_READY<<regs.SHIFT_SYNCHRO_STATE),
			2: regs.O_PLL_LCK | rfmDone | regs.S_FIFO_READY<<regs.SHIFT_SYNCHRO_STATE,
			3: regs.S_IDLE << regs.SHIFT_SYNCHRO_STATE,
		}...)

		fakeCnt24 = append(fakeCnt24, []uint32{
			0: 0x42, // daqSaveHRDataAsDIF
			1: 0x42, // loop
		}...)

		fakeDaqCSR = append(fakeDaqCSR, []uint32{
			0: (0x1 << 1) | 0x1,
			1: 0xd9003f00,
		}...)
	}

	fakeCtrl = append(fakeCtrl, []uint32{
		0: 0x1c400022, // cntStop
	}...)

	fakeState = append(fakeState, []uint32{
		0: regs.O_PLL_LCK | rfmDone, // stop trigger
	}...)

	var mu sync.RWMutex
	wrap(dev, &mu, &dev.regs.pio.ctrl, "pio.ctrl", fakeCtrl)
	wrap(dev, &mu, &dev.regs.pio.state, "pio.state", fakeState)
	wrap(dev, &mu, &dev.regs.pio.chkSC[rfmID], "pio.chk-sc", fakeChkSC)
	wrap(dev, &mu, &dev.regs.pio.cnt24, "pio.cnt24", fakeCnt24)

	wrap(
		dev, &mu,
		&dev.regs.fifo.daqCSR[rfmID].pins[regs.ALTERA_AVALON_FIFO_STATUS_REG],
		"fifo.daq-csr[rfm]",
		fakeDaqCSR,
	)
}
