// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conddb

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestASICRW(t *testing.T) {
	for _, tc := range []struct {
		dif uint8
	}{
		{dif: 1},
		{dif: 2},
	} {
		t.Run(fmt.Sprintf("dif=%d", tc.dif), func(t *testing.T) {
			want := loadASICs(t, tc.dif)

			for _, asic := range want[:1] {
				asic.PrimaryID = 0
				asic.DIFID = 0
				buf := asic.HRConfig()
				var got ASIC
				err := got.FromHRConfig(buf)
				if err != nil {
					t.Fatalf("could not unmarshal HR cfg: %+v", err)
				}
				if got, want := got, asic; !reflect.DeepEqual(got, want) {
					var (
						sgot  strings.Builder
						swant strings.Builder
					)
					_ = json.NewEncoder(&sgot).Encode(got)
					_ = json.NewEncoder(&swant).Encode(want)
					t.Fatalf("asic[%d]: differ\ngot:\n%s\nwant:\n%s",
						asic.Header,
						sgot.String(),
						swant.String(),
					)
				}
			}
		})
	}
}

func loadASICs(t *testing.T, dif uint8) []ASIC {
	raw, err := os.Open(fmt.Sprintf("../eda/testdata/asic-rfm-%03d.json", dif))
	if err != nil {
		t.Fatalf("could not load ASICs cfg for dif=%d: %+v", dif, err)
	}
	defer raw.Close()

	var asics []ASIC
	err = json.NewDecoder(raw).Decode(&asics)
	if err != nil {
		t.Fatalf("could not decode ASICs cfg for dif=%d: %+v", dif, err)
	}

	return asics
}
