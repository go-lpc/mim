// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command lcio2eda converts a LCIO file into an EDA raw data file.
package main // import "github.com/go-lpc/mim/cmd/lcio2eda"

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/go-lpc/mim/internal/xcnv"
	"go-hep.org/x/hep/lcio"
)

var (
	msg = log.New(os.Stdout, "lcio2eda: ", 0)
)

func main() {
	var (
		oname = flag.String("o", "out.raw", "path to output EDA raw file")
	)

	flag.Usage = func() {
		fmt.Printf(`Usage: lcio2eda [OPTIONS] file.lcio

ex:
 $> lcio2eda -o out.raw ./input.lcio

options:
`)
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		msg.Fatalf("missing input LCIO file")
	}

	if *oname == "" {
		flag.Usage()
		msg.Fatalf("invalid output EDA file name")
	}

	err := process(*oname, flag.Arg(0))
	if err != nil {
		msg.Fatalf("could not convert LCIO file: %+v", err)
	}
}

func numEvents(fname string) (int64, error) {
	r, err := lcio.Open(fname)
	if err != nil {
		return 0, fmt.Errorf("could not open %q: %w", fname, err)
	}
	defer r.Close()

	var n int64
	for r.Next() {
		n++
	}

	err = r.Err()
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("could not assess number of events in %q: %w", fname, err)
	}

	return n, nil
}

func process(oname, fname string) error {
	n, err := numEvents(fname)
	if err != nil {
		msg.Fatalf("could not assess number of events: %+v", err)
	}
	msg.Printf("input:  %s", fname)
	msg.Printf("events: %d", n)
	freq := int(n / 10)
	if freq == 0 {
		freq = 1
	}

	r, err := lcio.Open(fname)
	if err != nil {
		return fmt.Errorf("could not open LCIO file: %w", err)
	}
	defer r.Close()

	f, err := os.Create(oname)
	if err != nil {
		return fmt.Errorf("could not create output EDA file: %w", err)
	}
	defer f.Close()

	err = xcnv.LCIO2EDA(f, r, freq, msg)
	if err != nil {
		return fmt.Errorf("could not convert to EDA: %w", err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("could not close output EDA file: %w", err)
	}
	return nil
}
