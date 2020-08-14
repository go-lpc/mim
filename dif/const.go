// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dif

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
