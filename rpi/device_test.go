// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpi

import (
	"fmt"
	"io"
	"testing"
)

func TestFTDIOpen(t *testing.T) {
	dev, err := ftdiOpenImpl(0, 0)
	if err == nil {
		_ = dev.Close()
	}
}

type ierr struct {
	n int
	e error
}

type failingRW struct {
	rs []ierr
	ws []ierr
}

func (rw *failingRW) Read(p []byte) (int, error) {
	i := len(rw.rs)
	rs := rw.rs[i-1]
	rw.rs = rw.rs[:i-1]
	return rs.n, rs.e
}

func (rw *failingRW) Write(p []byte) (int, error) {
	i := len(rw.ws)
	ws := rw.ws[i-1]
	rw.ws = rw.ws[:i-1]
	return ws.n, ws.e
}

func TestDevice(t *testing.T) {
	ftdiOpen = ftdiOpenTest
	defer func() {
		ftdiOpen = ftdiOpenImpl
	}()

	dev, err := newDevice(0x1, 0x2)
	if err != nil {
		t.Fatalf("could not create fake device: %+v", err)
	}
	defer dev.close()

	var (
		rw failingRW
		ft = fakeDevice{buf: &rw}
	)
	dev.ft = &ft

	for _, tc := range []struct {
		name string
		f    func() error
		want error
	}{
		{
			name: "usbRegRead-eof",
			f: func() error {
				rw.ws = append(rw.ws, ierr{0, io.EOF})
				_, err := dev.usbRegRead(0x1234)
				return err
			},
			want: fmt.Errorf("could not write USB addr 0x%x: %w", 0x1234, io.EOF),
		},
		{
			name: "usbRegRead-short-write",
			f: func() error {
				rw.ws = append(rw.ws, ierr{1, nil})
				_, err := dev.usbRegRead(0x1234)
				return err
			},
			want: fmt.Errorf("could not write USB addr 0x%x: %w", 0x1234, io.ErrShortWrite),
		},
		{
			name: "usbRegRead-err-read",
			f: func() error {
				rw.ws = append(rw.ws, ierr{2, nil})
				rw.rs = append(rw.rs, ierr{2, io.ErrUnexpectedEOF})
				_, err := dev.usbRegRead(0x1234)
				return err
			},
			want: fmt.Errorf("could not read register 0x%x: %w", 0x1234, io.ErrUnexpectedEOF),
		},
		{
			name: "usbCmdWrite-eof",
			f: func() error {
				rw.ws = append(rw.ws, ierr{0, io.EOF})
				return dev.usbCmdWrite(0x1234)
			},
			want: fmt.Errorf("could not write USB command 0x%x: %w", 0x1234, io.EOF),
		},
		{
			name: "usbCmdWrite-short-write",
			f: func() error {
				rw.ws = append(rw.ws, ierr{1, nil})
				return dev.usbCmdWrite(0x1234)
			},
			want: fmt.Errorf("could not write USB command 0x%x: %w", 0x1234, io.ErrShortWrite),
		},
		{
			name: "usbRegWrite-eof",
			f: func() error {
				rw.ws = append(rw.ws, ierr{0, io.EOF})
				return dev.usbRegWrite(0x1234, 0x255)
			},
			want: fmt.Errorf("could not write USB register (0x%x, 0x%x): %w", 0x1234, 0x255, io.EOF),
		},
		{
			name: "usbRegWrite-short-write",
			f: func() error {
				rw.ws = append(rw.ws, ierr{1, nil})
				return dev.usbRegWrite(0x1234, 0x255)
			},
			want: fmt.Errorf("could not write USB register (0x%x, 0x%x): %w", 0x1234, 0x255, io.ErrShortWrite),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.f()
			switch {
			case got == nil && tc.want == nil:
				// ok
			case got == nil && tc.want != nil:
				t.Fatalf("got=%v, want=%v", got, tc.want)
			case got != nil && tc.want != nil:
				if got, want := got.Error(), tc.want.Error(); got != want {
					t.Fatalf("got= %v\nwant=%v", got, want)
				}
			case got != nil && tc.want == nil:
				t.Fatalf("got=%+v\nwant=%v", got, tc.want)
			}
		})
	}
}
