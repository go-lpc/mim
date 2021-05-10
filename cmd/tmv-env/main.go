// Copyright ©2021 The mim Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command tmv-env builds a x-compiling environment for a Cyclone-V board running
// a Linux-SoCFPGA-3.10-ltsi kernel.
package main // import "github.com/go-lpc/mim/cmd/tmv-env"

import (
	"flag"
	"log"

	"github.com/go-lpc/mim/internal/xbuild"
)

func main() {
	xmain()
}

func xmain() {
	log.SetPrefix("tmv-env: ")
	log.SetFlags(0)

	flag.Parse()

	err := xbuild.Docker()
	if err != nil {
		log.Fatalf("could not setup environment: %+v", err)
	}
}
