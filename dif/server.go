// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dif

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-daq/tdaq"
	"github.com/ziutek/ftdi"
	"golang.org/x/xerrors"
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
}

func (srv *Server) scanDevices(ctx tdaq.Context) error {
	srv.difs = srv.difs[:0]
	srv.devs = make(map[uint32]DeviceInfo)

	devs, err := ftdiListDevices(0x0403)
	if err != nil {
		return xerrors.Errorf("could not build list of connected FTDI devices: %w", err)
	}

	for _, dev := range devs {
		ctx.Msg.Infof("found DIF 0x%x", dev.ProdID)
		srv.difs = append(srv.difs, dev.ID)

		srv.devs[dev.ID] = dev
	}

	sort.Slice(srv.difs, func(i, j int) bool {
		di := srv.difs[i]
		dj := srv.difs[j]
		return di < dj
	})

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
