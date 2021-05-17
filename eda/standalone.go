// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-lpc/mim/eda/internal/regs"
)

type standalone struct {
	dev  *Device
	run  uint32
	stop chan os.Signal
}

func newStandalone(odir, devmem, devshm string, run int, opts ...Option) (*standalone, error) {
	dev, err := newDevice(devmem, odir, devshm, opts...)
	if err != nil {
		return nil, fmt.Errorf("could not create EDA device: %w", err)
	}
	dev.id = 1
	srv := &standalone{
		dev:  dev,
		run:  uint32(run),
		stop: make(chan os.Signal, 1),
	}
	return srv, nil
}

func RunStandalone(cfg string, run, threshold, rfmMask int, opts ...Option) error {
	const (
		odir   = "/home/root/run"
		devmem = "/dev/mem"
		devshm = "/dev/shm"
	)

	xopts := []Option{
		WithThreshold(uint32(threshold)),
		WithRFMMask(uint32(rfmMask)),
		WithConfigDir(cfg),
	}
	xopts = append(xopts, opts...)

	srv, err := newStandalone(odir, devmem, devshm, run, xopts...)
	if err != nil {
		return fmt.Errorf("could not create standalone server: %w", err)
	}
	return srv.runDAQ()
}

func (srv *standalone) runDAQ() error {
	dev := srv.dev
	defer dev.Close()

	signal.Notify(srv.stop, os.Interrupt, syscall.SIGUSR1)
	defer signal.Stop(srv.stop)

	err := dev.Configure()
	if err != nil {
		return fmt.Errorf("could not configure EDA board: %w", err)
	}

	// --- init FPGA ---

	// reset FPGA and set clock.
	err = dev.syncResetFPGA()
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

	err = dev.syncSelectCmdSoft()
	if err != nil {
		return fmt.Errorf("eda: could select cmd-soft mode: %w", err)
	}

	// --- init HR ---
	err = dev.initHR()
	if err != nil {
		return fmt.Errorf("eda: could not initialize HardRoc: %w", err)
	}

	// --- init run ---
	out, err := os.Create(filepath.Join(
		dev.cfg.run.dir, fmt.Sprintf("hr_daq_%03d.bin", srv.run),
	))
	if err != nil {
		return fmt.Errorf("eda: could not create output DAQ file: %w", err)
	}
	defer out.Close()

	cycleID := 0

	for _, rfm := range dev.rfms {
		err = dev.daqFIFOInit(rfm)
		if err != nil {
			return fmt.Errorf("eda: could not initialize DAQ FIFO (RFM=%d): %w", rfm, err)
		}
	}

	err = dev.cntReset()
	if err != nil {
		return fmt.Errorf("eda: could not reset counters: %w", err)
	}

	err = dev.syncResetBCID()
	if err != nil {
		return fmt.Errorf("eda: could not reset BCID: %w", err)
	}

	err = dev.syncStart()
	if err != nil {
		return fmt.Errorf("eda: could not start acquisition: %w", err)
	}
	dev.msg.Printf("sync-state: %[1]d 0x%[1]x\n", dev.syncState())

	err = dev.syncArmFIFO()
	if err != nil {
		return fmt.Errorf("eda: could not arm FIFO: %w", err)
	}

readout:
	for {
		select {
		case <-srv.stop:
			dev.msg.Printf("stopping acquisition...")
			break readout
		default:
		}
		dev.msg.Printf("trigger %d", cycleID)

		//	ramloop:
		//		for {
		//			switch state := dev.syncState(); {
		//			case state == regs.S_RAMFULL:
		//				// ok.
		//				dev.msg.Printf("ramfull")
		//				break ramloop
		//			case state > regs.S_RAMFULL:
		//				dev.msg.Printf("state: %d", state)
		//				break ramloop
		//			}
		//		}

		err = dev.syncRAMFullExt()
		if err != nil {
			return fmt.Errorf("eda: could not sync for RAMFULL-EXT: %w", err)
		}

		// wait until data is ready.
	fifo:
		for {
			switch state := dev.syncState(); state {
			case regs.S_FIFO_READY:
				break fifo
			default:
				select {
				case <-srv.stop:
					dev.msg.Printf("stopping acquisition...")
					break readout
				default:
				}
			}
		}

		// read hardroc data.
		for _, rfm := range dev.rfms {
			dev.daqWriteDIFData(out, rfm)
		}
		err = dev.syncAckFIFO()
		if err != nil {
			return fmt.Errorf("eda: could not ACK FIFO: %w", err)
		}

		err = dev.syncStart()
		if err != nil {
			return fmt.Errorf("eda: could not start ACQ (cycle=%d): %w", cycleID, err)
		}

		cycleID++
	}

	err = dev.syncStop()
	if err != nil {
		return fmt.Errorf("eda: could not stop ACQ: %w", err)
	}

	err = out.Close()
	if err != nil {
		return fmt.Errorf("eda: could not close output raw file: %w", err)
	}

	return nil
}
