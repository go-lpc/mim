// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/go-lpc/mim/conddb"
	"github.com/go-lpc/mim/eda/internal/regs"
)

func TestServerFail(t *testing.T) {
	const (
		addr   = ":invalid"
		odir   = ""
		devmem = "/dev/mem"
		devshm = "/dev/shm"
		cfgdir = ""
	)

	err := Serve(addr, odir, devmem, devshm, cfgdir)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestServer(t *testing.T) {
	fdev, err := newFakeDev()
	if err != nil {
		t.Fatalf("could not create fake-dev: %+v", err)
	}
	defer fdev.close()

	var (
		odir   = fdev.tmpdir
		cfgdir = ""
	)

	addr, err := getTCPPort()
	if err != nil {
		t.Fatalf("could not get TCP port: %+v", err)
	}
	addr = "localhost:" + addr

	srv, err := newServer(
		addr, odir, fdev.mem, fdev.shm, cfgdir,
		func(dev *Device) { dev.cfg.mode = "db" },
		WithRFMMask(1<<1), // dummy
	)
	if err != nil {
		t.Fatal(err)
	}

	quit := make(chan int)
	defer close(quit)

	makeSink := func(addr string) net.Listener {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatalf("could not listen on %q: %+v", addr, err)
		}
		return l
	}

	sinks := map[int]net.Listener{
		1: makeSink("localhost:10001"),
		//2: makeSink("localhost:10002"),
	}

	for k := range sinks {
		v := sinks[k]
		go func(l net.Listener) {
			conn, err := l.Accept()
			if err != nil {
				t.Errorf("could not accept on %q: %+v", l.Addr(), err)
				return
			}
			defer conn.Close()

			buf := make([]byte, 8+daqBufferSize)
			for {
				select {
				case <-quit:
					return
				default:
					_, err := conn.Read(buf[:8])
					if err != nil {
						if errors.Is(err, io.EOF) {
							return
						}
						t.Errorf("could not read DAQ DIF header: %+v", err)
						continue
					}
					size := binary.LittleEndian.Uint32(buf[4:8])
					if size == 0 {
						continue
					}
					_, err = conn.Read(buf[:size])
					if err != nil {
						t.Errorf("could not read DAQ DIF data: %+v", err)
						continue
					}
					copy(buf[:4], "ACK\x00")
					_, err = conn.Write(buf[:4])
					if err != nil {
						t.Errorf("could not send back ACK: %+v", err)
						continue
					}
				}
			}
		}(v)
		defer v.Close()
	}

	errch := make(chan error)
	go func() {
		errch <- srv.serve()
	}()

	dim, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("could not dial eda-srv: %+v", err)
	}
	defer dim.Close()

	ack := func(name string) {
		var rep struct {
			Msg string `json:"msg"`
		}

		err := json.NewDecoder(dim).Decode(&rep)
		if err != nil {
			t.Fatalf("could not read %q-reply from eda-srv: %+v", name, err)
		}
		if rep.Msg != "ok" {
			t.Fatalf("invalid %q-reply from eda-srv: %q", name, rep.Msg)
		}
	}

	ackErr := func(name string) {
		var rep struct {
			Msg string `json:"msg"`
		}

		err := json.NewDecoder(dim).Decode(&rep)
		if err != nil {
			t.Fatalf("could not read %q-reply from eda-srv: %+v", name, err)
		}
		if rep.Msg == "ok" {
			t.Fatalf("invalid %q-reply from eda-srv: %q", name, rep.Msg)
		}
	}

	for _, name := range []string{
		"err-invalid-req",
		"err-invalid-cmd",
		"scan",
		"err-scan",
		"err-configure",
		"err-initialize",
		"err-start",
		"err-start-run-nbr",
		"err-stop",

		"configure",
		"initialize",
		"start",
		"stop",
	} {
		srv.msg.Printf("sending %q...", name)
		switch name {
		case "scan":
			type DAQ struct {
				RShaper     int `json:"rshaper"`
				TriggerMode int `json:"trigger_type"`
			}
			type Arg struct {
				RFM  int `json:"rfm"`
				EDA  int `json:"eda"`
				Slot int `json:"slot"`
				DAQ  DAQ `json:"daq_state"`
			}
			type Req struct {
				Name string `json:"name"`
				Args []Arg  `json:"args"`
			}
			req := Req{
				Name: name,
				Args: []Arg{
					{RFM: 1, EDA: 1, Slot: 2, DAQ: DAQ{RShaper: 3}},
					//{RFM: 2, EDA: 1, Slot: 3},
				},
			}
			err = json.NewEncoder(dim).Encode(req)
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ack(name)
			fdev.fpga(srv.dev, 2, regs.O_SC_DONE_2, nil)

		case "err-invalid-req":
			_, err = dim.Write([]byte("{]"))
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ackErr(name)

		case "err-invalid-cmd":
			_, err = dim.Write([]byte(`{"name":"unknown-command"}`))
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ackErr(name)

		case "err-scan":
			_, err = dim.Write([]byte(
				`{"name":"scan", "args":[{"rfm": "1"}]}`,
			))
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ackErr(name)

		case "err-configure":
			_, err = dim.Write([]byte(
				`{"name":"configure", "args":[{"dif": "1"}]}`,
			))
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ackErr(name)

		case "err-initialize":
		//	_, err = dim.Write([]byte(
		//		`{"name":"initialize", "args":[{"dif": "1"}]}`,
		//	))
		//	if err != nil {
		//		t.Fatalf("could not send %q: %+v", name, err)
		//	}
		//	ackErr(name)

		case "err-start":
			_, err = dim.Write([]byte(
				`{"name":"start", "args":[42]}`,
			))
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ackErr(name)

		case "err-start-run-nbr":
			_, err = dim.Write([]byte(
				`{"name":"start", "args":["x"]}`,
			))
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ackErr(name)

		case "err-stop":
		//	_, err = dim.Write([]byte(
		//		`{"name":"stop", "args":[]}`,
		//	))
		//	if err != nil {
		//		t.Fatalf("could not send %q: %+v", name, err)
		//	}
		//	ackErr(name)

		case "configure":
			type Arg struct {
				DIF   uint8         `json:"dif"`
				ASICS []conddb.ASIC `json:"asics"`
			}
			type Req struct {
				Name string `json:"name"`
				Args []Arg  `json:"args"`
			}

			req := Req{
				Name: name,
				Args: []Arg{
					{DIF: 1, ASICS: loadASICs(t, 1)},
					//{DIF: 2, ASICS: loadASICs(t, 2)},
				},
			}
			err = json.NewEncoder(dim).Encode(req)
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ack(name)

		case "initialize":
			type Req struct {
				Name string `json:"name"`
			}
			req := Req{
				Name: name,
			}
			err = json.NewEncoder(dim).Encode(req)
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ack(name)

		case "start":
			type Req struct {
				Name string   `json:"name"`
				Args []string `json:"args"`
			}
			req := Req{
				Name: name,
				Args: []string{"42"},
			}
			err = json.NewEncoder(dim).Encode(req)
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ack(name)

		case "stop":
			type Req struct {
				Name string `json:"name"`
			}
			req := Req{
				Name: name,
			}

			err = json.NewEncoder(dim).Encode(req)
			if err != nil {
				t.Fatalf("could not send %q: %+v", name, err)
			}
			ack(name)
		}
	}

	srv.close()

	err = <-errch
	if err != nil && !isErrClosed(err) {
		t.Fatalf("could not run server: %+v", err)
	}
}

func isErrClosed(err error) bool {
	// FIXME(sbinet): when Go-1.16 is out:
	// return errors.Is(err, net.ErrClosed)
	return strings.HasSuffix(err.Error(), "use of closed network connection")
}

func loadASICs(t *testing.T, dif uint8) []conddb.ASIC {
	raw, err := os.Open(fmt.Sprintf("testdata/asic-rfm-%03d.json", dif))
	if err != nil {
		t.Fatalf("could not load ASICs cfg for dif=%d: %+v", dif, err)
	}
	defer raw.Close()

	var asics []conddb.ASIC
	err = json.NewDecoder(raw).Decode(&asics)
	if err != nil {
		t.Fatalf("could not decode ASICs cfg for dif=%d: %+v", dif, err)
	}

	return asics
}

func getTCPPort() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return "", err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port), nil
}
