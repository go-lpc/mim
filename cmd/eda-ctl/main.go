// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command eda-ctl controls the C acq_chb_client process.
package main // import "github.com/go-lpc/mim/cmd/eda-ctl"

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mail "gopkg.in/gomail.v2"
)

func main() {
	var (
		name = flag.String("cmd", "acq_chb_client", "command to run")
		addr = flag.String("addr", ":8866", "[ip]:port to listen on")
		dir  = flag.String("dir", "", "directory to monitor")
		freq = flag.Duration("freq", 30*time.Second, "probing interval")
	)

	flag.Parse()

	log.SetPrefix("eda-ctl: ")
	log.SetFlags(0)

	run(*name, *addr, *dir, *freq)
}

func run(name, addr, dir string, freq time.Duration) {
	srv, err := newServer(addr, dir, freq)
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

	dir    string
	freq   time.Duration
	alerts map[string]int // keep track of the number of alerts per file
}

func newServer(addr, dir string, freq time.Duration) (*server, error) {
	srv, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("could not listen on %q: %w", addr, err)
	}
	return &server{
		conn:   srv,
		buf:    new(bytes.Buffer),
		dir:    dir,
		freq:   freq,
		alerts: make(map[string]int),
	}, nil
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
	done := make(chan int)
	defer close(done)

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
			srv.cmd.Stdout = os.Stdout
			srv.cmd.Stderr = io.MultiWriter(os.Stderr, srv.buf)
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

			run := req.Args[4]
			go srv.monitor(run, done)

		case "stop":
			log.Printf("stopping command...")
			// make sure the process is eventually reaped by PID-1
			go func() { _ = srv.cmd.Wait() }()
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
}

type Reply struct {
	Msg string `json:"msg"`
	Err string `json:"err,omitempty"`
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

func (srv *server) monitor(run string, quit chan int) {
	var (
		tick  = time.NewTicker(srv.freq)
		table = make(map[string]int64)
	)

	defer tick.Stop()

	for {
		select {
		case <-quit:
			return
		case <-tick.C:
			cur, err := srv.list(srv.dir, run)
			if err != nil {
				log.Printf("could not list files: %+v", err)
				continue
			}
			srv.compare(table, cur)
			table = cur
		}
	}
}

func (srv *server) list(dir, run string) (map[string]int64, error) {
	table := make(map[string]int64)
	glob := filepath.Join(dir, "eda_*"+run+"*raw")
	files, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("could not glob %q: %w", glob, err)
	}
	for _, fname := range files {
		fi, err := os.Stat(fname)
		if err != nil {
			return nil, fmt.Errorf("could not stat %q: %w", fname, err)
		}
		table[fname] = fi.Size()
	}
	return table, nil
}

func (srv *server) compare(ref, chk map[string]int64) {
	for fname := range chk {
		if _, ok := ref[fname]; !ok {
			// file just appeared.
			// nothing to compare against.
			continue
		}
		refsz := ref[fname]
		chksz := chk[fname]
		if refsz == chksz {
			// file didn't grow!
			srv.alert(fname, refsz)
		}
	}
}

func (srv *server) alert(fname string, size int64) {
	log.Printf("file %q didn't change in the last %v (size=%d bytes)",
		fname, srv.freq, size,
	)
	srv.alerts[fname]++

	const maxAlerts = 5
	if srv.alerts[fname] < maxAlerts {
		srv.alertMail(fname, size)
		//srv.alertSMS(fname, size)
	}
}

var (
	alertMailUsr  = os.Getenv("MAIL_USERNAME")
	alertMailPwd  = os.Getenv("MAIL_PASSWORD")
	alertMailSrv  = os.Getenv("MAIL_SERVER")
	alertMailPort = atoi(os.Getenv("MAIL_PORT"))
	alertMailTgts = strings.Split(os.Getenv("MAIL_TGTS"), ",")
)

func (srv *server) alertMail(fname string, size int64) {
	if alertMailUsr == "" || alertMailPwd == "" ||
		alertMailSrv == "" || alertMailPort == 0 ||
		alertMailTgts == nil || len(alertMailTgts) == 0 {
		log.Printf("could not send mail alert: missing credentials")
		return
	}

	msg := mail.NewMessage()
	msg.SetHeader("From", alertMailUsr)
	msg.SetHeader("Bcc", alertMailTgts...)
	msg.SetHeader("Subject", fmt.Sprintf("[eda-ctl] file alert: %q", fname))
	msg.SetBody("text/plain", fmt.Sprintf("file: %q\nsize: %d bytes\nfreq: %v",
		fname, size, srv.freq,
	))

	dial := mail.NewDialer(alertMailSrv, alertMailPort, alertMailUsr, alertMailPwd)
	dial.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	err := dial.DialAndSend(msg)
	if err != nil {
		log.Printf("could not send mail alert: %+v", err)
	}
}

var (
	alertSMSEndPoint = os.Getenv("SMS_ENDPOINT")
)

func (srv *server) alertSMS(fname string, size int64) {
	if alertSMSEndPoint == "" {
		log.Printf("could not send sms alert: no end-point")
		return
	}

	var msg struct {
		Action string `json:"action"`
		Data   struct {
			All bool   `json:"all"`
			Msg string `json:"message"`
		}
	}
	msg.Action = "send"
	msg.Data.All = true
	msg.Data.Msg = fmt.Sprintf("[eda-ctl]: alert file=%q size=%d freq=%v",
		fname, size, srv.freq,
	)

	data := new(bytes.Buffer)
	err := json.NewEncoder(data).Encode(msg)
	if err != nil {
		log.Printf("could not encode sms to json: %+v", err)
		return
	}
	resp, err := http.Post(alertSMSEndPoint, "application/json", data)
	if err != nil {
		log.Printf("could not POST sms alert: %+v", err)
		return
	}
	defer resp.Body.Close()

	var status struct {
		Msg string `json:"status"`
	}
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		log.Printf("could not decode sms reply: %+v", err)
		return
	}
	if status.Msg != "success" {
		log.Printf("could not send sms: status=%q", status.Msg)
		return
	}
}

func atoi(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}
