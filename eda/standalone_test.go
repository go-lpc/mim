// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"os"
	"testing"

	"github.com/go-lpc/mim/eda/internal/regs"
)

func TestFailStandalone(t *testing.T) {
	const (
		cfgdir    = ""
		run       = 42
		threshold = 50
		rfmMask   = (1<<0 | 1<<1)
	)

	err := RunStandalone(
		cfgdir, run, threshold, rfmMask,
		WithDevSHM("/dev/null"),
	)

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestStandalone(t *testing.T) {
	fdev, err := newFakeDev()
	if err != nil {
		t.Fatalf("could not create fake-dev: %+v", err)
	}
	defer fdev.close()

	var (
		odir   = fdev.tmpdir
		cfgdir = "testdata"
	)

	srv, err := newStandalone(
		odir, fdev.mem, fdev.shm, cfgdir, 42,
		WithRFMMask(1<<1),
	)
	if err != nil {
		t.Fatalf("could not create standalone server: %+v", err)
	}

	const (
		rfmID   = 1
		rfmDone = regs.O_SC_DONE_1
	)

	// inject callback to automatically stop run when
	// fake registers ran out
	exhaust := func() {
		go func() {
			srv.stop <- os.Interrupt
		}()
	}

	fdev.fpga(srv.dev, rfmID, rfmDone, exhaust)

	err = srv.run()
	if err != nil {
		t.Fatalf("could run standalone server: %+v", err)
	}
}
