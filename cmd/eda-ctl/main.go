// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command eda-ctl controls the C acq_chb_client process.
package main // import "github.com/go-lpc/mim/cmd/eda-ctl"

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	var (
		name = flag.String("cmd", "acq_chb_client", "command to run")
		addr = flag.String("addr", ":8866", "[ip]:port to listen on")
	)

	flag.Parse()

	log.SetPrefix("eda-ctl: ")
	log.SetFlags(0)

	run(*name, *addr)
}

func run(name, addr string) {
	srv, err := newServer(addr)
	if err != nil {
		log.Fatalf("could not create server: %+v", err)
	}
	log.Printf("running eda-ctl server on %q...", addr)
	srv.run(name)
}

type server struct {
	conn net.Listener
	cmd  *exec.Cmd
	buf  *bytes.Buffer
}

func newServer(addr string) (*server, error) {
	srv, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("could not listen on %q: %w", addr, err)
	}
	return &server{conn: srv, buf: new(bytes.Buffer)}, nil
}

func (srv *server) run(name string) {
	defer srv.conn.Close()

	for {
		conn, err := srv.conn.Accept()
		if err != nil {
			log.Printf("could not accept connection: %+v", err)
		}
		go srv.handle(conn, name)
	}
}

func (srv *server) handle(conn net.Conn, name string) {
	defer conn.Close()
	for {
		var (
			req Request
			err = json.NewDecoder(conn).Decode(&req)
		)
		if err != nil {
			log.Printf("could not decode command: %+v", err)
			return
		}
		switch req.Name {
		case "start":
			log.Printf("starting command... %s %v", name, req.Args)
			// FIXME(sbinet): mutex?
			srv.buf.Reset()
			srv.cmd = exec.Command(name, req.Args...)
			srv.cmd.Stdout = io.MultiWriter(os.Stdout, srv.buf)
			srv.cmd.Stderr = os.Stderr
			err = srv.cmd.Start()
			if err != nil {
				log.Printf("could not start %s %s: %+v",
					srv.cmd.Path,
					strings.Join(srv.cmd.Args, " "),
					err,
				)
				_ = json.NewEncoder(conn).Encode(Reply{Err: err.Error()})
				return
			}
			err = srv.checkCmdStatus()
			if err != nil {
				_ = srv.cmd.Process.Kill()
				log.Printf("command not in proper state: %+v", err)
				_ = json.NewEncoder(conn).Encode(Reply{Err: err.Error()})
				return
			}
			_ = json.NewEncoder(conn).Encode(Reply{Msg: "ok"})
			log.Printf("starting command... [done]")

		case "stop":
			log.Printf("stopping command...")
			err = srv.cmd.Process.Signal(os.Interrupt)
			if err != nil {
				log.Printf("could not stop %s %s: %+v",
					srv.cmd.Path,
					strings.Join(srv.cmd.Args, " "),
					err,
				)
				_ = json.NewEncoder(conn).Encode(Reply{Err: err.Error()})
				return
			}
			_ = json.NewEncoder(conn).Encode(Reply{Msg: "ok"})
			log.Printf("stopping command... [done]")
			return

		default:
			log.Printf("unknown command %q", req.Name)
			_ = json.NewEncoder(conn).Encode(Reply{Err: "unknown command"})
		}
	}
}

type Request struct {
	Name string   `json:"cmd"`
	Args []string `json:"args"`
	Err  string   `json:"err"`
}

type Reply struct {
	Msg string `json:"msg"`
	Err string `json:"err"`
}

func (srv *server) checkCmdStatus() error {
	var (
		timeout = 10 * time.Second
		timer   = time.NewTimer(timeout)
	)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return fmt.Errorf(
				"could not assess command status before timeout (%v)",
				timeout,
			)
		default:
			buf := srv.buf.Bytes()
			buf = bytes.TrimSpace(buf)
			buf = bytes.TrimRight(buf, "\r\n")
			if bytes.HasSuffix(buf, []byte("waiting for reset_BCID command")) {
				return nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}
