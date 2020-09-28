// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/go-lpc/mim/conddb"
	_ "github.com/go-sql-driver/mysql"
)

const (
	dbname = "tmvsrv"
)

func main() {
	log.SetPrefix("mim-sql: ")
	log.SetFlags(0)

	var (
		hrcfg = flag.String("hr-cfg", "", "HardRoc config to inspect")
		dif   = flag.Int("dif", 0x9, "DIF ID to inspect")
	)

	flag.Parse()

	log.Printf("dif: %03d", *dif)
	log.Printf("cfg: %q", *hrcfg)

	db, err := conddb.Open(dbname)
	if err != nil {
		log.Fatalf("could not open MIM db: %+v", err)
	}
	defer db.Close()

	err = doQuery(db, *hrcfg, *dif)
	if err != nil {
		log.Fatalf("could not do query: %+v", err)
	}
}

func doQuery(db *conddb.DB, hrConfig string, difID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if hrConfig == "" {
		v, err := db.LastHRConfig(ctx)
		if err != nil {
			return fmt.Errorf("could not get last hrconfig value: %w", err)
		}
		hrConfig = v
		log.Printf("hrconfig: %q", hrConfig)
	}

	asics, err := db.ASICConfig(ctx, hrConfig, uint8(difID))
	if err != nil {
		return fmt.Errorf("could not get ASIC cfg (hr=%q, id=0x%x): %w",
			hrConfig, uint8(difID), err,
		)
	}
	log.Printf("asics: %d", len(asics))
	//	for i, asic := range asics {
	//		log.Printf("row[%d]: %#v (%q)", i, asic, asic.PreAmpGain)
	//	}

	detid, err := db.LastDetectorID(ctx)
	if err != nil {
		return fmt.Errorf("could not get last det-id: %w", err)
	}
	log.Printf("det-id: %d", detid)
	{
		rows, err := db.QueryContext(ctx, "SELECT dif, asu, iy FROM chambers WHERE detector=? ORDER BY dif", detid)
		if err != nil {
			return fmt.Errorf("could not get chambers definition: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var (
				difid uint32
				asu   uint32
				iy    uint32
			)
			err = rows.Scan(&difid, &asu, &iy)
			if err != nil {
				return fmt.Errorf("could not scan chambers definition: %w", err)
			}
			switch {
			case difid < 100:
				log.Printf(">>> dif=%03d, eda=%02d, slot=%d", difid, asu, iy)
			default:
				log.Printf(">>> dif=%03d, asu=%02d, iy=%d", difid, asu, iy)
			}
		}
	}

	daqstates, err := db.DAQStates(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve daqstates: %w", err)
	}
	log.Printf("daqstates: %d", len(daqstates))
	for i, daq := range daqstates {
		log.Printf("row[%d]: %#v", i, daq)
	}

	return nil
}
