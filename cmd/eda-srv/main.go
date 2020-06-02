// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command eda-srv manages the data output files from EDA.
package main // import "github.com/go-lpc/mim/cmd/eda-srv"

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	log.SetPrefix("eda-srv: ")
	log.SetFlags(0)

	var (
		odir = flag.String("dir", "", "output directory where to store files fetched from EDA")
		host = flag.String("host", "", "EDA host where to fetch files from")
		addr = flag.String("addr", ":8080", "[ip]:[port] to listen on")
	)

	flag.Parse()

	run(*odir, *host, *addr)
}

func run(odir, host, addr string) {
	srv, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("could not listen on %q: %+v", addr, err)
	}
	defer srv.Close()

	for {
		conn, err := srv.Accept()
		if err != nil {
			log.Printf("could not accept connection: %+v", err)
		}
		go serve(conn, odir, host)
	}
}

func serve(conn net.Conn, odir, host string) {
	defer conn.Close()

	log.Printf("serving %q...", conn.RemoteAddr().String())
	buf := make([]byte, 4)
	for {
		_, err := io.ReadFull(conn, buf[:4])
		if err != nil {
			log.Printf("could not read message size header: %+v", err)
			return
		}
		sz := binary.LittleEndian.Uint32(buf[:4])
		log.Printf("message size: %d", sz)

		buf = make([]byte, sz)
		_, err = io.ReadFull(conn, buf)
		if err != nil {
			log.Printf("could not read file path: %+v", err)
			return
		}

		fname := string(buf)

		log.Printf("sending ACK for %q...", fname)
		_, err = conn.Write([]byte("ACK"))
		if err != nil {
			log.Printf("could not send ACK message back: %+v", err)
		}

		log.Printf("fetching file %q...", fname)
		err = fetch(odir, host, fname)
		if err != nil {
			log.Printf("could not fetch file %q from %q: %+v", fname, host, err)
		}
	}
}

func fetch(odir, host, fname string) error {
	cmd := exec.Command("scp", "-oCiphers=aes128-ctr", host+":"+fname, filepath.Join(odir, filepath.Base(fname)))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("could not copy file from %q: %w", host, err)
	}
	return nil
}
