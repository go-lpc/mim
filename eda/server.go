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

	"github.com/go-lpc/mim/conddb"
)

// server allows to control an EDA board device.
type server struct {
	ctl net.Listener

	msg    *log.Logger
	odir   string
	devmem string
	devshm string

	newDevice func(devmem, odir, devshm string, opts ...Option) (device, error)

	opts []Option
	dev  device
}

func Serve(addr, odir, devmem, devshm string, opts ...Option) error {
	srv, err := newServer(addr, odir, devmem, devshm, opts...)
	if err != nil {
		return fmt.Errorf("could not create eda server: %w", err)
	}
	return srv.serve()
}

func newServer(addr, odir, devmem, devshm string, opts ...Option) (*server, error) {
	ctl, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("could not create eda-ctl server on %q: %w", addr, err)
	}

	srv := &server{
		ctl: ctl,

		msg: log.New(os.Stdout, "eda-svc: ", 0),

		odir:   odir,
		devmem: devmem,
		devshm: devshm,

		newDevice: func(devmem, odir, devshm string, opts ...Option) (device, error) {
			return newCDevice(devmem, odir, devshm, opts...)
		},

		opts: opts,
	}
	return srv, nil
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

	srv.dev = nil
	dev, err := srv.newDevice(
		srv.devmem, srv.odir, srv.devshm,
		srv.opts...,
	)
	if err != nil {
		return fmt.Errorf("could not create EDA device: %w", err)
	}
	defer dev.Close()
	srv.dev = dev

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
			Name string           `json:"name"`
			Args *json.RawMessage `json:"args"`
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
		srv.msg.Printf("received request: name=%q", req.Name)

		switch strings.ToLower(req.Name) {
		case "scan":
			var args []conddb.RFM
			err = json.Unmarshal(*req.Args, &args)
			if err != nil {
				srv.msg.Printf("could not decode %q payload: %+v",
					req.Name, err,
				)
				srv.reply(conn, err)
				continue
			}

			err = dev.Boot(args)
			if err != nil {
				srv.msg.Printf("could not bootstrap EDA: %+v", err)
				srv.reply(conn, err)
				continue
			}

			srv.reply(conn, err)
			// FIXME(sbinet): compare expected scan-result with
			// EDA introspection functions.
			// if err != nil {
			// 	srv.msg.Printf("could not scan EDA device: %+v", err)
			// 	continue
			// }

		case "configure":
			var args []struct {
				DIF   uint8         `json:"dif"`
				ASICs []conddb.ASIC `json:"asics"`
			}
			err = json.Unmarshal(*req.Args, &args)
			if err != nil {
				srv.msg.Printf("could not decode %q payload: %+v",
					req.Name, err,
				)
				srv.reply(conn, err)
				continue
			}

			for _, arg := range args {
				addr := fmt.Sprintf("%s:%d", dim, 10000+int(arg.DIF))
				srv.msg.Printf("configuring DIF=%d with addr=%q", arg.DIF, addr)
				err := dev.ConfigureDIF(addr, arg.DIF, arg.ASICs)
				if err != nil {
					srv.msg.Printf("could not configure EDA device(dif=%d): %+v", arg.DIF, err)
					srv.reply(conn, err)
					continue
				}
			}
			srv.reply(conn, nil)

		case "initialize":
			err = dev.Initialize()
			srv.reply(conn, err)
			if err != nil {
				srv.msg.Printf("could not initialize EDA device: %+v", err)
				continue
			}

		case "start":
			var args []string
			err = json.Unmarshal(*req.Args, &args)
			if err != nil {
				srv.msg.Printf("could not decode %q payload: %+v",
					req.Name, err,
				)
				srv.reply(conn, err)
				continue
			}

			run, err := strconv.Atoi(args[0])
			if err != nil {
				srv.msg.Printf("could not decode run-nbr for start-run (args=%v): %+v",
					req.Args, err,
				)
				srv.reply(conn, err)
				continue
			}

			err = dev.Start(uint32(run))
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
			err = fmt.Errorf("unknown command %q", req.Name)
			srv.reply(conn, err)
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
