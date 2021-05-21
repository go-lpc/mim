// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpi

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/go-daq/tdaq"
	"github.com/ziutek/ftdi"
)

type DeviceInfo struct {
	VendorID uint32
	ProdID   uint32
	Name     string
	ID       uint32
	Type     uint32
}

type Server struct {
	name string

	difs []uint32
	devs map[uint32]DeviceInfo
	rdos map[uint32]*Readout

	calib struct {
		gain       uint32
		thresholds [3]uint32
	}

	db map[uint32]*DbInfo
}

func (srv *Server) scanDevices(ctx tdaq.Context) error {
	srv.difs = srv.difs[:0]
	srv.devs = make(map[uint32]DeviceInfo)
	srv.db = make(map[uint32]*DbInfo)

	devs, err := ftdiListDevices(0x0403)
	if err != nil {
		return fmt.Errorf("could not build list of connected FTDI devices: %w", err)
	}

	for _, dev := range devs {
		ctx.Msg.Infof("found DIF 0x%x", dev.ProdID)
		srv.difs = append(srv.difs, dev.ID)
		srv.devs[dev.ID] = dev
	}

	sort.Slice(srv.difs, func(i, j int) bool {
		return srv.difs[i] < srv.difs[j]
	})

	return nil
}

func (srv *Server) initialize(ctx tdaq.Context, id uint32) error {
	if _, dup := srv.rdos[id]; dup {
		ctx.Msg.Errorf("DIF 0x%x already registered", id)
		return fmt.Errorf("DIF 0x%x already registered", id)
	}

	dev, ok := srv.devs[id]
	if !ok {
		ctx.Msg.Errorf("DIF 0x%x not found in device map", id)
		return fmt.Errorf("DIF 0x%x not found in device map", id)
	}

	rdo, err := NewReadout(dev.Name, dev.ProdID, ctx.Msg)
	if err != nil {
		ctx.Msg.Errorf("could not create readout for DIF 0x%x (name=%q): %w",
			id, dev.Name, err,
		)
		return fmt.Errorf("could not create readout for DIF 0x%x (name=%q): %w",
			id, dev.Name, err,
		)
	}
	defer func() {
		if err != nil {
			_ = rdo.close()
		}
	}()

	err = rdo.checkRW(0x1234, 100)
	if err != nil {
		ctx.Msg.Errorf("could not check r/w readout for DIF 0x%x (name=%q): %w",
			id, dev.Name, err,
		)
		return fmt.Errorf("could not check r/w readout for DIF 0x%x (name=%q): %w",
			id, dev.Name, err,
		)
	}

	srv.rdos[id] = rdo
	ctx.Msg.Infof("readout for DIF 0x%x: OK", id)

	return nil
}

func (srv *Server) preConfigure(ctx tdaq.Context, id, ctlreg uint32) error {
	rdo, ok := srv.rdos[id]
	if !ok {
		return fmt.Errorf("could not find readout for DIF 0x%x", id)
	}
	rdo.setPowerManagment(0x8c52, 0x3e6, 0xd640, 0x4e, 0x4e)
	err := rdo.dev.setControlRegister(ctlreg)
	if err != nil {
		return fmt.Errorf("could not set control register for readout 0x%x: %w", id, err)
	}
	err = rdo.configureRegisters()
	if err != nil {
		return fmt.Errorf("could not configure registers for readout 0x%x: %w", id, err)
	}

	return nil
}

func (srv *Server) configureChips(dif uint32, slow [][]byte, numASICs uint32) (uint32, error) {
	rdo, ok := srv.rdos[dif]
	if !ok || rdo == nil {
		return 0, fmt.Errorf("could not find readout 0x%x", dif)
	}

	if numASICs != MaxNumASICs {
		rdo.nasics = int(numASICs)
		err := rdo.configureRegisters()
		if err != nil {
			return 0, fmt.Errorf("could not configure registers with ASICs=%d for readout 0x%x: %w",
				numASICs, dif, err,
			)
		}
	}

	return rdo.configureChips(slow)
}

func (srv *Server) cmdScan(ctx tdaq.Context) error {
	return srv.scanDevices(ctx)
}

func (srv *Server) cmdInitialize(ctx tdaq.Context, req tdaq.Frame) error {
	dec := tdaq.NewDecoder(bytes.NewReader(req.Body))
	difid := dec.ReadU32()

	err := srv.initialize(ctx, difid)
	if err != nil {
		return fmt.Errorf("could not intialize DIF 0x%x: %w", difid, err)
	}

	return nil
}

func (srv *Server) cmdRegisterState(ctx tdaq.Context, req tdaq.Frame) error {
	dec := tdaq.NewDecoder(bytes.NewReader(req.Body))
	str := dec.ReadStr()
	if str != "" {
		panic("TODO")
	}
	// TODO(sbinet)
	return nil
}

func (srv *Server) cmdLoopConfigure(ctx tdaq.Context, req tdaq.Frame) error {
	dec := tdaq.NewDecoder(bytes.NewReader(req.Body))
	id := dec.ReadU32()
	n := int(dec.ReadU32())

	db, ok := srv.db[id]
	if !ok {
		return fmt.Errorf("could not retrieve db-info for DIF 0x%x", id)
	}

	if db.ID != id {
		return fmt.Errorf("inconsistent db-info: id=0x%x|0x%x", db.ID, id)
	}

	for i := 0; i < n; i++ {
		slc, err := srv.configureChips(id, db.Slow, db.NumASICs)
		if err != nil {
			return fmt.Errorf("could not configure chips for DIF 0x%x: %w", id, err)
		}
		str, ok := slcStatus(slc)
		ctx.Msg.Infof("slow control frame 0x%x: %s", id, str)
		if !ok {
			return fmt.Errorf("could not configure DIF 0x%x SLC=0x%x [%s]",
				id, slc, str,
			)
		}
	}

	// TODO(sbinet)
	return nil
}

func (srv *Server) cmdPreconfigure(ctx tdaq.Context, req tdaq.Frame) error {
	dec := tdaq.NewDecoder(bytes.NewReader(req.Body))
	ctlreg := dec.ReadU32()

	for _, id := range srv.difs {
		err := srv.preConfigure(ctx, id, ctlreg)
		if err != nil {
			return fmt.Errorf(
				"could not preconfigure readout 0x%x w/ ctlreg=0x%x: %w",
				id, ctlreg, err,
			)
		}

		db, ok := srv.db[id]
		if !ok {
			return fmt.Errorf("could not retrieve db-info for DIF 0x%x", id)
		}

		slc, err := srv.configureChips(id, db.Slow, db.NumASICs)
		if err != nil {
			return fmt.Errorf("could not configure chips for DIF 0x%x: %w", id, err)
		}
		str, ok := slcStatus(slc)
		ctx.Msg.Infof("slow control frame 0x%x: %s", id, str)
		if !ok {
			return fmt.Errorf("could not configure DIF 0x%x SLC=0x%x [%s]",
				id, slc, str,
			)
		}

	}

	return nil
}

func (srv *Server) cmdConfigureChips(ctx tdaq.Context, req tdaq.Frame) error {
	dec := tdaq.NewDecoder(bytes.NewReader(req.Body))
	dif := dec.ReadU32()
	nasics := dec.ReadU32()
	scframe := make([][]byte, nasics)
	for i := range scframe {
		scframe[i] = make([]byte, hardrocV2SLCFrameSize)
	}

	srv.BOO
	_, err := srv.configureChips(dif, scframe, nasics)
	if err != nil {
		return fmt.Errorf("could not configure chips for DIF=0x%x: %w", dif, err)
	}
	return nil
}

func (srv *Server) OnConfig(ctx tdaq.Context, resp *tdaq.Frame, req tdaq.Frame) error {
	ctx.Msg.Debugf("received /config command...")
	err := srv.scanDevices(ctx)
	if err != nil {
		ctx.Msg.Errorf("could not scan devices: %+v", err)
		return fmt.Errorf("could not scan devices: %w", err)
	}

	return nil
}

func (srv *Server) OnInit(ctx tdaq.Context, resp *tdaq.Frame, req tdaq.Frame) error {
	ctx.Msg.Debugf("received /init command...")
	for id := range srv.devs {
		dev := srv.devs[id]
		err := srv.initialize(ctx, dev.ID)
		if err != nil {
			ctx.Msg.Errorf("could not initialize DIF 0x%x: %+v", dev.ID, err)
			return fmt.Errorf("could not initialize DIF 0x%x: %w", dev.ID, err)
		}
	}
	return nil
}

func (srv *Server) OnReset(ctx tdaq.Context, resp *tdaq.Frame, req tdaq.Frame) error {
	ctx.Msg.Debugf("received /reset command...")
	return nil
}

func (srv *Server) OnStart(ctx tdaq.Context, resp *tdaq.Frame, req tdaq.Frame) error {
	ctx.Msg.Debugf("received /start command...")
	for _, id := range srv.difs {
		rdo, ok := srv.rdos[id]
		if !ok {
			return fmt.Errorf("could not find rdo w/ DIF=0x%x", id)
		}

		err := rdo.start()
		if err != nil {
			return fmt.Errorf("could not start readout for DIF=0x%x: %w", id, err)
		}
	}
	return nil
}

func (srv *Server) OnStop(ctx tdaq.Context, resp *tdaq.Frame, req tdaq.Frame) error {
	ctx.Msg.Debugf("received /stop command...")
	for _, id := range srv.difs {
		rdo, ok := srv.rdos[id]
		if !ok {
			return fmt.Errorf("could not find rdo w/ DIF=0x%x", id)
		}

		err := rdo.stop()
		if err != nil {
			return fmt.Errorf("could not stop readout for DIF=0x%x: %w", id, err)
		}
	}
	return nil
}

func (srv *Server) OnQuit(ctx tdaq.Context, resp *tdaq.Frame, req tdaq.Frame) error {
	ctx.Msg.Debugf("received /quit command...")
	return nil
}

func ftdiListDevices(vid uint16) ([]DeviceInfo, error) {
	var devs []DeviceInfo

	add := func(vid, pid uint16) {
		lst, err := ftdi.FindAll(int(vid), int(pid))
		if err != nil {
			return
		}
		for _, dev := range lst {
			var (
				difid uint32
				dtype uint32
			)
			switch {
			case strings.HasPrefix(dev.Serial, "FT101"):
				fmt.Sscanf(dev.Serial, "FT101%d", &difid)
				dtype = 0
			case strings.HasPrefix(dev.Serial, "DCCCCC"):
				fmt.Sscanf(dev.Serial, "DCCCCC%d", &difid)
				dtype = 0x10
			}

			devs = append(devs, DeviceInfo{
				VendorID: uint32(vid),
				ProdID:   uint32(pid),
				Name:     dev.Serial,
				ID:       difid,
				Type:     dtype,
			})
			dev.Close()
		}
	}

	add(vid, 0x6001) // usb-1
	add(vid, 0x6014) // usb-2

	return devs, nil
}
