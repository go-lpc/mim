// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conddb

import (
	"bytes"
	"context"
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"

	"github.com/go-lpc/mim/internal/fakedb"
)

func init() {
	drvName = "fakedb"
}

func TestOpen(t *testing.T) {
	db, err := Open("fakedb")
	if err != nil {
		t.Fatalf("could not open conddb: %+v", err)
	}
	defer db.Close()
}

func TestLastHRConfig(t *testing.T) {
	db, err := Open("fakedb")
	if err != nil {
		t.Fatalf("could not open conddb: %+v", err)
	}
	defer db.Close()

	_ = fakedb.Run(context.Background(), fakedb.Rows{
		Names: []string{"hrconfig"},
		Values: [][]driver.Value{
			{"LPC2020_0"},
		},
	}, func(ctx context.Context) error {
		hrcfg, err := db.LastHRConfig(ctx)
		if err != nil {
			t.Fatalf("could not retrieve last HR cfg: %+v", err)
		}

		if got, want := hrcfg, "LPC2020_0"; got != want {
			t.Fatalf("invalid last HR cfg: got=%q, want=%q", got, want)
		}
		return nil
	})

}

func TestLastDetectorID(t *testing.T) {
	db, err := Open("fakedb")
	if err != nil {
		t.Fatalf("could not open conddb: %+v", err)
	}
	defer db.Close()

	_ = fakedb.Run(context.Background(), fakedb.Rows{
		Names: []string{"identifier"},
		Values: [][]driver.Value{
			{uint32(139)},
		},
	}, func(ctx context.Context) error {
		detid, err := db.LastDetectorID(context.Background())
		if err != nil {
			t.Fatalf("could not retrieve last det ID: %+v", err)
		}

		if got, want := detid, uint32(139); got != want {
			t.Fatalf("invalid last det ID: got=%d, want=%d", got, want)
		}
		return nil
	})

}

func TestQueryContext(t *testing.T) {
	db, err := Open("fakedb")
	if err != nil {
		t.Fatalf("could not open conddb: %+v", err)
	}
	defer db.Close()

	const queryLastDetID = "SELECT identifier FROM detectors ORDER BY datetime DESC LIMIT 1"

	_ = fakedb.Run(context.Background(), fakedb.Rows{
		Names: []string{"identifier"},
		Values: [][]driver.Value{
			{uint32(139)},
		},
	}, func(ctx context.Context) error {
		rows, err := db.QueryContext(context.Background(), queryLastDetID)
		if err != nil {
			t.Fatalf("could not execute query %q: %+v", queryLastDetID, err)
		}
		defer rows.Close()

		var detid uint32
		for rows.Next() {
			err = rows.Scan(&detid)
			if err != nil {
				t.Fatalf("could not scan det-id: %+v", err)
			}
		}

		if err := rows.Err(); err != nil {
			t.Fatalf("could not scan det-id: %+v", err)
		}

		if got, want := detid, uint32(139); got != want {
			t.Fatalf("invalid last det ID: got=%d, want=%d", got, want)
		}
		return nil
	})
}

func TestDAQStates(t *testing.T) {
	db, err := Open("fakedb")
	if err != nil {
		t.Fatalf("could not open conddb: %+v", err)
	}
	defer db.Close()

	want := []DAQState{
		{10, 20, 30, 40},
		{11, 21, 31, 41},
		{12, 22, 32, 42},
		{13, 23, 33, 43},
	}
	_ = fakedb.Run(context.Background(), fakedb.Rows{
		Names: []string{
			"identifier", "hrconfig", "rshape", "trigger_type",
		},
		Values: [][]driver.Value{
			{want[0].ID, want[0].HRConfig, want[0].RShape, want[0].TriggerMode},
			{want[1].ID, want[1].HRConfig, want[1].RShape, want[1].TriggerMode},
			{want[2].ID, want[2].HRConfig, want[2].RShape, want[2].TriggerMode},
			{want[3].ID, want[3].HRConfig, want[3].RShape, want[3].TriggerMode},
		},
	}, func(ctx context.Context) error {
		daq, err := db.DAQStates(ctx)
		if err != nil {
			t.Fatalf("could not retrieve daq states: %+v", err)
		}

		if got, want := daq, want; !reflect.DeepEqual(got, want) {
			t.Fatalf("invalid daq states:\ngot= %#v\nwant=%#v", got, want)
		}
		return nil
	})

}

func TestASICConfig(t *testing.T) {
	db, err := Open("fakedb")
	if err != nil {
		t.Fatalf("could not open conddb: %+v", err)
	}
	defer db.Close()

	want := []ASIC{
		{
			PrimaryID:    1,
			Header:       1,
			DIFID:        1,
			Razchnextval: 1,
			Razchnintval: 2,
			Trigextval:   3,
			EnTrigOut:    4,
			Trig0b:       5,
			Trig1b:       6,
			Trig2b:       7,
			SmallDAC:     8,
			B2:           9,
			B1:           10,
			B0:           11,
			Mask2:        12,
			Mask1:        13,
			Mask0:        14,
			Sw50f0:       15,
			Sw100f0:      16,
			Sw100k0:      17,
			Sw50k0:       18,
			Sw50f1:       19,
			Sw100f1:      20,
			Sw100k1:      21,
			Sw50k1:       22,
			Cmdb0fsb1:    23,
			Cmdb1fsb1:    24,
			Cmdb2fsb1:    25,
			Cmdb3fsb1:    26,
			Sw50f2:       27,
			Sw100f2:      28,
			Sw100k2:      29,
			Sw50k2:       30,
			Cmdb0fsb2:    31,
			Cmdb1fsb2:    32,
			Cmdb2fsb2:    33,
			Cmdb3fsb2:    34,
			PreAmpGain:   []byte(strings.Repeat("ab", 64)),
		},
		{
			PrimaryID:    1,
			Header:       2,
			DIFID:        11,
			Razchnextval: 11,
			Razchnintval: 12,
			Trigextval:   13,
			EnTrigOut:    14,
			Trig0b:       15,
			Trig1b:       16,
			Trig2b:       17,
			SmallDAC:     18,
			B2:           19,
			B1:           110,
			B0:           111,
			Mask2:        112,
			Mask1:        113,
			Mask0:        114,
			Sw50f0:       115,
			Sw100f0:      116,
			Sw100k0:      117,
			Sw50k0:       118,
			Sw50f1:       119,
			Sw100f1:      120,
			Sw100k1:      121,
			Sw50k1:       122,
			Cmdb0fsb1:    123,
			Cmdb1fsb1:    124,
			Cmdb2fsb1:    125,
			Cmdb3fsb1:    126,
			Sw50f2:       127,
			Sw100f2:      128,
			Sw100k2:      129,
			Sw50k2:       130,
			Cmdb0fsb2:    131,
			Cmdb1fsb2:    132,
			Cmdb2fsb2:    133,
			Cmdb3fsb2:    134,
			PreAmpGain:   []byte(strings.Repeat("bc", 64)),
		},
	}

	_ = fakedb.Run(context.Background(), fakedb.Rows{
		Names: []string{
			"identifier",
			"header",
			"dif_id",
			"razchnextval",
			"razchnintval",
			"trigextval",
			"entrigout",
			"trig0b",
			"trig1b",
			"trig2b",
			"smalldac",
			"b2",
			"b1",
			"b0",
			"mask2",
			"mask1",
			"mask0",
			"sw50f0",
			"sw100f0",
			"sw100k0",
			"sw50k0",
			"sw50f1",
			"sw100f1",
			"sw100k1",
			"sw50k1",
			"cmdb0fsb1",
			"cmdb1fsb1",
			"cmdb2fsb1",
			"cmdb3fsb1",
			"sw50f2",
			"sw100f2",
			"sw100k2",
			"sw50k2",
			"cmdb0fsb2",
			"cmdb1fsb2",
			"cmdb2fsb2",
			"cmdb3fsb2",
			"pagain",
		},
		Values: [][]driver.Value{
			{
				want[0].PrimaryID,
				want[0].Header,
				want[0].DIFID,
				want[0].Razchnextval,
				want[0].Razchnintval,
				want[0].Trigextval,
				want[0].EnTrigOut,
				want[0].Trig0b,
				want[0].Trig1b,
				want[0].Trig2b,
				want[0].SmallDAC,
				want[0].B2,
				want[0].B1,
				want[0].B0,
				want[0].Mask2,
				want[0].Mask1,
				want[0].Mask0,
				want[0].Sw50f0,
				want[0].Sw100f0,
				want[0].Sw100k0,
				want[0].Sw50k0,
				want[0].Sw50f1,
				want[0].Sw100f1,
				want[0].Sw100k1,
				want[0].Sw50k1,
				want[0].Cmdb0fsb1,
				want[0].Cmdb1fsb1,
				want[0].Cmdb2fsb1,
				want[0].Cmdb3fsb1,
				want[0].Sw50f2,
				want[0].Sw100f2,
				want[0].Sw100k2,
				want[0].Sw50k2,
				want[0].Cmdb0fsb2,
				want[0].Cmdb1fsb2,
				want[0].Cmdb2fsb2,
				want[0].Cmdb3fsb2,
				want[0].PreAmpGain,
			},
			{
				want[1].PrimaryID,
				want[1].Header,
				want[1].DIFID,
				want[1].Razchnextval,
				want[1].Razchnintval,
				want[1].Trigextval,
				want[1].EnTrigOut,
				want[1].Trig0b,
				want[1].Trig1b,
				want[1].Trig2b,
				want[1].SmallDAC,
				want[1].B2,
				want[1].B1,
				want[1].B0,
				want[1].Mask2,
				want[1].Mask1,
				want[1].Mask0,
				want[1].Sw50f0,
				want[1].Sw100f0,
				want[1].Sw100k0,
				want[1].Sw50k0,
				want[1].Sw50f1,
				want[1].Sw100f1,
				want[1].Sw100k1,
				want[1].Sw50k1,
				want[1].Cmdb0fsb1,
				want[1].Cmdb1fsb1,
				want[1].Cmdb2fsb1,
				want[1].Cmdb3fsb1,
				want[1].Sw50f2,
				want[1].Sw100f2,
				want[1].Sw100k2,
				want[1].Sw50k2,
				want[1].Cmdb0fsb2,
				want[1].Cmdb1fsb2,
				want[1].Cmdb2fsb2,
				want[1].Cmdb3fsb2,
				want[1].PreAmpGain,
			},
		},
	}, func(ctx context.Context) error {
		asics, err := db.ASICConfig(context.Background(), "LPC2020_0", 1)
		if err != nil {
			t.Fatalf("could not retrieve asics cfg: %+v", err)
		}

		if got, want := asics, want; !reflect.DeepEqual(got, want) {
			t.Fatalf("invalid asics cfg:\ngot= %#v\nwant=%#v", got, want)
		}

		for i, asic := range asics {
			got := asic.HRConfig()
			want := want[i].HRConfig()

			if !bytes.Equal(got, want) {
				t.Fatalf("invalid asics[%d] cfg", i)
			}
		}

		return nil
	})
}
