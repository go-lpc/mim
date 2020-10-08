// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command daq-boot (re)starts all the C++ DAQ processes.
package main // import "github.com/go-lpc/mim/cmd/daq-boot"

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/sbinet/pmon"
	"golang.org/x/sync/errgroup"
)

var (
	cmds = []*exec.Cmd{
		exec.Command("dns"),
		exec.Command("dimdb"),
		// exec.Command("dim-eda"),
		exec.Command("dimwriter"),
	}
	dir = os.Getenv("SDHCALLOGDIR")

	doMon  = flag.Bool("pmon", false, "enable pmon monitoring")
	doFreq = flag.Duration("freq", 1*time.Second, "pmon frequency")

	stop = make(chan os.Signal, 1)
)

func main() {
	flag.Parse()

	log.SetPrefix("daq-boot: ")
	log.SetFlags(0)

	err := run(*doMon, *doFreq, cmds, dir, stop)
	if err != nil {
		log.Fatalf("%+v", err)
	}
}

func run(doMon bool, freq time.Duration, cmds []*exec.Cmd, dir string, stop chan os.Signal) error {
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)

	for _, cmd := range cmds {
		name := filepath.Base(cmd.Path)
		kill := exec.Command("killall", name)
		kill.Stderr = os.Stderr
		kill.Stdout = os.Stdout
		err := kill.Run()
		if err != nil {
			log.Printf("could not kill %q: %+v", name, err)
		}
	}

	if dir == "" {
		dir = "/var/log/sdhcal"
	}

	var (
		grp  errgroup.Group
		kill = make(chan int)
	)
	grp.Go(func() error {
		return start(cmds[0], dir, kill, doMon, freq)
	})

	for i := range cmds[1:] {
		name := cmds[i+1]
		grp.Go(func() error {
			return start(name, dir, kill, doMon, freq)
		})
	}

	go func() {
		<-stop
		close(kill)
	}()

	err := grp.Wait()
	if err != nil {
		return fmt.Errorf("could not boot DAQ: %w", err)
	}
	return nil
}

func start(cmd *exec.Cmd, dir string, kill chan int, doMon bool, freq time.Duration) error {
	name := filepath.Base(cmd.Path)
	out, err := os.Create(filepath.Join(dir, name+".log"))
	if err != nil {
		return fmt.Errorf("could not create output log file for %q: %w", name, err)
	}
	defer out.Close()

	cmd.Stdout = out
	cmd.Stderr = out

	log.Printf("starting %q...", name)
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("could not start %q: %w", name, err)
	}

	if doMon {
		p, err := pmon.Monitor(cmd.Process.Pid)
		if err != nil {
			return fmt.Errorf("could not start monitoring %q (pid=%d): %w", name, cmd.Process.Pid, err)
		}
		f, err := os.Create(filepath.Join(dir, name+"-pmon.log"))
		if err != nil {
			return fmt.Errorf("could not create pmon log file for command %q: %w", name, err)
		}
		defer f.Close()
		p.W = f
		p.Freq = freq

		go func() {
			log.Printf("run pmon %q...", name)
			err := p.Run()
			if err != nil {
				log.Printf("could not start monitoring %q: %+v", name, err)
			}
		}()

		defer func() {
			err := p.Kill()
			if err != nil {
				log.Printf("could not stop monitoring %q: %+v", name, err)
			}
		}()
	}

	errch := make(chan error)
	go func() {
		errch <- cmd.Wait()
	}()

	select {
	case <-kill:
		err = cmd.Process.Kill()
		if err != nil {
			return fmt.Errorf("could not kill %q: %+v", name, err)
		}
	case err = <-errch:
		if err != nil {
			return fmt.Errorf("could not run %q: %w", name, err)
		}
	}

	return nil
}
