// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xcnv

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"unsafe"

	"github.com/go-lpc/mim/internal/eformat"
	"go-hep.org/x/hep/lcio"
)

func LCIO2EDA(w io.Writer, r *lcio.Reader, freq int, msg *log.Logger) error {
	var (
		enc = eformat.NewEncoder(w)
		i   = 0
	)

	for r.Next() {
		if i%freq == 0 {
			msg.Printf("processing evt %d...", i)
		}
		evt := r.Event()
		raw := evt.Get("RU_XDAQ").(*lcio.GenericObject).Data[0].I32s
		buf := bytesFromI32s(raw[6:])
		dec := eformat.NewDecoder(buf[1], bytes.NewReader(buf))
		dec.IsEDA = true

		var d eformat.DIF
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

	return nil
}

func bytesFromI32s(raw []int32) []byte {
	n := len(raw)
	if n == 0 {
		return nil
	}
	const i32sz = 4
	ptr := (*byte)(unsafe.Pointer(&raw[0]))
	sli := unsafe.Slice(ptr, i32sz*n)
	return sli
}
