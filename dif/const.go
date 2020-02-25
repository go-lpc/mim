// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dif

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

const (
	MaxEventSize = (hardrocV2SLCFrameSize+1)*MaxNumASICs + (20*ASICMemDepth+2)*MaxNumASICs + 3 + MaxFwHeaderSize + 2 + MaxAnalogDataSize + 50

	MaxAnalogDataSize = 1024*64*2 + 20
	MaxFwHeaderSize   = 50
	MaxNumASICs       = 48  // max number of hardrocs per dif that the system can handle
	MaxNumDIFs        = 200 // max number of difs that the system can handle
	ASICMemDepth      = 128 // memory depth of one asic . 128 is for hardroc v1

	hardrocV2SLCFrameSize = 109
	microrocSLCFrameSize  = 74
)
