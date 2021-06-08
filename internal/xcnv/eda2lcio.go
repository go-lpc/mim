// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xcnv

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"unsafe"

	"github.com/go-lpc/mim/internal/eformat"
	"go-hep.org/x/hep/lcio"
)

func EDA2LCIO(w *lcio.Writer, dec *eformat.Decoder, run int32, msg *log.Logger) error {
	var (
		buf = new(bytes.Buffer)
		raw = &lcio.GenericObject{
			Data: []lcio.GenericObjectData{
				{I32s: nil},
			},
		}
	)

loop:
	for i := 0; ; i++ {
		if i%100 == 0 {
			msg.Printf("processing evt %d...", i)
		}
		var d eformat.DIF
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
						"Clock":   {200},
						"Trigger": {0},
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

	return nil
}

func i32sFrom(w *bytes.Buffer, d *eformat.DIF) []int32 {
	const i32sz = 4

	w.Reset()
	_, _ = w.Write(make([]byte, 6*i32sz))
	err := eformat.NewEncoder(w).Encode(d)
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
	ptr := (*int32)(unsafe.Pointer(&raw[0]))
	sli := unsafe.Slice(ptr, len(raw)/i32sz)

	return sli
}
