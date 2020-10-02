// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/go-lpc/mim/conddb"
)

func TestCompareConfig(t *testing.T) {
	for _, tc := range []struct {
		rfms []int
	}{
		{
			rfms: []int{1},
		},
		{
			rfms: []int{2},
		},
	} {
		t.Run("", func(t *testing.T) {
			for _, rfm := range tc.rfms {
				var asics []conddb.ASIC
				f, err := os.Open(fmt.Sprintf("testdata/asic-rfm-%03d.json", rfm))
				if err != nil {
					t.Fatalf("could not load ASICs for RFM=%d: %+v", rfm, err)
				}
				defer f.Close()

				err = json.NewDecoder(f).Decode(&asics)
				if err != nil {
					t.Fatalf("could not decode ASICs for RFM=%d: %+v", rfm, err)
				}

				sort.Slice(asics, func(i, j int) bool {
					ai := asics[i]
					aj := asics[j]
					return ai.Header < aj.Header
				})

				var (
					devDB  Device
					devCSV Device

					rfmsDB  = []int{rfm}
					rfmsCSV = []int{rfm - 1}
				)

				err = testCfgWithDB(&devDB, asics, 3, rfmsDB)
				if err != nil {
					t.Fatalf("could not configure dev-DB: %+v", err)
				}

				err = testCfgWithCSV(&devCSV, 50, 3, rfmsCSV)
				if err != nil {
					t.Fatalf("could not configure dev-CSV: %+v", err)
				}

				for i := range asics {
					var (
						ihr  = (nHR - 1 - i) * nBytesCfgHR
						buf1 = devDB.cfg.hr.data[ihr : ihr+nBytesCfgHR]
						buf2 = devCSV.cfg.hr.data[ihr : ihr+nBytesCfgHR]
					)
					if !bytes.Equal(buf1, buf2) {
						t.Errorf("asic-%d: hr-data NOT OK", i)
					}
				}
			}
		})
	}
}

func testCfgWithDB(dev *Device, asics []conddb.ASIC, rshaper uint32, rfms []int) error {
	WithRShaper(rshaper)(dev)
	dev.cfg.hr.cshaper = 3
	dev.cfg.hr.data = dev.cfg.hr.buf[4:]
	dev.cfg.hr.db = newDbConfig()
	dev.rfms = rfms

	{
		rfmID := asics[0].DIFID
		dev.setDBConfig(rfmID, asics)
		err := dev.configASICs(rfmID)
		if err != nil {
			return fmt.Errorf("could not configure ASICs for rfm=%d: %w", rfmID, err)
		}
	}
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
		dev.hrscSetChipID(hr, hr+1)
	}

	for i := range dev.rfms {
		rfm := uint32(dev.rfms[i])
		for hr := uint32(0); hr < nHR; hr++ {
			for ch := uint32(0); ch < nChans; ch++ {
				m0 := bitU64(asics[hr].Mask0, ch)
				m1 := bitU64(asics[hr].Mask1, ch)
				m2 := bitU64(asics[hr].Mask2, ch)

				mask := uint32(m0 | m1<<1 | m2<<2)
				dev.hrscSetMask(hr, ch, mask)
			}

			dev.hrscSetDAC0(hr, uint32(asics[hr].B0))
			dev.hrscSetDAC1(hr, uint32(asics[hr].B1))
			dev.hrscSetDAC2(hr, uint32(asics[hr].B2))

			for ch := uint32(0); ch < nChans; ch++ {
				v, err := strconv.ParseUint(string(asics[hr].PreAmpGain[2*ch:2*ch+2]), 16, 8)
				if err != nil {
					return err
				}
				gain := uint32(v)
				dev.cfg.preamp.gains[nChans*(nHR*rfm+hr)+ch] = gain
				dev.hrscSetPreAmp(hr, ch, gain)
			}
		}
	}
	return nil
}

func testCfgWithCSV(dev *Device, thresh, rshaper uint32, rfms []int) error {
	WithConfigDir("testdata")(dev)
	WithThreshold(thresh)(dev)
	WithRShaper(rshaper)(dev)
	dev.cfg.hr.db = newDbConfig()
	dev.cfg.hr.cshaper = 3
	dev.cfg.hr.data = dev.cfg.hr.buf[4:]
	dev.rfms = rfms

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
		dev.hrscSetChipID(hr, hr+1)
	}

	// for each active RFM, tune the configuration and send it.
	for _, rfm := range dev.rfms {
		// mask unused channels
		for hr := uint32(0); hr < nHR; hr++ {
			for ch := uint32(0); ch < nChans; ch++ {
				mask := dev.cfg.mask.table[nChans*(nHR*uint32(rfm)+hr)+ch]
				dev.hrscSetMask(hr, ch, mask)
			}
		}

		// set DAC thresholds
		for hr := uint32(0); hr < nHR; hr++ {
			dev.cfg.daq.floor[3*(nHR*uint32(rfm)+hr)+0] += dev.cfg.daq.delta
			dev.cfg.daq.floor[3*(nHR*uint32(rfm)+hr)+1] += dev.cfg.daq.delta
			dev.cfg.daq.floor[3*(nHR*uint32(rfm)+hr)+2] += dev.cfg.daq.delta

			th0 := dev.cfg.daq.floor[3*(nHR*uint32(rfm)+hr)+0]
			th1 := dev.cfg.daq.floor[3*(nHR*uint32(rfm)+hr)+1]
			th2 := dev.cfg.daq.floor[3*(nHR*uint32(rfm)+hr)+2]
			dev.hrscSetDAC0(hr, th0)
			dev.hrscSetDAC1(hr, th1)
			dev.hrscSetDAC2(hr, th2)
		}

		for hr := uint32(0); hr < nHR; hr++ {
			for ch := uint32(0); ch < nChans; ch++ {
				gain := dev.cfg.preamp.gains[nChans*hr+ch]
				dev.hrscSetPreAmp(hr, ch, gain)
			}
		}

	}
	return nil
}
