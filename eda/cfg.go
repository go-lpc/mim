// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"fmt"

	"github.com/go-lpc/mim/conddb"
)

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
			dev.hrscSetBit(ihr, uint32(n-1-i), uint32(v))
		}
	}
	return nil
}
