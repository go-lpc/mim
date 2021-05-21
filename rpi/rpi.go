// Copyright 2021 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package rpi holds functions to manipulate data from RPi.
package rpi // import "github.com/go-lpc/mim/rpi"

import (
	"fmt"
	"strings"
)

type DbInfo struct {
	ID       uint32
	NumASICs uint32
	Slow     [][]byte // [MaxNumASICs][hardrocV2SLCFrameSize]byte
}

func slcStatus(slc uint32) (string, bool) {
	var (
		o  = new(strings.Builder)
		ok = true
	)
	switch {
	case slc&0x0003 == 0x01:
		fmt.Fprintf(o, "SLC CRC OK     - ")
	case slc&0x0003 == 0x02:
		fmt.Fprintf(o, "SLC CRC Failed - ")
		ok = false
	default:
		fmt.Fprintf(o, "SLC CRC forb   - ")
		ok = false
	}

	switch {
	case slc&0x000c == 0x04:
		fmt.Fprintf(o, "All OK     - ")
	case slc&0x000c == 0x08:
		fmt.Fprintf(o, "All Failed - ")
		ok = false
	default:
		fmt.Fprintf(o, "All forb   - ")
		ok = false
	}

	switch {
	case slc&0x0030 == 0x10:
		fmt.Fprintf(o, "L1 OK     - ")
	case slc&0x0030 == 0x20:
		fmt.Fprintf(o, "L1 Failed - ")
		ok = false
	default:
		fmt.Fprintf(o, "L1 forb   - ")
		ok = false
	}

	return o.String(), ok
}
