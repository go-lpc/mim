// Copyright 2021 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command dif-split splits a DIF/EDA binary file into n DIF files,
// one per DIF-ID.
package main // import "github.com/go-lpc/mim/cmd/dif-split"

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-lpc/mim/internal/eformat"
)

var (
	msg = log.New(os.Stdout, "dif-split: ", 0)
)

func main() {
	xmain(os.Args[1:])
}

func xmain(args []string) {
	var (
		fset = flag.NewFlagSet("dif", flag.ExitOnError)

		oname = fset.String("o", "out.raw", "path to output DIF file")
		eda   = fset.Bool("eda", false, "enable EDA hack")
	)

	fset.Usage = func() {
		fmt.Printf(`Usage: dif-split [OPTIONS] file.raw

ex:
 $> dif-split -o out.raw ./input.eda.raw

options:
`)
		fset.PrintDefaults()
	}

	err := fset.Parse(args)
	if err != nil {
		log.Fatalf("could not parse input arguments: %+v", err)
	}

	if fset.NArg() != 1 {
		fset.Usage()
		msg.Fatalf("missing input DIF raw file")
	}

	if *oname == "" {
		fset.Usage()
		msg.Fatalf("invalid output DIF raw file")
	}

	for _, arg := range fset.Args() {
		err := process(*oname, *eda, arg)
		if err != nil {
			msg.Fatalf("could not split DIF file %q: %+v", arg, err)
		}
	}
}

func process(oname string, isEDA bool, fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("could not open EDA file: %w", err)
	}
	defer f.Close()

	out := make(map[uint8]*eformat.Encoder)

	dec := eformat.NewDecoder(0, f)
	dec.IsEDA = isEDA

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

		enc, ok := out[d.Header.ID]
		if !ok {
			oid := outFileFrom(oname, d.Header.ID)
			msg.Printf("creating output file %q...", oid)
			o, err := os.Create(oid)
			if err != nil {
				return fmt.Errorf("could not create output file: %w", err)
			}
			defer o.Close()

			enc = eformat.NewEncoder(o)
			out[d.Header.ID] = enc
		}

		err = enc.Encode(&d)
		if err != nil {
			return fmt.Errorf("could not encode DIF: %w", err)
		}
	}

	return nil
}

func outFileFrom(fname string, id uint8) string {
	var (
		ext   = filepath.Ext(fname)
		oname = strings.Replace(fname, ext, fmt.Sprintf("-%03d%s", id, ext), 1)
	)
	return oname
}
