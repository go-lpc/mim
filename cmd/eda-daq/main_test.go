// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"net"
	"os"
	"testing"
)

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

	err = run(runID, threshold, rshaper, rfmMask, ":8877", ":8899",
		"outdir", devmem.Name(), devshm, "../../eda/testdata",
	)
	if err != nil {
		t.Fatalf("could not run eda-daq: %+v", err)
	}
}
