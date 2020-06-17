// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command eda2lcio converts an EDA raw data file to an LCIO one.
package main // import "github.com/go-lpc/mim/cmd/eda2lcio"

import (
	"compress/flate"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/go-lpc/mim/dif"
	"github.com/go-lpc/mim/internal/xcnv"
	"go-hep.org/x/hep/lcio"
)

var (
	msg = log.New(os.Stdout, "eda2lcio: ", 0)
)

func main() {
	var (
		oname = flag.String("o", "out.lcio", "path to output LCIO file")
		compr = flag.Int("lvl", flate.DefaultCompression, "compression level for output LCIO file")
	)

	flag.Usage = func() {
		fmt.Printf(`Usage: eda2lcio [OPTIONS] file.raw

ex:
 $> eda2lcio -o out.lcio -lvl=9 ./input.eda.raw

options:
`)
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		msg.Fatalf("missing input EDA raw file")
	}

	if *oname == "" {
		flag.Usage()
		msg.Fatalf("invalid output LCIO file name")
	}

	err := process(*oname, *compr, flag.Arg(0))
	if err != nil {
		msg.Fatalf("could not convert EDA file: %+v", err)
	}
}

func process(oname string, lvl int, fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("could not open EDA file: %w", err)
	}
	defer f.Close()

	run, err := runNbrFrom(fname)
	if err != nil {
		return fmt.Errorf("could not infer run from %q: %w", fname, err)
	}

	w, err := lcio.Create(oname)
	if err != nil {
		return fmt.Errorf("could not create output LCIO file: %w", err)
	}
	defer w.Close()

	w.SetCompressionLevel(lvl)

	dec := dif.NewDecoder(edaIDFrom(f), f)
	err = xcnv.EDA2LCIO(w, dec, run, msg)
	if err != nil {
		return fmt.Errorf("could not convert EDA to LCIO: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("could not close output LCIO file: %w", err)
	}

	return nil
}

func edaIDFrom(f io.ReaderAt) uint8 {
	p := []byte{0}
	_, err := f.ReadAt(p, 1)
	if err != nil {
		panic(err)
	}
	return uint8(p[0])
}

func runNbrFrom(fname string) (int32, error) {
	var (
		name = filepath.Base(fname)
		run  int32
		itr  int32
	)
	_, err := fmt.Sscanf(name, "eda_%d.%d.raw", &run, &itr)
	return run, err
}
