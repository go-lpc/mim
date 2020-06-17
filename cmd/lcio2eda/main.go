// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command lcio2eda converts a LCIO file into an EDA raw data file.
package main // import "github.com/go-lpc/mim/cmd/lcio2eda"

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"unsafe"

	"github.com/go-lpc/mim/dif"
	"go-hep.org/x/hep/lcio"
)

func main() {
	log.SetPrefix("lcio2eda: ")
	log.SetFlags(0)

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
		log.Fatalf("missing input LCIO file")
	}

	if *oname == "" {
		flag.Usage()
		log.Fatalf("invalid output EDA file name")
	}

	n, err := numEvents(flag.Arg(0))
	if err != nil {
		log.Fatalf("could not assess number of events: %+v", err)
	}
	log.Printf("input:  %s", flag.Arg(0))
	log.Printf("events: %d", n)

	err = process(*oname, flag.Arg(0), int(n/10))
	if err != nil {
		log.Fatalf("could not convert LCIO file: %+v", err)
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

func process(oname, fname string, freq int) error {
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

	var (
		enc = dif.NewEncoder(f)
		i   = 0
	)
	for r.Next() {
		if i%freq == 0 {
			log.Printf("processing evt %d...", i)
		}
		evt := r.Event()
		raw := evt.Get("RU_XDAQ").(*lcio.GenericObject).Data[0].I32s
		buf := bytesFrom(raw[6:])
		dec := dif.NewDecoder(buf[1], bytes.NewReader(buf))

		var d dif.DIF
		err := dec.Decode(&d)
		if err != nil {
			return fmt.Errorf("could not decode EDA: %w", err)
		}
		err = enc.Encode(&d)
		if err != nil {
			return fmt.Errorf("could not re-encode EDA: %w", err)
		}
		i++
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("could not close output EDA file: %w", err)
	}
	return nil
}

func bytesFrom(raw []int32) []byte {
	hdr := *(*reflect.SliceHeader)(unsafe.Pointer(&raw))
	hdr.Len *= 4
	hdr.Cap *= 4

	return *(*[]byte)(unsafe.Pointer(&hdr))
}
