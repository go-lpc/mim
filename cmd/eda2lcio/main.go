// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command eda2lcio converts an EDA raw data file to an LCIO one.
package main // import "github.com/go-lpc/mim/cmd/eda2lcio"

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"unsafe"

	"github.com/go-lpc/mim/dif"
	"go-hep.org/x/hep/lcio"
)

func main() {
	log.SetPrefix("eda2lcio: ")
	log.SetFlags(0)

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
		log.Fatalf("missing input EDA raw file")
	}

	if *oname == "" {
		flag.Usage()
		log.Fatalf("invalid output LCIO file name")
	}

	err := process(*oname, *compr, flag.Arg(0))
	if err != nil {
		log.Fatalf("could not convert EDA file: %+v", err)
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

	var (
		buf = new(bytes.Buffer)
		raw = &lcio.GenericObject{
			Data: []lcio.GenericObjectData{
				{I32s: nil},
			},
		}
		dec = dif.NewDecoder(edaIDFrom(f), f)
	)

loop:
	for i := 0; ; i++ {
		if i%100 == 0 {
			log.Printf("processing evt %d...", i)
		}
		var d dif.DIF
		err := dec.Decode(&d)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break loop
			}
			return fmt.Errorf("could not decode EDA: %w", err)
		}

		if i == 0 {
			err = w.WriteRunHeader(&lcio.RunHeader{
				RunNumber: run,
				Detector:  "SD-HCAL",
				Descr:     "",
				Params: lcio.Params{
					Ints: map[string][]int32{
						"Clock":   []int32{200},
						"Trigger": []int32{0},
					},
				},
			})
			if err != nil {
				return fmt.Errorf("could not write run header: %w", err)
			}
		}

		evt := lcio.Event{
			RunNumber:   run,
			EventNumber: int32(i),
			TimeStamp:   int64(d.Header.AbsBCID),
			Detector:    "SD-HCAL",
		}
		raw.Data[0].I32s = i32sFrom(buf, &d)
		evt.Add("RU_XDAQ", raw)

		err = w.WriteEvent(&evt)
		if err != nil {
			return fmt.Errorf("could not write EDA event: %w", err)
		}
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

func i32sFrom(w *bytes.Buffer, d *dif.DIF) []int32 {
	const i32sz = 4

	w.Reset()
	_, _ = w.Write(make([]byte, 6*i32sz))
	err := dif.NewEncoder(w).Encode(d)
	if err != nil {
		panic(err)
	}

	mod := i32sz - (len(w.Bytes()) % i32sz)
	if mod != 0 {
		// align to an even number of int32s
		_, _ = w.Write(make([]byte, mod))
	}

	raw := w.Bytes()
	// we use LittleEndian here b/c that's what is in effect done
	// in the C++ DAQ (via a shm on an AMD64 machine).
	binary.LittleEndian.PutUint32(raw[0*i32sz:], 0xcafe) // FIXME(sbinet): ???
	binary.LittleEndian.PutUint32(raw[1*i32sz:], d.Header.DTC)
	binary.LittleEndian.PutUint32(raw[2*i32sz:], d.Header.DTC)
	binary.LittleEndian.PutUint32(raw[3*i32sz:], d.Header.GTC)
	binary.LittleEndian.PutUint32(raw[4*i32sz:], uint32(d.Header.ID))
	binary.LittleEndian.PutUint32(raw[5*i32sz:], uint32(len(raw)))

	// FIXME(sbinet): use a special io.Writer that writes to
	// a []int32 instead of this unsafe business ?
	hdr := *(*reflect.SliceHeader)(unsafe.Pointer(&raw))
	hdr.Len /= 4
	hdr.Cap /= 4

	data := *(*[]int32)(unsafe.Pointer(&hdr))
	return data
}
