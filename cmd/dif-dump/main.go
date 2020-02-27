// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command dif-dump decodes and displays DIF data.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/go-lpc/mim/dif"
	"golang.org/x/xerrors"
)

func main() {
	log.SetPrefix("dif-dump: ")
	log.SetFlags(0)

	flag.Parse()
	fname := flag.Arg(0)

	f, err := os.Open(fname)
	if err != nil {
		log.Fatalf("could not open %q: %+v", fname, err)
	}
	defer f.Close()

	dec := dif.NewDecoder(difIDFrom(f), f)
loop:
	for {
		var d dif.DIF
		err := dec.Decode(&d)
		if err != nil {
			if xerrors.Is(err, io.EOF) {
				break loop
			}
			log.Fatalf("could not decode DIF: %+v", err)
		}
		fmt.Printf("=== DIF-ID 0x%x ===\n", d.Header.ID)
		fmt.Printf("DIF trigger: % 10d\n", d.Header.DTC)
		fmt.Printf("ACQ trigger: % 10d\n", d.Header.ATC)
		fmt.Printf("Gbl trigger: % 10d\n", d.Header.GTC)
		fmt.Printf("Abs BCID:    % 10d\n", d.Header.AbsBCID)
		fmt.Printf("Time DIF:    % 10d\n", d.Header.TimeDIFTC)
		fmt.Printf("Frames:      % 10d\n", len(d.Frames))

		for _, frame := range d.Frames {
			fmt.Printf("  hroc=0x%02x BCID=% 8d %x\n",
				frame.Header, frame.BCID, frame.Data,
			)
		}
	}
}

func difIDFrom(f io.ReaderAt) uint8 {
	p := []byte{0}
	_, err := f.ReadAt(p, 1)
	if err != nil {
		panic(err)
	}
	return uint8(p[0])
}
