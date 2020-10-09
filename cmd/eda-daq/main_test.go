// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"
)

func TestXMain(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want error
	}{
		{
			args: []string{"-=3"},
			want: fmt.Errorf("could not parse input arguments: bad flag syntax: -=3"),
		},
		{
			args: []string{"-run=-1", "-thresh=10", "-rshaper=3", "-rfm=3"},
			want: fmt.Errorf("invalid run number value (=-1)"),
		},
		{
			args: []string{"-run=42", "-thresh=-1", "-rshaper=3", "-rfm=3"},
			want: fmt.Errorf("invalid threshold value (=-1)"),
		},
		{
			args: []string{"-run=42", "-thresh=10", "-rshaper=-1", "-rfm=3"},
			want: fmt.Errorf("invalid R-shaper value (=-1)"),
		},
		{
			args: []string{"-run=42", "-thresh=10", "-rshaper=3", "-rfm=-1"},
			want: fmt.Errorf("invalid RFM mask value (=-1)"),
		},
		{
			args: []string{"-run=42", "-thresh=10", "-rshaper=3", "-rfm=1"},
			want: fmt.Errorf("could not run eda-daq: could not dial eda-srv \":8877\": dial tcp :8877: connect: connection refused"),
		},
	} {
		t.Run("", func(t *testing.T) {
			got := xmain(tc.args)
			switch {
			case got == nil && tc.want == nil:
				// ok.
			case got != nil && tc.want == nil:
				t.Fatalf("could not run eda-daq: %+v", got)
			case got == nil && tc.want != nil:
				t.Fatalf("expected an error (%v)", tc.want.Error())
			case got != nil && tc.want != nil:
				if got, want := got.Error(), tc.want.Error(); got != want {
					t.Fatalf("invalid error:\ngot= %q\nwant=%q\n", got, want)
				}
			}
		})
	}
}

func TestRun(t *testing.T) {
	t.Skip() // FIXME(sbinet)

	lis, err := net.Listen("tcp", ":8877")
	if err != nil {
		t.Fatalf("could not create fake eda-srv server: %+v", err)
	}
	defer lis.Close()
	go func() {
		conn, err := lis.Accept()
		if err != nil {
			t.Errorf("could not accept conn: %+v", err)
		}
		defer conn.Close()
	}()

	devshm, err := ioutil.TempDir("", "eda-daq-")
	if err != nil {
		t.Fatalf("could not create fake dev-shm: %+v", err)
	}
	defer os.RemoveAll(devshm)

	devmem, err := ioutil.TempFile("", "eda-daq-")
	if err != nil {
		t.Fatalf("could not create fake dev-mem: %+v", err)
	}
	defer devmem.Close()

	const (
		devmemSize = 4281335808 // regs.LW_H2F_BASE+regs.LW_H2F_SPAN
	)
	_, err = devmem.WriteAt([]byte{1}, devmemSize)
	if err != nil {
		t.Fatalf("could not write to dev-mem: %+v", err)
	}
	err = devmem.Close()
	if err != nil {
		t.Fatalf("could not close devmem: %+v", err)
	}

	const (
		runID     = 42
		threshold = 126
		rshaper   = 3
		rfmMask   = 1
	)

	err = run(runID, threshold, rshaper, rfmMask, ":8877",
		"outdir", devmem.Name(), devshm, "../../eda/testdata",
	)
	if err != nil {
		t.Fatalf("could not run eda-daq: %+v", err)
	}
}
