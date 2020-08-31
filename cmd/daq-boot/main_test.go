// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	dir, err := ioutil.TempDir("", "daq-boot-")
	if err != nil {
		t.Fatalf("could not create tmpdir: %+v", err)
	}
	defer os.RemoveAll(dir)

	cmds := make([]string, 3)
	for i := range cmds {
		cpu := filepath.Join(dir, "run-cpu-"+strconv.Itoa(i))
		cmds[i] = cpu

		cmd := exec.Command("go", "build", "-o", cpu, "github.com/sbinet/pmon/_examples/run-cpu")
		err = cmd.Run()
		if err != nil {
			t.Fatalf("could not create test program: %+v", err)
		}
	}

	for _, tc := range []struct {
		name string
		cmds []*exec.Cmd
		mon  bool
		stop bool
	}{
		{
			name: "simple",
			cmds: []*exec.Cmd{
				exec.Command(cmds[0], "-timeout=5s"),
				exec.Command(cmds[1], "-timeout=5s"),
				exec.Command(cmds[2], "-timeout=5s"),
			},
		},
		{
			name: "simple-pmon",
			cmds: []*exec.Cmd{
				exec.Command(cmds[0], "-timeout=5s"),
				exec.Command(cmds[1], "-timeout=5s"),
				exec.Command(cmds[2], "-timeout=5s"),
			},
			mon: true,
		},
		{
			name: "simple-stop",
			cmds: []*exec.Cmd{
				exec.Command(cmds[0], "-timeout=10s"),
				exec.Command(cmds[1], "-timeout=10s"),
				exec.Command(cmds[2], "-timeout=10s"),
			},
			stop: true,
		},
		{
			name: "simple-stop-pmon",
			cmds: []*exec.Cmd{
				exec.Command(cmds[0], "-timeout=10s"),
				exec.Command(cmds[1], "-timeout=10s"),
				exec.Command(cmds[2], "-timeout=10s"),
			},
			stop: true,
			mon:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "daq-boot-")
			if err != nil {
				t.Fatalf("could not create tmpdir: %+v", err)
			}
			defer os.RemoveAll(dir)

			stop := make(chan os.Signal, 1)
			if tc.stop {
				go func() {
					time.Sleep(5 * time.Second)
					stop <- os.Interrupt
				}()
			}
			err = run(tc.mon, 1*time.Second, tc.cmds, dir, stop)
			if err != nil {
				t.Fatalf("could not run processes: %+v", err)
			}
		})
	}
}
