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
