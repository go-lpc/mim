// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// dif-dump decodes and displays DIF data files.
//
// Usage: dif-dump [OPTIONS] FILE1 [FILE2 [FILE3 ...]]
//
// Example:
//
//  $> dif-dump ./testdata/Event_425050855_109_109_183
//  === DIF-ID 0xb7 ===
//  DIF trigger:        109
//  ACQ trigger:          0
//  Gbl trigger:        109
//  Abs BCID:     425050855
//  Time DIF:       1864732
//  Frames:             183
//    hroc=0x01 BCID= 1448778 000000000000000000000000000005f0
//    hroc=0x01 BCID= 1533835 0400000055b955540000040000000000
//    hroc=0x01 BCID= 1520655 00000010000000000000000000000000
//  [...]
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/go-lpc/mim/internal/eformat"
)

const usage = `dif-dump decodes and displays DIF data files.

Usage: dif-dump [OPTIONS] FILE1 [FILE2 [FILE3 ...]]

Example:

 $> dif-dump ./testdata/Event_425050855_109_109_183
 === DIF-ID 0xb7 ===
 DIF trigger:        109
 ACQ trigger:          0
 Gbl trigger:        109
 Abs BCID:     425050855
 Time DIF:       1864732
 Frames:             183
   hroc=0x01 BCID= 1448778 000000000000000000000000000005f0
   hroc=0x01 BCID= 1533835 0400000055b955540000040000000000
   hroc=0x01 BCID= 1520655 00000010000000000000000000000000
 [...]

`

func main() {
	xmain(os.Stdout, os.Args[1:])
}

func xmain(w io.Writer, args []string) {
	log.SetPrefix("dif-dump: ")
	log.SetFlags(0)

	var (
		fset = flag.NewFlagSet("dif", flag.ExitOnError)

		eda = fset.Bool("eda", false, "enable EDA hack")
	)

	fset.Usage = func() {
		fmt.Print(usage)
		fset.PrintDefaults()
	}

	err := fset.Parse(args)
	if err != nil {
		log.Fatalf("could not parse input arguments: %+v", err)
	}

	if fset.NArg() == 0 {
		fset.Usage()
		log.Fatalf("missing path to input DIF file")
	}

	for _, fname := range fset.Args() {
		err := process(w, fname, *eda)
		if err != nil {
			log.Fatalf("could not dump file %q: %+v", fname, err)
		}
	}
}

func process(w io.Writer, fname string, eda bool) error {
	wbuf := bufio.NewWriter(w)
	defer wbuf.Flush()

	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("could not open %q: %w", fname, err)
	}
	defer f.Close()

	dec := eformat.NewDecoder(0, f)
	dec.IsEDA = eda
loop:
	for {
		var d eformat.DIF
		err := dec.Decode(&d)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break loop
			}
			return fmt.Errorf("could not decode DIF: %w", err)
		}
		fmt.Fprintf(wbuf, "=== DIF-ID 0x%x ===\n", d.Header.ID)
		fmt.Fprintf(wbuf, "DIF trigger: % 10d\n", d.Header.DTC)
		fmt.Fprintf(wbuf, "ACQ trigger: % 10d\n", d.Header.ATC)
		fmt.Fprintf(wbuf, "Gbl trigger: % 10d\n", d.Header.GTC)
		fmt.Fprintf(wbuf, "Abs BCID:    % 10d\n", d.Header.AbsBCID)
		fmt.Fprintf(wbuf, "Time DIF:    % 10d\n", d.Header.TimeDIFTC)
		fmt.Fprintf(wbuf, "Frames:      % 10d\n", len(d.Frames))

		for _, frame := range d.Frames {
			fmt.Fprintf(wbuf, "  hroc=0x%02x BCID=% 8d %x\n",
				frame.Header, frame.BCID, frame.Data,
			)
		}
	}

	return nil
}
