// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

// server allows to control an EDA board device.
type server struct {
	ctl net.Listener

	msg    *log.Logger
	odir   string
	devmem string
	devshm string
	cfgdir string
}

func Serve(addr, odir, devmem, devshm, cfgdir string) error {
	ctl, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("could not create eda-ctl server on %q: %w", addr, err)
	}

	srv := &server{
		ctl: ctl,

		msg: log.New(os.Stdout, "eda-svc: ", 0),

		odir:   odir,
		devmem: devmem,
		devshm: devshm,
		cfgdir: cfgdir,
	}
	return srv.serve()
}

func (srv *server) serve() error {
	defer srv.close()

	for {
		conn, err := srv.ctl.Accept()
		if err != nil {
			return fmt.Errorf("could not accept connection: %w", err)
		}

		err = srv.handle(conn)
		if err != nil {
			srv.msg.Printf("could not run EDA board: %+v", err)
			continue
		}
	}
}

func (srv *server) handle(conn net.Conn) error {
	defer conn.Close()
	srv.msg.Printf("serving %v...", conn.RemoteAddr())
	defer srv.msg.Printf("serving %v... [done]", conn.RemoteAddr())

	dev, err := newDevice(srv.devmem, srv.odir, srv.devshm, srv.cfgdir)
	if err != nil {
		return fmt.Errorf("could not create EDA device: %w", err)
	}
	defer dev.Close()

	// FIXME(sbinet): use DIM hooks to configure those
	dev.id = 1
	dev.rfms = []int{0, 1}
	dev.cfg.daq.delta = 180
	dev.cfg.hr.rshaper = 3
	dim, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return fmt.Errorf("could not extract dim-host ip from %q: %+v", conn.RemoteAddr().String(), err)
	}
	//  dev.cfg.daq.addrs = make([]string, len(dev.rfms))
	//	for i, rfm := range dev.rfms {
	//		difid := difIDFrom(dev.id, i)
	//		dev.cfg.daq.addrs[i] = fmt.Sprintf("%s:%d", dim, 10000+difid)
	//	}

loop:
	for {
		var req struct {
			Name string   `json:"name"`
			Args []string `json:"args"`
		}

		err = json.NewDecoder(conn).Decode(&req)
		if err != nil {
			srv.msg.Printf("could not decode command request: %+v", err)
			srv.reply(conn, err)
			if errors.Is(err, io.EOF) {
				break loop
			}
			continue
		}
		dev.msg.Printf("received request: name=%q, args=%v", req.Name, req.Args)

		switch strings.ToLower(req.Name) {
		case "configure":
			difid, err := strconv.Atoi(req.Args[0])
			if err != nil {
				srv.msg.Printf("could not decode difid to configure (args=%v): %+v",
					req.Args, err,
				)
				srv.reply(conn, err)
				continue
			}
			// FIXME(sbinet): handle hysteresis, make sure addrs are unique.
			dev.cfg.daq.addrs = append(dev.cfg.daq.addrs, fmt.Sprintf(
				"%s:%d", dim, 10000+difid,
			))
			srv.msg.Printf("addrs: %q", dev.cfg.daq.addrs)

			err = dev.Configure()
			srv.reply(conn, err)
			if err != nil {
				srv.msg.Printf("could not configure EDA device: %+v", err)
				continue
			}

		case "initialize":
			err = dev.Initialize()
			srv.reply(conn, err)
			if err != nil {
				srv.msg.Printf("could not initialize EDA device: %+v", err)
				continue
			}

		case "start":
			run, err := strconv.Atoi(req.Args[0])
			if err != nil {
				srv.msg.Printf("could not decode run-nbr for start-run (args=%v): %+v",
					req.Args, err,
				)
				srv.reply(conn, err)
				continue
			}
			dev.run = uint32(run)

			err = dev.Start()
			srv.reply(conn, err)
			if err != nil {
				srv.msg.Printf("could not start EDA device: %+v", err)
				continue
			}

		case "stop":
			err = dev.Stop()
			srv.reply(conn, err)
			if err != nil {
				srv.msg.Printf("could not stop EDA device: %+v", err)
				return fmt.Errorf("could not stop EDA device: %w", err)
			}
			break loop

		default:
			srv.msg.Printf("unknown command name=%q, args=%q", req.Name, req.Args)
			continue
		}
	}

	return nil
}

func (srv *server) reply(conn net.Conn, err error) {
	rep := struct {
		Msg string `json:"msg"`
	}{"ok"}
	if err != nil {
		rep.Msg = fmt.Sprintf("%+v", err)
	}

	_ = json.NewEncoder(conn).Encode(rep)
}

func (srv *server) close() {
	_ = srv.ctl.Close()
}
