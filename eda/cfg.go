// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-lpc/mim/conddb"
)

type Option func(*config)

func WithThreshold(v uint32) Option {
	return func(cfg *config) {
		cfg.daq.delta = v
	}
}

func WithRFMMask(v uint32) Option {
	return func(cfg *config) {
		cfg.daq.rfm = v
	}
}

func WithRShaper(v uint32) Option {
	return func(cfg *config) {
		cfg.hr.rshaper = v
	}
}

func WithCShaper(v uint32) Option {
	return func(cfg *config) {
		cfg.hr.cshaper = v
	}
}

func WithDevSHM(dir string) Option {
	return func(cfg *config) {
		cfg.run.dir = dir
	}
}

func WithCtlAddr(addr string) Option {
	return func(cfg *config) {
		cfg.ctl.addr = addr
	}
}

func WithConfigDir(dir string) Option {
	return func(cfg *config) {
		if dir == "" {
			return
		}
		cfg.mode = "csv"
		cfg.hr.fname = filepath.Join(dir, "conf_base.csv")
		cfg.daq.fname = filepath.Join(dir, "dac_floor_4rfm.csv")
		cfg.preamp.fname = filepath.Join(dir, "pa_gain_4rfm.csv")
		cfg.mask.fname = filepath.Join(dir, "mask_4rfm.csv")
	}
}

func WithDAQMode(mode string) Option {
	return func(cfg *config) {
		cfg.daq.mode = mode
	}
}

func WithResetBCID(timeout time.Duration) Option {
	return func(cfg *config) {
		cfg.daq.timeout = timeout
	}
}

type config struct {
	mode string // csv or db
	ctl  struct {
		addr string // addr+port to eda-ctl
	}

	hr struct {
		fname   string
		rshaper uint32 // resistance shaper
		cshaper uint32 // capacity shaper

		db dbConfig // configuration from tmv-db
	}

	daq struct {
		mode  string // dcc, noise or inj
		fname string
		floor [nRFM * nHR * 3]uint32
		delta uint32 // delta threshold
		rfm   uint32 // RFM ON mask

		addrs []string // [addr:port]s for sending DIF data

		timeout time.Duration // timeout for reset-BCID
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

func newConfig() config {
	cfg := config{
		mode: "db",
	}
	cfg.hr.db = newDbConfig()
	cfg.hr.cshaper = 3
	cfg.daq.mode = "dcc"
	return cfg
}

// dbConfig holds the configuration from the TMVDb
// for each of the RFMs.
type dbConfig struct {
	asics map[uint8][]conddb.ASIC // rfm-id -> ASICs configuration
}

func newDbConfig() dbConfig {
	return dbConfig{
		asics: make(map[uint8][]conddb.ASIC, nRFM),
	}
}

func (dev *Device) setDBConfig(dif uint8, asics []conddb.ASIC) {
	dev.cfg.mode = "db"
	dev.cfg.hr.db.asics[dif] = make([]conddb.ASIC, len(asics))
	copy(dev.cfg.hr.db.asics[dif], asics)
}

func (dev *Device) configASICs(dif uint8) error {
	asics, ok := dev.cfg.hr.db.asics[dif]
	if !ok {
		return fmt.Errorf("eda: could not find conddb configuration for DIF=%d", dif)
	}

	for i, asic := range asics {
		var (
			cfg = asic.HRConfig()
			ihr = uint32(i)
			n   = len(cfg)
		)
		for i, v := range cfg {
			dev.brd.hrscSetBit(ihr, uint32(n-1-i), uint32(v))
		}
	}
	return nil
}
