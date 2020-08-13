// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-lpc/mim/eda/internal/regs"
)

func TestRun(t *testing.T) {
	const (
		daqAddr = ":9999"
		edaID   = 1
	)

	sink := func(done chan int, rfm int) string {
		srv, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("could not create rfm-server: %+v", err)
		}
		go func() {
			conn, err := srv.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					t.Errorf("could not accept connection from rfm-server: %+v", err)
					return
				}
			}
			defer conn.Close()
			defer srv.Close()

			buf := make([]byte, 8+daqBufferSize)
			for {
				select {
				case <-done:
					return
				default:
					_, err := conn.Read(buf[:8])
					if err != nil {
						if errors.Is(err, io.EOF) {
							return
						}
						t.Errorf("could not read DAQ DIF header: %+v", err)
						continue
					}
					size := binary.LittleEndian.Uint32(buf[4:8])
					if size == 0 {
						continue
					}
					_, err = conn.Read(buf[:size])
					if err != nil {
						t.Errorf("could not read DAQ DIF data: %+v", err)
						continue
					}
					copy(buf[:4], "ACK\x00")
					_, err = conn.Write(buf[:4])
					if err != nil {
						t.Errorf("could not send back ACK: %+v", err)
						continue
					}
				}
			}
		}()
		return srv.Addr().String()
	}

	for _, tc := range []struct {
		rfm  int
		done uint32
	}{
		{
			rfm:  0,
			done: regs.O_SC_DONE_0,
		},
		{
			rfm:  1,
			done: regs.O_SC_DONE_1,
		},
		{
			rfm:  2,
			done: regs.O_SC_DONE_2,
		},
		{
			rfm:  3,
			done: regs.O_SC_DONE_3,
		},
	} {
		t.Run(fmt.Sprintf("rfm=%d", tc.rfm), func(t *testing.T) {
			done := make(chan int)
			defer close(done)

			rfmAddr := sink(done, tc.rfm)

			tmpdir, err := ioutil.TempDir("", "eda-daq-")
			if err != nil {
				t.Fatalf("could not create tmp-dir: %+v", err)
			}
			defer os.RemoveAll(tmpdir)

			devmem, err := os.Create(filepath.Join(tmpdir, "dev.mem"))
			if err != nil {
				t.Fatalf("could not create fake dev-mem: %+v", err)
			}
			defer devmem.Close()

			_, err = devmem.WriteAt([]byte{1}, regs.LW_H2F_BASE+regs.LW_H2F_SPAN)
			if err != nil {
				t.Fatalf("could not write to dev-mem: %+v", err)
			}
			err = devmem.Close()
			if err != nil {
				t.Fatalf("could not close devmem: %+v", err)
			}

			dev, err := NewDevice(devmem.Name(), 42, tmpdir,
				WithDevSHM(tmpdir),
				WithCtlAddr(""),
				WithConfigDir("./testdata"),
				WithThreshold(0),
				WithRFMMask(0),
				WithRShaper(0),
				WithCShaper(3),
			)
			if err != nil {
				t.Fatalf("could not create fake device: %+v", err)
			}
			defer dev.Close()

			dev.id = edaID
			dev.rfms = []int{tc.rfm}
			dev.cfg.daq.addrs = []string{rfmAddr}

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
				4: regs.O_PLL_LCK | tc.done,
				5: regs.O_PLL_LCK,
				6: regs.O_PLL_LCK | tc.done,
				7: regs.O_PLL_LCK | tc.done,
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
					0: regs.O_PLL_LCK | tc.done | regs.S_START_RO<<regs.SHIFT_SYNCHRO_STATE,
					1: regs.O_PLL_LCK | tc.done | regs.S_FIFO_READY<<regs.SHIFT_SYNCHRO_STATE,
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
					0: regs.O_PLL_LCK | tc.done | regs.S_START_RO<<regs.SHIFT_SYNCHRO_STATE,
					1: regs.O_PLL_LCK | tc.done | ^uint32(regs.S_FIFO_READY<<regs.SHIFT_SYNCHRO_STATE),
					2: regs.O_PLL_LCK | tc.done | regs.S_FIFO_READY<<regs.SHIFT_SYNCHRO_STATE,
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
				0: regs.O_PLL_LCK | tc.done, // stop trigger
			}...)

			wrap(dev, &dev.regs.pio.ctrl, "pio.ctrl", fakeCtrl)
			wrap(dev, &dev.regs.pio.state, "pio.state", fakeState)
			wrap(dev, &dev.regs.pio.chkSC[tc.rfm], "pio.chk-sc", fakeChkSC)
			wrap(dev, &dev.regs.pio.cnt24, "pio.cnt24", fakeCnt24)

			wrap(
				dev,
				&dev.regs.fifo.daqCSR[tc.rfm].pins[regs.ALTERA_AVALON_FIFO_STATUS_REG],
				"fifo.daq-csr[rfm]",
				fakeDaqCSR,
			)

			err = dev.Configure()
			if err != nil {
				t.Fatalf("could not configure device: %+v", err)
			}

			err = dev.Initialize()
			if err != nil {
				t.Fatalf("could not initialize device: %+v", err)
			}

			err = dev.Start()
			if err != nil {
				t.Fatalf("could not start run: %+v", err)
			}

			err = dev.Stop()
			if err != nil {
				t.Fatalf("could not stop run: %+v", err)
			}

			err = dev.Close()
			if err != nil {
				t.Fatalf("could not close device: %+v", err)
			}
		})
	}
}

func TestDumpRegisters(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "eda-daq-")
	if err != nil {
		t.Fatalf("could not create tmp-dir: %+v", err)
	}
	defer os.RemoveAll(tmpdir)

	devmem, err := os.Create(filepath.Join(tmpdir, "dev.mem"))
	if err != nil {
		t.Fatalf("could not create fake dev-mem: %+v", err)
	}
	defer devmem.Close()

	_, err = devmem.WriteAt([]byte{1}, regs.LW_H2F_BASE+regs.LW_H2F_SPAN)
	if err != nil {
		t.Fatalf("could not write to dev-mem: %+v", err)
	}
	err = devmem.Close()
	if err != nil {
		t.Fatalf("could not close devmem: %+v", err)
	}

	dev, err := NewDevice(devmem.Name(), 42, tmpdir,
		WithDevSHM(tmpdir),
		WithConfigDir("./testdata"),
	)
	if err != nil {
		t.Fatalf("could not create fake device: %+v", err)
	}
	defer dev.Close()

	wrap(dev, &dev.regs.pio.state, "pio.state", []uint32{
		0x1,
		0x8 << regs.SHIFT_CMD_CODE_MEM,
	})

	wrap(dev, &dev.regs.pio.ctrl, "pio.ctrl", []uint32{
		0x2,
		0x2,
	})

	wrap(dev, &dev.regs.pio.pulser, "pio.pulser", []uint32{
		0x3,
	})

	wrap(dev, &dev.regs.pio.cntHit0[0], "pio.cntHit0[0]", []uint32{
		0x4,
	})

	wrap(dev, &dev.regs.pio.cntHit0[1], "pio.cntHit0[1]", []uint32{
		0x5,
	})

	wrap(dev, &dev.regs.pio.cntHit0[2], "pio.cntHit0[2]", []uint32{
		0x6,
	})

	wrap(dev, &dev.regs.pio.cntHit0[3], "pio.cntHit0[3]", []uint32{
		0x7,
	})

	wrap(dev, &dev.regs.pio.cntHit1[0], "pio.cntHit0[0]", []uint32{
		0x8,
	})

	wrap(dev, &dev.regs.pio.cntHit1[1], "pio.cntHit1[1]", []uint32{
		0x9,
	})

	wrap(dev, &dev.regs.pio.cntHit1[2], "pio.cntHit1[2]", []uint32{
		0x10,
	})

	wrap(dev, &dev.regs.pio.cntHit1[3], "pio.cntHit1[3]", []uint32{
		0x11,
	})

	wrap(dev, &dev.regs.pio.cntTrig, "pio.cntTrig", []uint32{
		0x12,
	})

	wrap(dev, &dev.regs.pio.cnt48MSB, "pio.cnt48MSB", []uint32{
		0x13,
	})

	wrap(dev, &dev.regs.pio.cnt48LSB, "pio.cnt48LSB", []uint32{
		0x14,
	})

	wrap(dev, &dev.regs.fifo.daqCSR[0].pins[0], "fifo.daqCSR[0]", []uint32{
		0x15,
	})

	wrap(dev, &dev.regs.fifo.daqCSR[1].pins[0], "fifo.daqCSR[1]", []uint32{
		0x16,
	})

	wrap(dev, &dev.regs.fifo.daqCSR[2].pins[0], "fifo.daqCSR[2]", []uint32{
		0x17,
	})

	wrap(dev, &dev.regs.fifo.daqCSR[3].pins[0], "fifo.daqCSR[3]", []uint32{
		0x18,
	})

	o := new(strings.Builder)
	err = dev.DumpRegisters(o)
	if err != nil {
		t.Fatalf("could not run dump-registers: %+v", err)
	}

	want := `pio.state=       0x00000001
pio.ctrl=        0x00000002
pio.pulser=      0x00000003
pio.cnt.hit0[0]= 0x00000004
pio.cnt.hit0[1]= 0x00000005
pio.cnt.hit0[2]= 0x00000006
pio.cnt.hit0[3]= 0x00000007
pio.cnt.hit1[0]= 0x00000008
pio.cnt.hit1[1]= 0x00000009
pio.cnt.hit1[2]= 0x00000010
pio.cnt.hit1[3]= 0x00000011
pio.cnt.trig=    0x00000012
pio.cnt48MSB=    0x00000013
pio.cnt48LSB=    0x00000014
fifo.daqCSR[0]=  0x00000015
fifo.daqCSR[1]=  0x00000016
fifo.daqCSR[2]=  0x00000017
fifo.daqCSR[3]=  0x00000018
synchro FSM state= 8 (stop run)
`

	if got, want := o.String(), want; got != want {
		t.Fatalf(
			"invalid dump-registers:\ngot:\n%s\nwant:\n%s\n",
			got, want,
		)
	}
}

func TestDumpFIFOStatus(t *testing.T) {
	for _, rfmID := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("rfm=%d", rfmID), func(t *testing.T) {
			tmpdir, err := ioutil.TempDir("", "eda-daq-")
			if err != nil {
				t.Fatalf("could not create tmp-dir: %+v", err)
			}
			defer os.RemoveAll(tmpdir)

			devmem, err := os.Create(filepath.Join(tmpdir, "dev.mem"))
			if err != nil {
				t.Fatalf("could not create fake dev-mem: %+v", err)
			}
			defer devmem.Close()

			_, err = devmem.WriteAt([]byte{1}, regs.LW_H2F_BASE+regs.LW_H2F_SPAN)
			if err != nil {
				t.Fatalf("could not write to dev-mem: %+v", err)
			}
			err = devmem.Close()
			if err != nil {
				t.Fatalf("could not close devmem: %+v", err)
			}

			dev, err := NewDevice(devmem.Name(), 42, tmpdir,
				WithDevSHM(tmpdir),
				WithConfigDir("./testdata"),
			)
			if err != nil {
				t.Fatalf("could not create fake device: %+v", err)
			}
			defer dev.Close()

			dev.rfms = []int{rfmID}

			wrap(dev, &dev.regs.fifo.daqCSR[rfmID].pins[regs.ALTERA_AVALON_FIFO_LEVEL_REG], "fifo-level", []uint32{
				0x1,
			})

			wrap(dev, &dev.regs.fifo.daqCSR[rfmID].pins[regs.ALTERA_AVALON_FIFO_STATUS_REG], "fifo-status", []uint32{
				0xffffff,
			})

			wrap(dev, &dev.regs.fifo.daqCSR[rfmID].pins[regs.ALTERA_AVALON_FIFO_EVENT_REG], "fifo-event", []uint32{
				0xffffff,
			})

			wrap(dev, &dev.regs.fifo.daqCSR[rfmID].pins[regs.ALTERA_AVALON_FIFO_IENABLE_REG], "fifo-ienable", []uint32{
				0xffffff,
			})

			wrap(dev, &dev.regs.fifo.daqCSR[rfmID].pins[regs.ALTERA_AVALON_FIFO_ALMOSTFULL_REG], "fifo-almost-full", []uint32{
				128,
			})

			wrap(dev, &dev.regs.fifo.daqCSR[rfmID].pins[regs.ALTERA_AVALON_FIFO_ALMOSTEMPTY_REG], "fifo-almost-empty", []uint32{
				255,
			})

			o := new(strings.Builder)
			err = dev.DumpFIFOStatus(o, rfmID)

			if err != nil {
				t.Fatalf("could not dump-fifo-status: %+v", err)
			}

			want := `---- FIFO status -------
fill level:		1
istatus:	 full:	 1	 empty:	 1	 almost full:	 1	 almost empty:	 1	 overflow:	 1	 underflow:	 1
event:  	 full:	 1	 empty:	 1	 almost full:	 1	 almost empty:	 1	 overflow:	 1	 underflow:	 1
ienable:	 full:	 1	 empty:	 1	 almost full:	 1	 almost empty:	 1	 overflow:	 1	 underflow:	 1
almostfull:		128
almostempty:		255


`

			if got, want := o.String(), want; got != want {
				t.Fatalf(
					"invalid dump-fifo:\ngot:\n%s\nwant:\n%s\n",
					got, want,
				)
			}
		})
	}
}

func TestDumpConfig(t *testing.T) {
	for _, rfmID := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("rfm=%d", rfmID), func(t *testing.T) {
			tmpdir, err := ioutil.TempDir("", "eda-daq-")
			if err != nil {
				t.Fatalf("could not create tmp-dir: %+v", err)
			}
			defer os.RemoveAll(tmpdir)

			devmem, err := os.Create(filepath.Join(tmpdir, "dev.mem"))
			if err != nil {
				t.Fatalf("could not create fake dev-mem: %+v", err)
			}
			defer devmem.Close()

			_, err = devmem.WriteAt([]byte{1}, regs.LW_H2F_BASE+regs.LW_H2F_SPAN)
			if err != nil {
				t.Fatalf("could not write to dev-mem: %+v", err)
			}
			err = devmem.Close()
			if err != nil {
				t.Fatalf("could not close devmem: %+v", err)
			}

			dev, err := NewDevice(devmem.Name(), 42, tmpdir,
				WithDevSHM(tmpdir),
				WithConfigDir("./testdata"),
			)
			if err != nil {
				t.Fatalf("could not create fake device: %+v", err)
			}
			defer dev.Close()

			dev.rfms = []int{rfmID}

			want := new(strings.Builder)
			{
				buf := make([]byte, szCfgHR)
				for i := range buf {
					buf[i] = byte(i + rfmID)
					j := 8 * (nHR*nBytesCfgHR - i - 1)
					fmt.Fprintf(want, "%d\t%x\n", j, buf[i])
				}
				_, err = dev.regs.ramSC[rfmID].w(buf)
				if err != nil {
					t.Fatalf("could not write test buffer: %+v", err)
				}
			}

			o := new(strings.Builder)
			err = dev.DumpConfig(o, rfmID)
			if err != nil {
				t.Fatalf("could not dump-fifo-status: %+v", err)
			}

			if got, want := o.String(), want.String(); got != want {
				t.Fatalf(
					"invalid dump-config:\ngot:\n%s\nwant:\n%s\n",
					got, want,
				)
			}
		})
	}
}

func TestNewDevice(t *testing.T) {
	t.Skip()
	for _, tc := range []struct {
		name string
		ctl  string
		err  error
	}{
		{
			name: "invalid-eda-ctl-addr",
			err:  fmt.Errorf("eda: could not dial DAQ data sink \":9999\": dial tcp :9999: connect: connection refused"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpdir, err := ioutil.TempDir("", "eda-daq-")
			if err != nil {
				t.Fatalf("could not create tmp-dir: %+v", err)
			}
			defer os.RemoveAll(tmpdir)

			devmem, err := os.Create(filepath.Join(tmpdir, "dev.mem"))
			if err != nil {
				t.Fatalf("could not create fake dev-mem: %+v", err)
			}
			defer devmem.Close()

			_, err = devmem.WriteAt([]byte{1}, regs.LW_H2F_BASE+regs.LW_H2F_SPAN)
			if err != nil {
				t.Fatalf("could not write to dev-mem: %+v", err)
			}
			err = devmem.Close()
			if err != nil {
				t.Fatalf("could not close devmem: %+v", err)
			}

			dev, err := NewDevice(devmem.Name(), 42, tmpdir,
				WithDevSHM(tmpdir),
				WithCtlAddr(tc.ctl),
				WithConfigDir("./testdata"),
			)

			switch {
			case err != nil && tc.err != nil:
				if got, want := err.Error(), tc.err.Error(); got != want {
					t.Fatalf("invalid error:\ngot= %v\nwant=%v", got, want)
				}
				return
			case err == nil && tc.err != nil:
				t.Fatalf("invalid error:\ngot= %v\nwant=%v", nil, tc.err.Error())
			case err != nil && tc.err == nil:
				t.Fatalf("could not create fake device: %+v", err)
			case err == nil && tc.err == nil:
				// ok
			}
			defer dev.Close()
		})
	}
}

type fakeReg32 struct {
	name string
	mu   sync.RWMutex
	cr   int
	cw   int

	rs []uint32
}

const dbg = false

func wrap(dev *Device, reg *reg32, name string, rs []uint32) *fakeReg32 {
	var (
		mon = fakeReg32{
			name: name,
			rs:   rs,
		}
		r = reg.r
		w = reg.w
	)
	reg.r = func() uint32 {
		mon.mu.Lock()
		defer mon.mu.Unlock()
		cr := mon.cr
		mon.cr++
		v := r()
		vv := v
		ok := false
		if cr < len(mon.rs) {
			v = mon.rs[cr]
			ok = true
		}
		if dbg {
			dev.msg.Printf("%s: nr=%d, v=0x%x|0x%x", name, cr, v, vv)
		}
		if !ok {
			dev.msg.Printf("%s: nr=%d, v=0x%x|0x%x", name, cr, v, vv)
			panic("exhaust: " + name)
		}
		return v
	}
	reg.w = func(v uint32) {
		mon.mu.Lock()
		defer mon.mu.Unlock()
		mon.cw++
		cw := mon.cw
		if dbg {
			dev.msg.Printf("%s: nw=%d, v=0x%x", name, cw-1, v)
		}
		w(v)
	}
	return &mon
}
