// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"

	"github.com/go-lpc/mim/eda"
)

func main() {
	var (
		addr = flag.String("addr", ":9999", "eda-ctl [addr]:port")
		odir = flag.String("o", "/home/root/run", "output dir")

		devmem = flag.String("dev-mem", "/dev/mem", "")
		devshm = flag.String("dev-shm", "/dev/shm", "")
		cfgdir = flag.String("cfg-dir", "/dev/shm/config_base", "")
	)

	log.SetPrefix("eda-ctl: ")
	log.SetFlags(0)

	flag.Parse()

	err := eda.Serve(*addr, *odir, *devmem, *devshm, *cfgdir)
	if err != nil {
		log.Fatalf("could not create eda-ctl service: %+v", err)
	}
}
