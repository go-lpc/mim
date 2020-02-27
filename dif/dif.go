// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package dif holds functions to manipulate data from DIFs.
package dif // import "github.com/go-lpc/mim/dif"

// DIF represents a detector interface.
type DIF struct {
	Header GlobalHeader
	Frames []Frame
}

type GlobalHeader struct {
	ID        uint8
	DTC       uint32 // DIF trigger counter
	ATC       uint32 // Acquisition trigger counter
	GTC       uint32 // Global trigger counter
	AbsBCID   uint64 // Absolute BCID
	TimeDIFTC uint32 // Time DIF trigger counter
}

type Frame struct {
	Header uint8 // Hardroc header
	BCID   uint32
	Data   [16]uint8
}

type File struct {
	Version uint8
	Headers []SCHeader
}

type SCHeader struct {
	Timestamp uint32 // epoch
	DIFID     uint8
	NumHRs    uint8 // number of Hardrocs
	HRs       [][72]uint8
}
