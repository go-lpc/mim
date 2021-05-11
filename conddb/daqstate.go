// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conddb

type DAQState struct {
	ID          uint64
	HRConfig    int32
	RShape      uint16
	TriggerMode uint16
}

type RFM struct {
	ID   int `json:"rfm"`
	EDA  int `json:"eda"`
	Slot int `json:"slot"`
	DAQ  struct {
		RShaper     int `json:"rshaper"`
		TriggerMode int `json:"trigger_type"`
	} `json:"daq_state"`
}
