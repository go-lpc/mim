// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command eda-daq-noise is a stand-alone program for data acquisition
// on the EDA board (DCC needed only for clock), with multiple RFMs.
package main // import "github.com/go-lpc/mim/cmd/eda-daq-noise"

import (
	"flag"
	"log"

	"github.com/go-lpc/mim/eda"
)

func main() {
	var (
		cfg    = flag.String("cfg-file", "", "path to configuration file")
		run    = flag.Int("run", 0, "run number to use for data acquisition")
		rfm    = flag.Int("rfm-mask", 0, "RFM mask")
		thresh = flag.Int("thresh", 0, "DAC threshold")
	)

	flag.Parse()

	log.SetPrefix("eda-daq: ")
	log.SetFlags(0)

	xmain(*cfg, *run, *thresh, *rfm)
}

func xmain(cfg string, run, threshold, rfmMask int) {
	err := eda.RunStandalone(cfg, run, threshold, rfmMask)
	if err != nil {
		log.Fatalf("could not run standalone: %+v", err)
	}
}
