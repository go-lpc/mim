// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package eformat describes and handles data in the DIF format.
package eformat // import "github.com/go-lpc/mim/internal/eformat"

const (
	//	scHeader  = 0xb1 // slow-control header marker
	//	scTrailer = 0xa1 // slow-control trailer marker

	gbHeader  = 0xb0 // global header marker
	gbHeaderB = 0xbb // global header marker (0xBB variant)
	gbTrailer = 0xa0 // global trailer marker

	frHeader  = 0xb4 // frame header marker
	frTrailer = 0xa3 // frame trailer marker

	anHeader = 0xc4 // analog frame header marker
	incFrame = 0xc3 // incomplete frame marker

)

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
