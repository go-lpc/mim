// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command eda-daq drives the EDA data acquisition in stand-alone mode.
package main // import "github.com/go-lpc/mim/cmd/eda-daq"

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/go-lpc/mim/eda"
)

func main() {
	var (
		runnbr    = flag.Int("run", -1, "run number")
		threshold = flag.Int("thresh", -1, "threshold")
		rshaper   = flag.Int("rshaper", -1, "R shaper")
		rfmOn     = flag.Int("rfm", -1, "RFM-ON mask")
		srvAddr   = flag.String("srv-addr", ":8877", "eda-srv [address]:port to dial")
		dimAddr   = flag.String("dim-addr", ":8899", "eda-dim [address]:port to dial")
		odir      = flag.String("o", "/home/root/run", "output dir")
	)

	log.SetPrefix("eda-daq: ")
	log.SetFlags(0)

	flag.Parse()

	log.Printf("run=%d threshold=%d R-shaper=%d RFM-ON[3:0]=%d", *runnbr, *threshold, *rshaper, *rfmOn)

	switch {
	case *runnbr < 0:
		log.Fatalf("invalid run number value")
	case *threshold < 0:
		log.Fatalf("invalid threshold value")
	case *rshaper < 0:
		log.Fatalf("invalid R-shaper value")
	case *rfmOn < 0:
		log.Fatalf("invalid RFM mask value")
	}

	err := run(
		uint32(*runnbr), uint32(*threshold), uint32(*rshaper), uint32(*rfmOn),
		*srvAddr, *dimAddr, *odir,
		"/dev/mem", "dev/shm", "/dev/shm/config_base",
	)
	if err != nil {
		log.Fatalf("could not run eda-daq: %+v", err)
	}
}

func run(run, threshold, rshaper, rfm uint32, srvAddr, dimAddr, odir, devmem, devshm, cfgdir string) error {
	conn, err := net.Dial("tcp", srvAddr)
	if err != nil {
		return fmt.Errorf("could not dial eda-srv %q: %w", srvAddr, err)
	}
	defer conn.Close()

	dev, err := eda.NewDevice(
		devmem, run, odir,
		eda.WithCtlAddr(":8877"),
		eda.WithDAQAddr(dimAddr),
		eda.WithThreshold(threshold),
		eda.WithRShaper(rshaper),
		eda.WithRFMMask(rfm),
		eda.WithDevSHM(devshm),
		eda.WithConfigDir(cfgdir),
	)
	if err != nil {
		return fmt.Errorf("could not initialize EDA device: %w", err)
	}
	defer dev.Close()

	err = dev.Configure()
	if err != nil {
		return fmt.Errorf("could not configure EDA device: %w", err)
	}

	err = dev.Initialize()
	if err != nil {
		return fmt.Errorf("could not initialize EDA device: %w", err)
	}

	return nil
}
