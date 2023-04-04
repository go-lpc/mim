// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestReadConf(t *testing.T) {
	t.Run("valid-hr", func(t *testing.T) {
		var (
			dev  Device
			hrID uint32
		)
		dev.cfg.hr.db = newDbConfig()
		dev.cfg.hr.data = dev.cfg.hr.buf[4:]

		err := dev.hrscReadConf("testdata/conf_base.csv", hrID)
		if err != nil {
			t.Fatalf("could not read config file: %+v", err)
		}

		for _, tc := range []struct {
			addr uint32
			want uint32
		}{
			{addr: 0, want: 0},
			{addr: 7, want: 0},
			{addr: 864, want: 0},
			{addr: 865, want: 1},
			{addr: 871, want: 1},
		} {
			t.Run(fmt.Sprintf("addr=%d", tc.addr), func(t *testing.T) {
				got := dev.hrscGetBit(hrID, tc.addr)
				if got != tc.want {
					t.Fatalf("invalid value: got=0x%x, want=0x%x", got, tc.want)
				}
			})
		}
	})

	tmp, err := os.MkdirTemp("", "mim-")
	if err != nil {
		t.Fatalf("could not create tmp dir: %+v", err)
	}
	defer os.RemoveAll(tmp)

	for _, tc := range []struct {
		name string
		data string
		want error
	}{
		{
			name: "invalid-format-1",
			data: `## comment
1,2,3,4,5
`,
			want: fmt.Errorf(`eda: invalid config file:2: line="1,2,3,4,5"`),
		},
		{
			name: "invalid-format-2",
			data: `## comment
1;2;3;4;5;x
`,
			want: fmt.Errorf(`eda: invalid config file:2: line="1;2;3;4;5;x"`),
		},
		{
			name: "invalid-addr",
			data: `## comment
871x;en_oc_dout1;;;1
`,
			want: fmt.Errorf(`eda: could not parse address "871x" in "871x;en_oc_dout1;;;1": strconv.ParseUint: parsing "871x": invalid syntax`),
		},
		{
			name: "invalid-bit",
			data: `## comment
871;en_oc_dout1;;;1x
`,
			want: fmt.Errorf(`eda: could not parse bit "1x" in "871;en_oc_dout1;;;1x": strconv.ParseUint: parsing "1x": invalid syntax`),
		},
		{
			name: "invalid-addr-value",
			data: `## comment
872;en_oc_dout1;;;1
`,
			want: fmt.Errorf(`eda: invalid bit address line:2: got=0x368, want=0x367`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				dev   Device
				hrID  uint32
				fname = filepath.Join(tmp, tc.name+".txt")
			)
			dev.cfg.hr.db = newDbConfig()
			dev.cfg.hr.data = dev.cfg.hr.buf[4:]

			err := os.WriteFile(fname, []byte(tc.data), 0644)
			if err != nil {
				t.Fatalf("could not create tmp file: %+v", err)
			}

			err = dev.hrscReadConf(fname, hrID)
			if err == nil {
				t.Fatalf("expected an error")
			}

			if got, want := err.Error(), tc.want.Error(); got != want {
				t.Fatalf("invalid error:\ngot= %v\nwant=%v", got, want)
			}
		})
	}
}

func TestReadConfHR(t *testing.T) {
	t.Run("valid-hr", func(t *testing.T) {
		var dev Device
		dev.cfg.hr.db = newDbConfig()
		dev.cfg.hr.data = dev.cfg.hr.buf[4:]

		err := dev.hrscReadConfHRs("testdata/hr_sc_385.csv")
		if err != nil {
			t.Fatalf("could not read hr-sc cfg: %+v", err)
		}

		for _, tc := range []struct {
			hr   uint32
			addr uint32
			want uint32
		}{
			{hr: 0, addr: 0, want: 0},
			{hr: 0, addr: 7, want: 0},
			{hr: 0, addr: 864, want: 0},
			{hr: 0, addr: 865, want: 1},
			{hr: 0, addr: 871, want: 1},
			{hr: 6, addr: 0, want: 0},
			{hr: 6, addr: 871, want: 1},
			{hr: 7, addr: 0, want: 0},
			{hr: 7, addr: 824, want: 0},
			{hr: 7, addr: 826, want: 1},
			{hr: 7, addr: 871, want: 1},
		} {
			t.Run(fmt.Sprintf("hr=%d-addr=%d", tc.hr, tc.addr), func(t *testing.T) {
				got := dev.hrscGetBit(tc.hr, tc.addr)
				if got != tc.want {
					t.Fatalf("invalid value: got=0x%x, want=0x%x", got, tc.want)
				}
			})
		}
	})

	tmp, err := os.MkdirTemp("", "mim-")
	if err != nil {
		t.Fatalf("could not create tmp dir: %+v", err)
	}
	defer os.RemoveAll(tmp)

	for _, tc := range []struct {
		name string
		data string
		want error
	}{
		{
			name: "invalid-format-1",
			data: `## comment
1,2,3
`,
			want: fmt.Errorf(`eda: invalid HR config file:2: line="1,2,3"`),
		},
		{
			name: "invalid-format-2",
			data: `## comment
1;2;3;x
`,
			want: fmt.Errorf(`eda: invalid HR config file:2: line="1;2;3;x"`),
		},
		{
			name: "invalid-hr-addr",
			data: `## comment
7x;871;1
`,
			want: fmt.Errorf(`eda: could not parse HR address "7x" in "7x;871;1": strconv.ParseUint: parsing "7x": invalid syntax`),
		},
		{
			name: "invalid-bit-addr",
			data: `## comment
7;871x;1
`,
			want: fmt.Errorf(`eda: could not parse bit address "871x" in "7;871x;1": strconv.ParseUint: parsing "871x": invalid syntax`),
		},
		{
			name: "invalid-bit-value",
			data: `## comment
7;871;1x
`,
			want: fmt.Errorf(`eda: could not parse bit value "1x" in "7;871;1x": strconv.ParseUint: parsing "1x": invalid syntax`),
		},
		{
			name: "invalid-hr-addr-value",
			data: `## comment
8;871;1
`,
			want: fmt.Errorf(`eda: invalid HR address line:2: got=8, want=7`),
		},
		{
			name: "invalid-bit-addr-value",
			data: `## comment
7;872;1
`,
			want: fmt.Errorf(`eda: invalid bit address line:2: got=872, want=871`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				dev   Device
				fname = filepath.Join(tmp, tc.name+".txt")
			)
			dev.cfg.hr.db = newDbConfig()
			dev.cfg.hr.data = dev.cfg.hr.buf[4:]

			err := os.WriteFile(fname, []byte(tc.data), 0644)
			if err != nil {
				t.Fatalf("could not create tmp file: %+v", err)
			}

			err = dev.hrscReadConfHRs(fname)
			if err == nil {
				t.Fatalf("expected an error")
			}

			if got, want := err.Error(), tc.want.Error(); got != want {
				t.Fatalf("invalid error:\ngot= %v\nwant=%v", got, want)
			}
		})
	}
}

func TestReadWriteConfHR(t *testing.T) {
	var dev Device

	dev.cfg.hr.db = newDbConfig()
	dev.cfg.hr.data = dev.cfg.hr.buf[4:]

	err := dev.hrscReadConfHRs("testdata/hr_sc_385.csv")
	if err != nil {
		t.Fatalf("could not read hr-sc cfg: %+v", err)
	}

	tmp, err := os.CreateTemp("", "mim-")
	if err != nil {
		t.Fatalf("could not create tmp file: %+v", err)
	}
	_ = tmp.Close()

	err = dev.hrscWriteConfHRs(tmp.Name())
	if err != nil {
		t.Fatalf("could not write hr-sc cfg: %+v", err)
	}

	got, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatalf("could not read back hr-sc cfg: %+v", err)
	}

	want, err := os.ReadFile("testdata/hr_sc_385.csv")
	if err != nil {
		t.Fatalf("could not read back hr-sc ref: %+v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("invalid r/w round-trip")
	}
}

func TestReadDacFloor(t *testing.T) {
	t.Run("valid-dac-file", func(t *testing.T) {
		var dev Device
		dev.cfg.hr.db = newDbConfig()
		err := dev.readThOffset("testdata/dac_floor_4rfm.csv")
		if err != nil {
			t.Fatalf("could not read config file: %+v", err)
		}

		got := dev.cfg.daq.floor
		want := [nRFM * nHR * 3]uint32{
			// RFM-0
			222, 93, 93,
			215, 92, 86,
			229, 90, 89,
			229, 95, 94,
			232, 98, 95,
			234, 100, 96,
			263, 103, 90,
			239, 100, 96,
			// RFM-1
			222, 97, 93,
			233, 94, 97,
			242, 93, 90,
			230, 93, 92,
			258, 91, 92,
			245, 95, 89,
			236, 99, 94,
			252, 92, 93,
			// RFM-2
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			// RFM-3
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
			250, 100, 96,
		}

		if got != want {
			t.Fatalf("invalid dac-floor:\ngot= %v\nwant=%v", got, want)
		}
	})

	tmp, err := os.MkdirTemp("", "mim-")
	if err != nil {
		t.Fatalf("could not create tmp dir: %+v", err)
	}
	defer os.RemoveAll(tmp)

	for _, tc := range []struct {
		name string
		data string
		want error
	}{
		{
			name: "invalid-file-fmt",
			data: `## comment
11,42
`,
			want: fmt.Errorf(`eda: invalid threshold offsets file:2: line="11,42"`),
		},
		{
			name: "invalid-file-fmt",
			data: `## comment
0;0;42
`,
			want: fmt.Errorf(`eda: invalid threshold offsets file:2: line="0;0;42"`),
		},
		{
			name: "invalid-rfm-id",
			data: `## comment
11;0;40;41;42
`,
			want: fmt.Errorf("eda: invalid RFM id=11 (line:2), want=0"),
		},
		{
			name: "invalid-rfm-value",
			data: `## comment
1.2;0;40;41;42
`,
			want: fmt.Errorf(`eda: could not parse RFM id "1.2" (line:2): strconv.ParseUint: parsing "1.2": invalid syntax`),
		},
		{
			name: "invalid-hr-id",
			data: `## comment
0;11;40;41;42
`,
			want: fmt.Errorf("eda: invalid HR id=11 (line:2), want=0"),
		},
		{
			name: "invalid-hr-value",
			data: `## comment
0;1.2;40;41;42
`,
			want: fmt.Errorf(`eda: could not parse HR id "1.2" (line:2): strconv.ParseUint: parsing "1.2": invalid syntax`),
		},
		{
			name: "invalid-dac0-value",
			data: `## comment
0;0;42.2;42;42
`,
			want: fmt.Errorf(`eda: could not parse threshold value dac0 for (RFM=0,HR=0) (line:2:"42.2"): strconv.ParseUint: parsing "42.2": invalid syntax`),
		},
		{
			name: "invalid-dac1-value",
			data: `## comment
0;0;42;42.2;42
`,
			want: fmt.Errorf(`eda: could not parse threshold value dac1 for (RFM=0,HR=0) (line:2:"42.2"): strconv.ParseUint: parsing "42.2": invalid syntax`),
		},
		{
			name: "invalid-dac2-value",
			data: `## comment
0;0;42;42;42.2
`,
			want: fmt.Errorf(`eda: could not parse threshold value dac2 for (RFM=0,HR=0) (line:2:"42.2"): strconv.ParseUint: parsing "42.2": invalid syntax`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				dev   Device
				fname = filepath.Join(tmp, tc.name+".txt")
			)
			dev.cfg.hr.db = newDbConfig()

			err := os.WriteFile(fname, []byte(tc.data), 0644)
			if err != nil {
				t.Fatalf("could not create tmp file: %+v", err)
			}

			err = dev.readThOffset(fname)
			if err == nil {
				t.Fatalf("expected an error")
			}

			if got, want := err.Error(), tc.want.Error(); got != want {
				t.Fatalf("invalid error:\ngot= %v\nwant=%v", got, want)
			}
		})
	}
}

func TestReadPreAmpGain(t *testing.T) {
	t.Run("valid-pre-amp", func(t *testing.T) {
		var dev Device
		dev.cfg.hr.db = newDbConfig()
		err := dev.readPreAmpGain("testdata/pa_gain_4rfm.csv")
		if err != nil {
			t.Fatalf("could not read config file: %+v", err)
		}

		got := dev.cfg.preamp.gains
		want := [nRFM * nHR * nChans]uint32{}
		for i := range want {
			want[i] = 255
		}

		if got != want {
			t.Fatalf("invalid preamp-gains:\ngot= %v\nwant=%v", got, want)
		}
	})

	tmp, err := os.MkdirTemp("", "mim-")
	if err != nil {
		t.Fatalf("could not create tmp dir: %+v", err)
	}
	defer os.RemoveAll(tmp)

	for _, tc := range []struct {
		name string
		data string
		want error
	}{
		{
			name: "invalid-file-fmt",
			data: `## comment
0,11,42,128
`,
			want: fmt.Errorf(`eda: invalid preamp-gain file:2: line="0,11,42,128"`),
		},
		{
			name: "invalid-file-fmt",
			data: `## comment
0;11;42,128
`,
			want: fmt.Errorf(`eda: invalid preamp-gain file:2: line="0;11;42,128"`),
		},
		{
			name: "invalid-file-fmt",
			data: `## comment
0;11;42;128;
`,
			want: fmt.Errorf(`eda: invalid preamp-gain file:2: line="0;11;42;128;"`),
		},
		{
			name: "invalid-rfm-id",
			data: `## comment
11;0;42;128
`,
			want: fmt.Errorf("eda: invalid RFM id=11 (line:2), want=0"),
		},
		{
			name: "invalid-rfm-value",
			data: `## comment
1.2;0;42;128
`,
			want: fmt.Errorf(`eda: could not parse RFM id "1.2" (line:2): strconv.ParseUint: parsing "1.2": invalid syntax`),
		},
		{
			name: "invalid-hr-id",
			data: `## comment
0;11;42;128
`,
			want: fmt.Errorf("eda: invalid HR id=11 (line:2), want=0"),
		},
		{
			name: "invalid-hr-value",
			data: `## comment
0;1.2;42;128
`,
			want: fmt.Errorf(`eda: could not parse HR id "1.2" (line:2): strconv.ParseUint: parsing "1.2": invalid syntax`),
		},
		{
			name: "invalid-ch-id",
			data: `## comment
0;0;1;128
`,
			want: fmt.Errorf("eda: invalid chan id=1 (line:2), want=0"),
		},
		{
			name: "invalid-ch-value",
			data: `## comment
0;0;1.1;128
`,
			want: fmt.Errorf(`eda: could not parse chan "1.1" (line:2): strconv.ParseUint: parsing "1.1": invalid syntax`),
		},
		{
			name: "invalid-value",
			data: `## comment
0;0;0;42.2
`,
			want: fmt.Errorf(`eda: could not parse gain for (RFM=0,HR=0,ch=0) (line:2:"42.2"): strconv.ParseUint: parsing "42.2": invalid syntax`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				dev   Device
				fname = filepath.Join(tmp, tc.name+".txt")
			)
			dev.cfg.hr.db = newDbConfig()

			err := os.WriteFile(fname, []byte(tc.data), 0644)
			if err != nil {
				t.Fatalf("could not create tmp file: %+v", err)
			}

			err = dev.readPreAmpGain(fname)
			if err == nil {
				t.Fatalf("expected an error")
			}

			if got, want := err.Error(), tc.want.Error(); got != want {
				t.Fatalf("invalid error:\ngot= %v\nwant=%v", got, want)
			}
		})
	}
}

func TestReadMask(t *testing.T) {
	t.Run("valid-mask", func(t *testing.T) {
		var dev Device
		dev.cfg.hr.db = newDbConfig()

		err := dev.readMask("testdata/mask_4rfm.csv")
		if err != nil {
			t.Fatalf("could not read config file: %+v", err)
		}

		got := dev.cfg.mask.table
		want := [nRFM * nHR * nChans]uint32{
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 6, 6, 6, 6, 6, 6, 6, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 6, 6,
			6, 6, 6, 6, 6, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
			7,
		}

		if got != want {
			t.Fatalf("invalid mask:\ngot= %v\nwant=%v", got, want)
		}
	})

	tmp, err := os.MkdirTemp("", "mim-")
	if err != nil {
		t.Fatalf("could not create tmp dir: %+v", err)
	}
	defer os.RemoveAll(tmp)

	for _, tc := range []struct {
		name string
		data string
		want error
	}{
		{
			name: "invalid-file-fmt",
			data: `## comment
0,11,42,128
`,
			want: fmt.Errorf(`eda: invalid mask file:2: line="0,11,42,128"`),
		},
		{
			name: "invalid-file-fmt",
			data: `## comment
0;11;42,128
`,
			want: fmt.Errorf(`eda: invalid mask file:2: line="0;11;42,128"`),
		},
		{
			name: "invalid-file-fmt",
			data: `## comment
0;11;42;128;
`,
			want: fmt.Errorf(`eda: invalid mask file:2: line="0;11;42;128;"`),
		},
		{
			name: "invalid-rfm-id",
			data: `## comment
11;0;42;128
`,
			want: fmt.Errorf("eda: invalid RFM id=11 (line:2), want=0"),
		},
		{
			name: "invalid-rfm-value",
			data: `## comment
1.2;0;42;128
`,
			want: fmt.Errorf(`eda: could not parse RFM id "1.2" (line:2): strconv.ParseUint: parsing "1.2": invalid syntax`),
		},
		{
			name: "invalid-hr-id",
			data: `## comment
0;11;42;128
`,
			want: fmt.Errorf("eda: invalid HR id=11 (line:2), want=0"),
		},
		{
			name: "invalid-hr-value",
			data: `## comment
0;1.2;42;128
`,
			want: fmt.Errorf(`eda: could not parse HR id "1.2" (line:2): strconv.ParseUint: parsing "1.2": invalid syntax`),
		},
		{
			name: "invalid-ch-id",
			data: `## comment
0;0;1;128
`,
			want: fmt.Errorf("eda: invalid chan id=1 (line:2), want=0"),
		},
		{
			name: "invalid-ch-value",
			data: `## comment
0;0;1.1;128
`,
			want: fmt.Errorf(`eda: could not parse chan "1.1" (line:2): strconv.ParseUint: parsing "1.1": invalid syntax`),
		},
		{
			name: "invalid-value",
			data: `## comment
0;0;0;42.2
`,
			want: fmt.Errorf(`eda: could not parse mask for (RFM=0,HR=0,ch=0) (line:2:"42.2"): strconv.ParseUint: parsing "42.2": invalid syntax`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				dev   Device
				fname = filepath.Join(tmp, tc.name+".txt")
			)
			dev.cfg.hr.db = newDbConfig()

			err := os.WriteFile(fname, []byte(tc.data), 0644)
			if err != nil {
				t.Fatalf("could not create tmp file: %+v", err)
			}

			err = dev.readMask(fname)
			if err == nil {
				t.Fatalf("expected an error")
			}

			if got, want := err.Error(), tc.want.Error(); got != want {
				t.Fatalf("invalid error:\ngot= %v\nwant=%v", got, want)
			}
		})
	}
}

func TestDAQSendDIFData(t *testing.T) {
	for _, tc := range []struct {
		name string
		conn func() net.Conn
		err  error
	}{
		{
			name: "no-header",
			conn: func() net.Conn {
				p1, p2 := net.Pipe()
				go func() {
					_ = p2.Close()
				}()
				return p1
			},
			err: fmt.Errorf("eda: could not send DIF data size header to pipe: io: read/write on closed pipe"),
		},
		{
			name: "no-data",
			conn: func() net.Conn {
				p1, p2 := net.Pipe()
				go func() {
					defer p2.Close()
					_, _ = io.ReadFull(p2, make([]byte, 8))
					_, _ = p2.Write([]byte("ACK\x00"))
				}()
				return p1
			},
			err: fmt.Errorf("eda: could not send DIF data to pipe: io: read/write on closed pipe"),
		},
		{
			name: "no-ack",
			conn: func() net.Conn {
				p1, p2 := net.Pipe()
				go func() {
					defer p2.Close()
					_, _ = io.ReadFull(p2, make([]byte, 8))
					_, _ = p2.Write([]byte("ACK\x00"))
					_, _ = io.ReadFull(p2, make([]byte, 66))
				}()
				return p1
			},
			err: fmt.Errorf("eda: could not read ACK DIF data from pipe: EOF"),
		},
		{
			name: "invalid-hdr-ack",
			conn: func() net.Conn {
				p1, p2 := net.Pipe()
				go func() {
					defer p2.Close()
					_, _ = io.ReadFull(p2, make([]byte, 8))
					_, _ = p2.Write([]byte("ACQ\x00"))
				}()
				return p1
			},
			err: fmt.Errorf("eda: invalid ACK DIF header from pipe: \"ACQ\\x00\""),
		},
		{
			name: "invalid-data-ack",
			conn: func() net.Conn {
				p1, p2 := net.Pipe()
				go func() {
					defer p2.Close()
					_, _ = io.ReadFull(p2, make([]byte, 8))
					_, _ = p2.Write([]byte("ACK\x00"))
					_, _ = io.ReadFull(p2, make([]byte, 66))
					_, _ = p2.Write([]byte("ACQ\x00"))
				}()
				return p1
			},
			err: fmt.Errorf("eda: invalid ACK DIF data from pipe: \"ACQ\\x00\""),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dev := &Device{
				msg: log.New(io.Discard, "eda: ", 0),
				buf: make([]byte, 4),
			}
			sck := tc.conn()
			dev.daq.rfm = []rfmSink{
				{
					w: &wbuf{
						p: make([]byte, daqBufferSize),
						c: 66,
					},
					buf: make([]byte, 8),
					sck: sck,
				},
			}
			err := dev.daqSendDIFData(0)
			switch {
			case err == nil && tc.err == nil:
				// ok.
				return
			case err == nil && tc.err != nil:
				t.Fatalf("expected an error (%v)", tc.err)
			case err != nil && tc.err == nil:
				t.Fatalf("could not send DIF data: %+v", err)
			case err != nil && tc.err != nil:
				if got, want := err.Error(), tc.err.Error(); got != want {
					t.Fatalf("invalid error:\ngot= %+v\nwant=%+v", got, want)
				}
			}
		})
	}
}
