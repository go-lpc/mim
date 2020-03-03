// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dif

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/go-daq/tdaq/log"
	"github.com/ziutek/ftdi"
)

func ftdiOpenTest(vid, pid uint16) (ftdiDevice, error) {
	return &fakeDevice{buf: new(bytes.Buffer)}, nil
}

func TestReadout(t *testing.T) {
	ftdiOpen = ftdiOpenTest
	defer func() {
		ftdiOpen = ftdiOpenImpl
	}()

	for _, tc := range []struct {
		name string
		err  error
		id   uint32
	}{
		{
			name: "FT101xxx",
			err:  fmt.Errorf("could not find DIF-id from %q: %s", "FT101xxx", errors.New("expected integer")),
		},
		{
			name: "FT101",
			err:  fmt.Errorf("could not find DIF-id from %q: %s", "FT101", io.EOF),
		},
		{
			name: "FT10142",
			id:   42,
		},
		{
			name: "FT101042",
			id:   42,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rdo, err := NewReadout(tc.name, 0x6014, nil)
			if err == nil && tc.err != nil {
				rdo.close()
				t.Fatalf("expected an error")
			}
			switch {
			case tc.err != nil:
				if got, want := err.Error(), tc.err.Error(); got != want {
					t.Fatalf("invalid error:\ngot= %v\nwant=%v", got, want)
				}
			default:
				defer rdo.close()
				if rdo.difID != tc.id {
					t.Fatalf("invalid DIF-id: got=%d, want=%d", rdo.difID, tc.id)
				}
			}
		})
	}

	const (
		name   = "FT101042"
		prodID = 0x6014
	)

	rdo, err := NewReadout(name, prodID, log.NewMsgStream("readout-"+name, log.LvlDebug, os.Stderr))
	if err != nil {
		t.Fatalf("could not create readout: %+v", err)
	}
	if got, want := rdo.difID, uint32(42); got != want {
		t.Fatalf("invalid DIF-ID: got=%d, want=%d", got, want)
	}

	err = rdo.configureRegisters()
	if err != nil {
		t.Fatalf("could not configure registers: %+v", err)
	}

	slow := make([][]byte, rdo.nasics)
	for i := range slow {
		slow[i] = make([]byte, hardrocV2SLCFrameSize)
	}
	_, err = rdo.configureChips(slow)
	if err != nil {
		t.Fatalf("could not configure chips: %+v", err)
	}

	err = rdo.start()
	if err != nil {
		t.Fatalf("could not start readout: %+v", err)
	}

	data := make([]byte, MaxEventSize)
	{
		w := new(bytes.Buffer)
		dif := DIF{
			Header: GlobalHeader{
				ID:        uint8(rdo.difID),
				DTC:       10,
				ATC:       11,
				GTC:       12,
				AbsBCID:   0x0000112233445566,
				TimeDIFTC: 0x00112233,
			},
			Frames: []Frame{
				{
					Header: 1,
					BCID:   0x001a1b1c,
					Data:   [16]uint8{0xa, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
				},
				{
					Header: 2,
					BCID:   0x002a2b2c,
					Data: [16]uint8{
						0xb, 21, 22, 23, 24, 25, 26, 27, 28, 29,
						210, 211, 212, 213, 214, 215,
					},
				},
			},
		}
		err = NewEncoder(w).Encode(&dif)
		if err != nil {
			t.Fatalf("could not encode DIF data: %+v", err)
		}
		rdo.dev.ft = &fakeDevice{w}
	}
	n, err := rdo.Readout(data)
	if err != nil {
		t.Fatalf("could not readout data: %+v", err)
	}
	if n <= 0 {
		t.Fatalf("could not readout data: n=%d", n)
	}
	data = data[:n]

	err = rdo.stop()
	if err != nil {
		t.Fatalf("could not stop readout: %+v", err)
	}

	err = rdo.close()
	if err != nil {
		t.Fatalf("could not close readout: %+v", err)
	}
}

func TestInvalidReadout(t *testing.T) {
	want := fmt.Errorf("no such device")
	ftdiOpen = func(vid, pid uint16) (ftdiDevice, error) { return nil, want }
	defer func() {
		ftdiOpen = ftdiOpenImpl
	}()

	rdo, err := NewReadout("FT101042", 0x6014, nil)
	if err == nil {
		_ = rdo.close()
		t.Fatalf("expected an error, got=%v", err)
	}
	if got, want := err.Error(), want.Error(); !strings.Contains(got, want) {
		t.Fatalf("invalid error.\ngot= %v\nwant=%v\n", got, want)
	}
}

type fakeDevice struct {
	buf io.ReadWriter
}

func (dev *fakeDevice) Reset() error { return nil }

func (dev *fakeDevice) SetBitmode(iomask byte, mode ftdi.Mode) error {
	return nil
}

func (dev *fakeDevice) SetFlowControl(flowctrl ftdi.FlowCtrl) error {
	return nil
}

func (dev *fakeDevice) SetLatencyTimer(lt int) error {
	return nil
}

func (dev *fakeDevice) SetWriteChunkSize(cs int) error {
	return nil
}

func (dev *fakeDevice) SetReadChunkSize(cs int) error {
	return nil
}

func (dev *fakeDevice) PurgeBuffers() error {
	return nil
}

func (dev *fakeDevice) Read(p []byte) (int, error) {
	return dev.buf.Read(p)
}

func (dev *fakeDevice) Write(p []byte) (int, error) {
	return dev.buf.Write(p)
}

func (dev *fakeDevice) Close() error { return nil }
