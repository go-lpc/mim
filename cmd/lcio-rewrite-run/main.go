// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command lcio-rewrite-run reads a LCIO file and rewrites its run number
// with the provided value.
package main // import "github.com/go-lpc/mim/cmd/lcio-rewrite-run"

import (
	"compress/flate"
	"flag"
	"fmt"
	"io"
	"log"

	"go-hep.org/x/hep/lcio"
)

func main() {
	log.SetPrefix("lcio-rewrite: ")
	log.SetFlags(0)

	var (
		runnbr = flag.Int("run", 0, "run number to use for output LCIO file")
		oname  = flag.String("o", "out.lcio", "path to output rewritten LCIO file")
		compr  = flag.Int("compr", flate.DefaultCompression, "compression level to use for output file")
	)

	flag.Usage = func() {
		fmt.Printf(`Usage: lcio-rewrite-run [OPTIONS] FILE.lcio

ex:
 $> lcio-rewrite-run -o output.lcio -run=1234 ./input.lcio
 lcio-rewrite: processing event 0...
 lcio-rewrite: processing event 10...
 lcio-rewrite: processing event 20...
 lcio-rewrite: processing event 30...
 lcio-rewrite: processed 36 events

options:
`)
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		log.Fatalf("missing input LCIO file to rewrite")
	}

	r, err := lcio.Open(flag.Arg(0))
	if err != nil {
		log.Fatalf("could not open input LCIO file: %+v", err)
	}
	defer r.Close()

	w, err := lcio.Create(*oname)
	if err != nil {
		log.Fatalf("could not create output LCIO file: %+v", err)
	}
	defer w.Close()

	w.SetCompressionLevel(*compr)

	n, err := numEvents(flag.Arg(0))
	if err != nil {
		log.Fatalf("could not assess number of events: %+v", err)
	}
	log.Printf("input:  %s", flag.Arg(0))
	log.Printf("events: %d", n)

	err = process(w, r, int32(*runnbr), int(n/10))
	if err != nil {
		log.Fatalf("could not rewrite %q: %+v", flag.Arg(0), err)
	}

	err = w.Close()
	if err != nil {
		log.Fatalf("could not close output file: %+v", err)
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

func process(w *lcio.Writer, r *lcio.Reader, run int32, freq int) error {
	var (
		rhdr lcio.RunHeader
		i    = 0
	)
	for r.Next() {
		if i == 0 {
			rhdr = r.RunHeader()
			rhdr.RunNumber = run

			err := w.WriteRunHeader(&rhdr)
			if err != nil {
				return fmt.Errorf("could not write run header: %w", err)
			}

		}

		evt := r.Event()
		evt.RunNumber = run
		if i%freq == 0 {
			log.Printf("processing event %d...", evt.EventNumber)
		}
		err := w.WriteEvent(&evt)
		if err != nil {
			return fmt.Errorf("could not write evt %d: %w", evt.EventNumber, err)
		}
		i++
	}

	err := r.Err()
	if err != nil && err != io.EOF {
		return fmt.Errorf("could not read LCIO file: %w", err)
	}

	log.Printf("processed %d events", i)

	return nil
}
