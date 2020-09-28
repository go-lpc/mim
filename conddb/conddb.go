// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conddb holds types to describe the condition and configuration
// database for the MIM detector.
package conddb // import "github.com/go-lpc/mim/conddb"

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	host = "localhost"
)

var (
	usr = "username"
	pwd = "s3cr3t"

	drvName = "mysql"
)

// DB exposes convenience methods to easily retrieve conditions data
// and configuration data from the MIM database.
type DB struct {
	db   *sql.DB
	name string // name of the MIM database
}

// Open opens a connection to the MIM database dbname.
func Open(dbname string) (*DB, error) {
	db, err := sql.Open(drvName, dsn(dbname))
	if err != nil {
		return nil, fmt.Errorf("conddb: could not open %q db: %w", dbname, err)
	}

	err = ping(db, dbname)
	if err != nil {
		return nil, fmt.Errorf("conddb: could not ping %q db: %w", dbname, err)
	}

	return &DB{db: db, name: dbname}, nil
}

func dsn(db string) string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s", usr, pwd, host, db)
}

func ping(db *sql.DB, dbname string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("conddb: could not ping %q db: %w", dbname, err)
	}

	return nil
}

func (db *DB) Close() error {
	return db.db.Close()
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return db.db.QueryContext(ctx, query, args...)
}

func (db *DB) LastHRConfig(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	hrcfg := ""
	rows, err := db.db.QueryContext(
		ctx,
		"SELECT hrconfig FROM detectors ORDER BY datetime DESC LIMIT 1",
	)
	if err != nil {
		return hrcfg, fmt.Errorf("conddb: could not query HR cfg: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&hrcfg)
		if err != nil {
			return hrcfg, fmt.Errorf("conddb: could not get HR cfg value: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return hrcfg, fmt.Errorf("conddb: could not scan db for HR cfg: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return hrcfg, fmt.Errorf("conddb: context error while retrieving HR cfg: %w", err)
	}

	return hrcfg, nil
}

func (db *DB) LastDetectorID(ctx context.Context) (uint32, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var detid uint32
	rows, err := db.db.QueryContext(
		ctx,
		"SELECT identifier FROM detectors ORDER BY datetime DESC LIMIT 1",
	)
	if err != nil {
		return detid, fmt.Errorf("conddb: could not query detector-id: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&detid)
		if err != nil {
			return detid, fmt.Errorf("conddb: could not get detector-id value: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return detid, fmt.Errorf("conddb: could not scan db for detector-id: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return detid, fmt.Errorf("conddb: context error while retrieving detector-id: %w", err)
	}

	return detid, nil
}

func (db *DB) ASICConfig(ctx context.Context, hrConfig string, difID uint8) ([]ASIC, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var (
		cfg = make([]ASIC, 0, numASICs)
		err error
	)

	rows, err := db.db.QueryContext(
		ctx,
		`
SELECT asics.* FROM asics
JOIN hrconfig_asics ON asics.identifier=hrconfig_asics.asic
JOIN hrconfig       ON hrconfig.identifier=hrconfig_asics.hrconfig
WHERE (
	hrconfig.name=? AND asics.dif_id=?
)
`,
		hrConfig, difID,
	)
	if err != nil {
		return cfg, fmt.Errorf("conddb: could not run ASIC cfg query: %w", err)
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		var asic ASIC
		err = rows.Scan(
			&asic.PrimaryID, &asic.Header, &asic.DIFID,
			&asic.Razchnextval, &asic.Razchnintval,
			&asic.Trigextval, &asic.EnTrigOut,
			&asic.Trig0b, &asic.Trig1b, &asic.Trig2b,
			&asic.SmallDAC,
			&asic.B2, &asic.B1, &asic.B0,
			&asic.Mask2, &asic.Mask1, &asic.Mask0,
			&asic.Sw50f0, &asic.Sw100f0, &asic.Sw100k0,
			&asic.Sw50k0, &asic.Sw50f1, &asic.Sw100f1,
			&asic.Sw100k1, &asic.Sw50k1,
			&asic.Cmdb0fsb1, &asic.Cmdb1fsb1, &asic.Cmdb2fsb1,
			&asic.Cmdb3fsb1,
			&asic.Sw50f2, &asic.Sw100f2, &asic.Sw100k2, &asic.Sw50k2,
			&asic.Cmdb0fsb2, &asic.Cmdb1fsb2, &asic.Cmdb2fsb2, &asic.Cmdb3fsb2,
			&asic.PreAmpGain,
		)
		if err != nil {
			return cfg, fmt.Errorf("conddb: could not scan row %d for ASIC cfg: %w", i, err)
		}
		i++

		cfg = append(cfg, asic)
	}

	if err := rows.Err(); err != nil {
		return cfg, fmt.Errorf("conddb: could not scan db for ASIC cfg: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return cfg, fmt.Errorf("conddb: context error while retrieving ASIC cfg: %w", err)
	}

	return cfg, nil
}

func (db *DB) DAQStates(ctx context.Context) ([]DAQState, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var cfg []DAQState
	rows, err := db.db.QueryContext(ctx, "SELECT * FROM daqstates")
	if err != nil {
		return cfg, fmt.Errorf(
			"conddb: could not run daqstates query: %w",
			err,
		)
	}
	defer rows.Close()

	for rows.Next() {
		var daq DAQState
		err = rows.Scan(&daq.ID, &daq.HRConfig, &daq.RShape, &daq.TriggerMode)
		if err != nil {
			return cfg, fmt.Errorf(
				"conddb: could not scan daqstates: %w",
				err,
			)
		}
		cfg = append(cfg, daq)
	}

	if err := rows.Err(); err != nil {
		return cfg, fmt.Errorf(
			"conddb: could not scan db for daqstates: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return cfg, fmt.Errorf(
			"conddb: context error while retrieving daqstates: %w",
			err,
		)
	}

	return cfg, nil
}
