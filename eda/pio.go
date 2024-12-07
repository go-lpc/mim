// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/go-lpc/mim/eda/internal/regs"
	"github.com/go-lpc/mim/internal/eformat"
	"github.com/go-lpc/mim/internal/mmap"
	"golang.org/x/sys/unix"
)

func (dev *Device) mmapLwH2F() error {
	data, err := unix.Mmap(
		int(dev.mem.fd.Fd()),
		regs.LW_H2F_BASE, regs.LW_H2F_SPAN,
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_SHARED,
	)
	if err != nil {
		return fmt.Errorf("eda: could not mmap lw-h2f: %w", err)
	}
	if data == nil || len(data) != regs.LW_H2F_SPAN {
		return fmt.Errorf("eda: invalid mmap'd data: %d", len(data))
	}
	dev.mem.lw = mmap.HandleFrom(data)

	err = dev.bindLwH2F()
	if err != nil {
		return fmt.Errorf("eda: could not read lw-h2f registers: %w", err)
	}

	return nil
}

func (dev *Device) mmapH2F() error {
	data, err := unix.Mmap(
		int(dev.mem.fd.Fd()),
		regs.H2F_BASE, regs.H2F_SPAN, unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_SHARED,
	)
	if err != nil {
		return fmt.Errorf("eda: could not mmap h2f: %w", err)
	}
	if data == nil || len(data) != regs.H2F_SPAN {
		return fmt.Errorf("eda: invalid mmap'd data: %d", len(data))
	}
	dev.mem.h2f = mmap.HandleFrom(data)

	err = dev.bindH2F()
	if err != nil {
		return fmt.Errorf("eda: could not read h2f registers: %w", err)
	}

	return nil
}

func (dev *Device) readThOffset(fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("eda: could not open threshold offsets file: %w", err)
	}
	defer f.Close()

	var (
		scan = bufio.NewScanner(f)
		line int
		rfm  uint32
		hr   uint32
	)
	for scan.Scan() {
		line++
		txt := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(txt, "#") || txt == "" {
			continue
		}
		toks := strings.Split(txt, ";")
		if len(toks) != 5 {
			return fmt.Errorf(
				"eda: invalid threshold offsets file:%d: line=%q",
				line, txt,
			)
		}
		v, err := strconv.ParseUint(toks[0], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse RFM id %q (line:%d): %w",
				toks[0], line, err,
			)
		}
		if uint32(v) != rfm {
			return fmt.Errorf(
				"eda: invalid RFM id=%d (line:%d), want=%d",
				v, line, rfm,
			)
		}

		v, err = strconv.ParseUint(toks[1], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse HR id %q (line:%d): %w",
				toks[1], line, err,
			)
		}
		if uint32(v) != hr {
			return fmt.Errorf(
				"eda: invalid HR id=%d (line:%d), want=%d",
				v, line, hr,
			)
		}

		v, err = strconv.ParseUint(toks[2], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse threshold value dac0 for (RFM=%d,HR=%d) (line:%d:%q): %w",
				rfm, hr, line, toks[2], err,
			)
		}
		dev.cfg.daq.floor[3*(nHR*rfm+hr)+0] = uint32(v)

		v, err = strconv.ParseUint(toks[3], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse threshold value dac1 for (RFM=%d,HR=%d) (line:%d:%q): %w",
				rfm, hr, line, toks[3], err,
			)
		}
		dev.cfg.daq.floor[3*(nHR*rfm+hr)+1] = uint32(v)

		v, err = strconv.ParseUint(toks[4], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse threshold value dac2 for (RFM=%d,HR=%d) (line:%d:%q): %w",
				rfm, hr, line, toks[4], err,
			)
		}
		dev.cfg.daq.floor[3*(nHR*rfm+hr)+2] = uint32(v)

		hr++
		if hr >= nHR {
			hr = 0
			rfm++
		}
	}

	err = scan.Err()
	if err != nil {
		return fmt.Errorf("eda: error while parsing threshold offsets: %w", err)
	}

	return nil
}

func (dev *Device) readPreAmpGain(fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("eda: could not open preamp-gain file: %w", err)
	}
	defer f.Close()

	var (
		scan = bufio.NewScanner(f)
		line int
		rfm  uint32
		hr   uint32
		ch   uint32
	)
	for scan.Scan() {
		line++
		txt := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(txt, "#") || txt == "" {
			continue
		}
		toks := strings.Split(txt, ";")
		if len(toks) != 4 {
			return fmt.Errorf(
				"eda: invalid preamp-gain file:%d: line=%q",
				line, txt,
			)
		}

		v, err := strconv.ParseUint(toks[0], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse RFM id %q (line:%d): %w",
				toks[0], line, err,
			)
		}
		if uint32(v) != rfm {
			return fmt.Errorf(
				"eda: invalid RFM id=%d (line:%d), want=%d",
				v, line, rfm,
			)
		}

		v, err = strconv.ParseUint(toks[1], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse HR id %q (line:%d): %w",
				toks[1], line, err,
			)
		}
		if uint32(v) != hr {
			return fmt.Errorf(
				"eda: invalid HR id=%d (line:%d), want=%d",
				v, line, hr,
			)
		}

		v, err = strconv.ParseUint(toks[2], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse chan %q (line:%d): %w",
				toks[2], line, err,
			)
		}
		if uint32(v) != ch {
			return fmt.Errorf(
				"eda: invalid chan id=%d (line:%d), want=%d",
				v, line, ch,
			)
		}

		v, err = strconv.ParseUint(toks[3], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse gain for (RFM=%d,HR=%d,ch=%d) (line:%d:%q): %w",
				rfm, hr, ch, line, toks[3], err,
			)
		}
		dev.cfg.preamp.gains[nChans*(nHR*rfm+hr)+ch] = uint32(v)
		ch++

		if ch >= nChans {
			ch = 0
			hr++
		}
		if hr >= nHR {
			hr = 0
			rfm++
		}
	}

	err = scan.Err()
	if err != nil {
		return fmt.Errorf("eda: error while parsing preamp-gains: %w", err)
	}

	return nil
}

func (dev *Device) readMask(fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("eda: could not open mask file: %w", err)
	}
	defer f.Close()

	var (
		scan = bufio.NewScanner(f)
		line int
		rfm  uint32
		hr   uint32
		ch   uint32
	)
	for scan.Scan() {
		line++
		txt := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(txt, "#") || txt == "" {
			continue
		}
		toks := strings.Split(txt, ";")
		if len(toks) != 4 {
			return fmt.Errorf(
				"eda: invalid mask file:%d: line=%q",
				line, txt,
			)
		}

		v, err := strconv.ParseUint(toks[0], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse RFM id %q (line:%d): %w",
				toks[0], line, err,
			)
		}
		if uint32(v) != rfm {
			return fmt.Errorf(
				"eda: invalid RFM id=%d (line:%d), want=%d",
				v, line, rfm,
			)
		}

		v, err = strconv.ParseUint(toks[1], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse HR id %q (line:%d): %w",
				toks[1], line, err,
			)
		}
		if uint32(v) != hr {
			return fmt.Errorf(
				"eda: invalid HR id=%d (line:%d), want=%d",
				v, line, hr,
			)
		}

		v, err = strconv.ParseUint(toks[2], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse chan %q (line:%d): %w",
				toks[2], line, err,
			)
		}
		if uint32(v) != ch {
			return fmt.Errorf(
				"eda: invalid chan id=%d (line:%d), want=%d",
				v, line, ch,
			)
		}

		v, err = strconv.ParseUint(toks[3], 10, 32)
		if err != nil {
			return fmt.Errorf(
				"eda: could not parse mask for (RFM=%d,HR=%d,ch=%d) (line:%d:%q): %w",
				rfm, hr, ch, line, toks[3], err,
			)
		}
		dev.cfg.mask.table[nChans*(nHR*rfm+hr)+ch] = uint32(v)
		ch++

		if ch >= nChans {
			ch = 0
			hr++
		}
		if hr >= nHR {
			hr = 0
			rfm++
		}
	}

	err = scan.Err()
	if err != nil {
		return fmt.Errorf("eda: error while parsing masks: %w", err)
	}

	return nil
}

func (dev *Device) bindLwH2F() error {
	return dev.brd.bindLwH2F(dev.mem.lw)
}

func (dev *Device) bindH2F() error {
	return dev.brd.bindH2F(dev.mem.h2f)
}

func (dev *Device) daqWriteDIFData(w io.Writer, slot int) {
	var (
		rfm  = &dev.daq.rfm[slot]
		fifo = &dev.brd.regs.fifo.daq[slot]
		wU8  = func(v uint8) {
			rfm.buf[0] = v
			_, _ = w.Write(rfm.buf[:1])
		}
		wU16 = func(v uint16) {
			binary.BigEndian.PutUint16(rfm.buf[:2], v)
			_, _ = w.Write(rfm.buf[:2])
		}
		wU32 = func(v uint32) {
			binary.BigEndian.PutUint32(rfm.buf[:4], v)
			_, _ = w.Write(rfm.buf[:4])
		}
	)

	// offset
	if rfm.cycle == 0 {
		rfm.bcid = dev.brd.cntBCID48LSB() - dev.brd.cntBCID24()
	}
	bcid48Offset := rfm.bcid

	// DIF DAQ header
	wU8(0xB0)
	wU8(dev.daq.rfm[slot].id)
	// counters
	wU32(rfm.cycle + 1) // FIXME(sbinet): off-by-one ?
	wU32(dev.brd.cntHit0(slot))
	//wU32(dev.cntHit1(rfm)) // FIXME(sbinet): hack
	wU32(rfm.cycle + 1) // FIXME(sbinet): hack (and off-by-one?)
	// assemble and correct absolute BCID
	bcid48 := uint64(dev.brd.cntBCID48MSB())
	bcid48 <<= 32
	bcid48 |= uint64(dev.brd.cntBCID48LSB())
	bcid48 -= uint64(bcid48Offset)
	// copy frame
	wU16(uint16(bcid48>>32) & 0xffff)
	wU32(uint32(bcid48))
	bcid24 := dev.brd.cntBCID24()
	wU8(uint8(bcid24 >> 16))
	wU16(uint16(bcid24 & 0xffff))
	// unused "nb-lines"
	wU8(0xff)

	// HR DAQ chunk
	var (
		lastHR = -1
		hrID   int
	)
	wU8(0xB4) // HR header

	const nWordsPerHR = 5
	n := int(dev.brd.daqFIFOFillLevel(slot) / nWordsPerHR)

	for i := 0; i < n; i++ {
		// read HR ID
		id := fifo.r()
		hrID = int(id >> 24)
		// insert trailer and header if new hardroc ID
		if hrID != lastHR {
			if lastHR >= 0 {
				wU8(0xA3) // HR trailer
				wU8(0xB4) // HR header
			}
		}
		wU32(id)
		wU32(fifo.r())
		wU32(fifo.r())
		wU32(fifo.r())
		wU32(fifo.r())
		lastHR = hrID
	}
	wU8(0xA3)    // last HR trailer
	wU8(0xA0)    // DIF DAQ trailer
	wU16(0xC0C0) // fake CRC

	rfm.cycle++
}

func (dev *Device) daqSendDIFData(i int) error {
	var (
		sink = &dev.daq.rfm[i]
		buf  = sink.buf
		w    = sink.w
		sck  = sink.sck
	)
	defer func() {
		w.c = 0
	}()

	errorf := func(format string, args ...interface{}) error {
		err := fmt.Errorf(format, args...)
		dev.msg.Printf("%+v", err)
		return err
	}

	hdr := buf[:8]
	cur := w.c
	copy(hdr, "HDR\x00")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(cur))

	_, err := sck.Write(hdr)
	if err != nil {
		return errorf(
			"eda: could not send DIF data size header to %v: %w",
			sck.RemoteAddr(), err,
		)
	}

	// wait for ACK
	_, err = io.ReadFull(sck, hdr[:4])
	if err != nil {
		return errorf(
			"eda: could not read ACK DIF header from %v: %+v",
			sck.RemoteAddr(), err,
		)
	}
	if string(hdr[:4]) != "ACK\x00" {
		return errorf(
			"eda: invalid ACK DIF header from %v: %q",
			sck.RemoteAddr(), hdr[:4],
		)
	}

	if cur == 0 {
		return nil
	}

	_, err = sck.Write(w.p[:cur])
	if err != nil {
		return errorf(
			"eda: could not send DIF data to %v: %w",
			sck.RemoteAddr(), err,
		)
	}

	if false {
		_, _ = dev.daq.f.Write(w.p[:cur])
		dec := eformat.NewDecoder(sink.id, bytes.NewReader(w.p[:cur]))
		dec.IsEDA = true
		var d eformat.DIF
		err = dec.Decode(&d)
		if err != nil {
			dev.msg.Printf("could not decode DIF: %+v", err)
		} else {
			wbuf := dev.msg.Writer()
			fmt.Fprintf(wbuf, "=== DIF-ID 0x%x ===\n", d.Header.ID)
			fmt.Fprintf(wbuf, "DIF trigger: % 10d\n", d.Header.DTC)
			fmt.Fprintf(wbuf, "ACQ trigger: % 10d\n", d.Header.ATC)
			fmt.Fprintf(wbuf, "Gbl trigger: % 10d\n", d.Header.GTC)
			fmt.Fprintf(wbuf, "Abs BCID:    % 10d\n", d.Header.AbsBCID)
			fmt.Fprintf(wbuf, "Time DIF:    % 10d\n", d.Header.TimeDIFTC)
			fmt.Fprintf(wbuf, "Frames:      % 10d\n", len(d.Frames))

			for _, frame := range d.Frames {
				fmt.Fprintf(wbuf, "  hroc=0x%02x BCID=% 8d %x\n",
					frame.Header, frame.BCID, frame.Data,
				)
			}
		}
	}

	// wait for ACK
	_, err = io.ReadFull(sck, hdr[:4])
	if err != nil {
		return errorf(
			"eda: could not read ACK DIF data from %v: %+v",
			sck.RemoteAddr(), err,
		)
	}
	if string(hdr[:4]) != "ACK\x00" {
		return errorf(
			"eda: invalid ACK DIF data from %v: %q",
			sck.RemoteAddr(), hdr[:4],
		)
	}

	return nil
}
