// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conddb // import "github.com/go-lpc/mim/conddb"

import (
	"fmt"
	"math"
	"strconv"
)

const (
	numASICs = math.MaxUint8
)

type ASIC struct {
	PrimaryID    int32  `json:"identifier"`
	Header       uint8  `json:"header"`
	DIFID        uint8  `json:"dif_id"`
	Razchnextval uint8  `json:"razchnextval"`
	Razchnintval uint8  `json:"razchnintval"`
	Trigextval   uint8  `json:"trigextval"`
	EnTrigOut    uint8  `json:"entrigout"`
	Trig0b       uint8  `json:"trig0b"`
	Trig1b       uint8  `json:"trig1b"`
	Trig2b       uint8  `json:"trig2b"`
	SmallDAC     uint8  `json:"smalldac"`
	B2           int16  `json:"b2"`
	B1           int16  `json:"b1"`
	B0           int16  `json:"b0"`
	Mask2        uint64 `json:"mask2"`
	Mask1        uint64 `json:"mask1"`
	Mask0        uint64 `json:"mask0"`
	Sw50f0       uint8  `json:"sw50f0"`
	Sw100f0      uint8  `json:"sw100f0"`
	Sw100k0      uint8  `json:"sw100k0"`
	Sw50k0       uint8  `json:"sw50k0"`
	Sw50f1       uint8  `json:"sw50f1"`
	Sw100f1      uint8  `json:"sw100f1"`
	Sw100k1      uint8  `json:"sw100k1"`
	Sw50k1       uint8  `json:"sw50k1"`
	Cmdb0fsb1    uint8  `json:"cmdb0fsb1"`
	Cmdb1fsb1    uint8  `json:"cmdb1fsb1"`
	Cmdb2fsb1    uint8  `json:"cmdb2fsb1"`
	Cmdb3fsb1    uint8  `json:"cmdb3fsb1"`
	Sw50f2       uint8  `json:"sw50f2"`
	Sw100f2      uint8  `json:"sw100f2"`
	Sw100k2      uint8  `json:"sw100k2"`
	Sw50k2       uint8  `json:"sw50k2"`
	Cmdb0fsb2    uint8  `json:"cmdb0fsb2"`
	Cmdb1fsb2    uint8  `json:"cmdb1fsb2"`
	Cmdb2fsb2    uint8  `json:"cmdb2fsb2"`
	Cmdb3fsb2    uint8  `json:"cmdb3fsb2"`
	PreAmpGain   []byte `json:"pagain"`
}

func (asic ASIC) HRConfig() []byte {
	const (
		nHR         = 8
		nBytesCfgHR = 109
	)
	var (
		buf = make([]byte, nHR*nBytesCfgHR)
		i   = 0
		o   = func(v uint8) {
			buf[i] = v
			i++
		}
		err = asic.marshal(o)
	)
	if err != nil {
		panic(err)
	}

	return buf
}

func (asic *ASIC) FromHRConfig(buf []byte) error {
	const (
		nHR         = 8
		nBytesCfgHR = 109
	)
	if got, want := len(buf), nHR*nBytesCfgHR; got != want {
		return fmt.Errorf("conddb: invalid ASIC hr-config buffer length (got=%d, want=%d)", got, want)
	}
	var (
		i = 0
		o = func() uint8 {
			v := buf[i]
			i++
			return v
		}
		err = asic.unmarshal(o)
	)
	return err
}

// func (asic ASIC) ToCSV(w io.Writer) error {
// 	const (
// 		nHR         = 8
// 		nBytesCfgHR = 109
// 		nChans      = 64
// 	)
// 	var (
// 		i = nHR*nBytesCfgHR - 1
// 		o = func(v uint8) {
// 			fmt.Fprintf(w, "%d;%d\n", i, v)
// 			i--
// 		}
// 	)
// 	return asic.marshal(o)
// }

func (asic ASIC) marshal(o func(v uint8)) error {
	const nChans = 64

	// 871
	o(asicEnOcDout1b)
	o(asicEnOcDout2b)
	o(asicEnOcTransmitOn1b)
	o(asicEnOcTransmitOn2b)
	o(asicEnOcChipsAtb)
	o(asicSelStartReadout)
	o(asicSelEndReadout)
	o(0)

	// 863
	o(0)
	o(0)
	o(0)
	o(asicClkMux)
	o(asicScOn)
	o(asic.Razchnextval)
	o(asic.Razchnintval)
	o(asic.Trigextval)

	// 855
	o(asicDiscrOrOr)
	o(asic.EnTrigOut)
	o(asic.Trig0b)
	o(asic.Trig1b)
	o(asic.Trig2b)
	o(asicOtaBgSwitch)
	o(asicDACSwitch)
	o(asic.SmallDAC)

	// 847
	o(bitI16(asic.B2, 9))
	o(bitI16(asic.B2, 8))
	o(bitI16(asic.B2, 7))
	o(bitI16(asic.B2, 6))
	o(bitI16(asic.B2, 5))
	o(bitI16(asic.B2, 4))
	o(bitI16(asic.B2, 3))
	o(bitI16(asic.B2, 2))

	// 839
	o(bitI16(asic.B2, 1))
	o(bitI16(asic.B2, 0))
	o(bitI16(asic.B1, 9))
	o(bitI16(asic.B1, 8))
	o(bitI16(asic.B1, 7))
	o(bitI16(asic.B1, 6))
	o(bitI16(asic.B1, 5))
	o(bitI16(asic.B1, 4))

	// 831
	o(bitI16(asic.B1, 3))
	o(bitI16(asic.B1, 2))
	o(bitI16(asic.B1, 1))
	o(bitI16(asic.B1, 0))
	o(bitI16(asic.B0, 9))
	o(bitI16(asic.B0, 8))
	o(bitI16(asic.B0, 7))
	o(bitI16(asic.B0, 6))

	// 823
	o(bitI16(asic.B0, 5))
	o(bitI16(asic.B0, 4))
	o(bitI16(asic.B0, 3))
	o(bitI16(asic.B0, 2))
	o(bitI16(asic.B0, 1))
	o(bitI16(asic.B0, 0))
	o(bitU8(asic.Header, 7))
	o(bitU8(asic.Header, 6))

	// 815
	o(bitU8(asic.Header, 5))
	o(bitU8(asic.Header, 4))
	o(bitU8(asic.Header, 3))
	o(bitU8(asic.Header, 2))
	o(bitU8(asic.Header, 1))
	o(bitU8(asic.Header, 0))

	for ii := uint8(0); ii < nChans; ii++ {
		o(bitU64(asic.Mask2, ii)) // 63
		o(bitU64(asic.Mask1, ii)) // 63
		o(bitU64(asic.Mask0, ii)) // 63
	}

	o(asicRsOrDiscri)
	o(asicDiscri1)

	// 615
	o(asicDiscri2)
	o(asicDiscri0)
	o(asicOtaBgSwitch)
	o(asicEnOtaq)
	o(asic.Sw50f0)
	o(asic.Sw100f0)
	o(asic.Sw100k0)
	o(asic.Sw50k0)

	// 607
	o(asicPowerOnFsb1)
	o(asicPowerOnFsb2)
	o(asicPowerOnFsb0)
	o(asicSel1)
	o(asicSel0)
	o(asic.Sw50f1)
	o(asic.Sw100f1)
	o(asic.Sw100k1)

	// 599
	o(asic.Sw50k1)
	o(asic.Cmdb0fsb1)
	o(asic.Cmdb1fsb1)
	o(asic.Cmdb2fsb1)
	o(asic.Cmdb3fsb1)
	o(asic.Sw50f2)
	o(asic.Sw100f2)
	o(asic.Sw100k2)

	// 591
	o(asic.Sw50k2)
	o(asic.Cmdb0fsb2)
	o(asic.Cmdb1fsb2)
	o(asic.Cmdb2fsb2)
	o(asic.Cmdb3fsb2)
	o(asicPowerOnW)
	o(asicPowerOnSs)
	o(asicPowerOnBuff)

	// 583
	o(bitU8(asicSwSsc, 2))
	o(bitU8(asicSwSsc, 1))
	o(bitU8(asicSwSsc, 0))
	o(asicCmdB3ss)
	o(asicCmdB2ss)
	o(asicCmdB1ss)
	o(asicCmdB0ss)
	o(asicPowerOnPreAmp)

	// 575
	for i := 0; i < len(asic.PreAmpGain); i += 2 {
		vv, err := strconv.ParseUint(string(asic.PreAmpGain[i:i+2]), 16, 8)
		if err != nil {
			return fmt.Errorf("conddb: could not convert %q: %+v", asic.PreAmpGain[i:i+2], err)
		}
		v := uint8(vv)
		o(bitU8(v, 7))
		o(bitU8(v, 6))
		o(bitU8(v, 5))
		o(bitU8(v, 4))
		o(bitU8(v, 3))
		o(bitU8(v, 2))
		o(bitU8(v, 1))
		o(bitU8(v, 0))
	}

	// 63 - ctest
	for i := 0; i < nChans; i++ {
		o(0) // 0 for acquisition mode. 1 for calibration tests.
	}

	return nil
}

func (asic *ASIC) unmarshal(o func() uint8) error {
	const nChans = 64

	// 871
	_ = o() // asicEnOcDout1b
	_ = o() // asicEnOcDout2b
	_ = o() // asicEnOcTransmitOn1b
	_ = o() // asicEnOcTransmitOn2b
	_ = o() // asicEnOcChipsAtb
	_ = o() // asicSelStartReadout
	_ = o() // asicSelEndReadout
	_ = o() // 0

	// 863
	_ = o() // 0
	_ = o() // 0
	_ = o() // 0
	_ = o() // asicClkMux
	_ = o() // asicScOn
	asic.Razchnextval = o()
	asic.Razchnintval = o()
	asic.Trigextval = o()

	// 855
	_ = o()              // asicDiscrOrOr
	asic.EnTrigOut = o() //
	asic.Trig0b = o()    //
	asic.Trig1b = o()    //
	asic.Trig2b = o()    //
	_ = o()              // asicOtaBgSwitch
	_ = o()              // asicDACSwitch
	asic.SmallDAC = o()  //

	// 847
	setBitI16(&asic.B2, 9, o())
	setBitI16(&asic.B2, 8, o())
	setBitI16(&asic.B2, 7, o())
	setBitI16(&asic.B2, 6, o())
	setBitI16(&asic.B2, 5, o())
	setBitI16(&asic.B2, 4, o())
	setBitI16(&asic.B2, 3, o())
	setBitI16(&asic.B2, 2, o())

	// 839
	setBitI16(&asic.B2, 1, o())
	setBitI16(&asic.B2, 0, o())
	setBitI16(&asic.B1, 9, o())
	setBitI16(&asic.B1, 8, o())
	setBitI16(&asic.B1, 7, o())
	setBitI16(&asic.B1, 6, o())
	setBitI16(&asic.B1, 5, o())
	setBitI16(&asic.B1, 4, o())

	// 831
	setBitI16(&asic.B1, 3, o())
	setBitI16(&asic.B1, 2, o())
	setBitI16(&asic.B1, 1, o())
	setBitI16(&asic.B1, 0, o())
	setBitI16(&asic.B0, 9, o())
	setBitI16(&asic.B0, 8, o())
	setBitI16(&asic.B0, 7, o())
	setBitI16(&asic.B0, 6, o())

	// 823
	setBitI16(&asic.B0, 5, o())
	setBitI16(&asic.B0, 4, o())
	setBitI16(&asic.B0, 3, o())
	setBitI16(&asic.B0, 2, o())
	setBitI16(&asic.B0, 1, o())
	setBitI16(&asic.B0, 0, o())
	setBitU8(&asic.Header, 7, o())
	setBitU8(&asic.Header, 6, o())

	// 815
	setBitU8(&asic.Header, 5, o())
	setBitU8(&asic.Header, 4, o())
	setBitU8(&asic.Header, 3, o())
	setBitU8(&asic.Header, 2, o())
	setBitU8(&asic.Header, 1, o())
	setBitU8(&asic.Header, 0, o())

	for ii := uint8(0); ii < nChans; ii++ {
		setBitU64(&asic.Mask2, ii, o()) // 63
		setBitU64(&asic.Mask1, ii, o()) // 63
		setBitU64(&asic.Mask0, ii, o()) // 63
	}

	_ = o() // asicRsOrDiscri
	_ = o() // asicDiscri1

	// 615
	_ = o() // asicDiscri2
	_ = o() // asicDiscri0
	_ = o() // asicOtaBgSwitch
	_ = o() // asicEnOtaq
	asic.Sw50f0 = o()
	asic.Sw100f0 = o()
	asic.Sw100k0 = o()
	asic.Sw50k0 = o()

	// 607
	_ = o() // asicPowerOnFsb1
	_ = o() // asicPowerOnFsb2
	_ = o() // asicPowerOnFsb0
	_ = o() // asicSel1
	_ = o() // asicSel0
	asic.Sw50f1 = o()
	asic.Sw100f1 = o()
	asic.Sw100k1 = o()

	// 599
	asic.Sw50k1 = o()
	asic.Cmdb0fsb1 = o()
	asic.Cmdb1fsb1 = o()
	asic.Cmdb2fsb1 = o()
	asic.Cmdb3fsb1 = o()
	asic.Sw50f2 = o()
	asic.Sw100f2 = o()
	asic.Sw100k2 = o()

	// 591
	asic.Sw50k2 = o()
	asic.Cmdb0fsb2 = o()
	asic.Cmdb1fsb2 = o()
	asic.Cmdb2fsb2 = o()
	asic.Cmdb3fsb2 = o()
	_ = o() // asicPowerOnW
	_ = o() // asicPowerOnSs
	_ = o() // asicPowerOnBuff

	// 583
	_ = o() // asicSwSsc bit-2
	_ = o() // asicSwSsc bit-1
	_ = o() // asicSwSsc bit-0
	_ = o() // asicCmdB3ss
	_ = o() // asicCmdB2ss
	_ = o() // asicCmdB1ss
	_ = o() // asicCmdB0ss
	_ = o() // asicPowerOnPreAmp

	// 575
	asic.PreAmpGain = make([]byte, 2*nChans)
	for i := 0; i < len(asic.PreAmpGain); i += 2 {
		v7 := o()
		v6 := o()
		v5 := o()
		v4 := o()
		v3 := o()
		v2 := o()
		v1 := o()
		v0 := o()

		bit := v7<<7 | v6<<6 | v5<<5 | v4<<4 |
			v3<<3 | v2<<2 | v1<<1 | v0<<0

		str := strconv.FormatUint(uint64(bit), 16)
		copy(asic.PreAmpGain[i:i+2], []byte(str))
	}

	// 63 - ctest
	for i := 0; i < nChans; i++ {
		_ = o() // 0 for acquisition mode. 1 for calibration tests.
	}

	return nil
}

func bitU64(v uint64, pos uint8) uint8 {
	o := v & (1 << uint64(pos))
	if o == 0 {
		return 0
	}
	return 1
}

func bitI16(v int16, pos uint8) uint8 {
	o := v & (1 << int16(pos))
	if o == 0 {
		return 0
	}
	return 1
}

func bitU8(v, pos uint8) uint8 {
	o := v & (1 << pos)
	if o == 0 {
		return 0
	}
	return 1
}

func setBitU64(w *uint64, pos, v uint8) {
	mask := ^(uint64(1) << pos)
	*w &= uint64(mask)
	*w |= uint64(v) << uint64(pos)
}

func setBitI16(w *int16, pos, v uint8) {
	mask := ^(uint16(1) << pos)
	*w &= int16(mask)
	*w |= int16(v) << pos
}

func setBitU8(w *uint8, pos, v uint8) {
	mask := ^(uint8(1) << pos)
	*w &= uint8(mask)
	*w |= uint8(v) << pos
}

const (
	asicEnOcDout1b       = 0x1
	asicEnOcDout2b       = 0x0
	asicEnOcTransmitOn1b = 0x1
	asicEnOcTransmitOn2b = 0x0
	asicEnOcChipsAtb     = 0x1
	asicSelStartReadout  = 0x1
	asicSelEndReadout    = 0x1
	asicClkMux           = 0x1
	asicScOn             = 0x1
	//	asicRazChanExtVal    = 0x0
	//	asicRazChanIntVal    = 0x1
	//	asicTrigExtVal       = 0x0
	asicDiscrOrOr = 0x1
	//	asicEnTrigOut        = 0x1
	//	asicTrig0b           = 0x1
	//	asicTrig1b           = 0x0
	//	asicTrig2b           = 0x0
	asicOtaBgSwitch = 0x1
	asicDACSwitch   = 0x1
	//	asicSmallDAC         = 0x0

	//	asicB2  = 0xB4
	//	asicB1  = 0xB4
	//  asicB0  = 0xB4
	//  asicHdr = 0x18 // asic id 1 to 24

	// asicMask2         = 0xFFFFFFFFFFFFFFFF
	// asicMask1         = 0xFFFFFFFFFFFFFFFF
	// asicMask0         = 0xFFFFFFFFFFFFFFFF
	asicRsOrDiscri = 0x1
	asicDiscri0    = 0x1
	asicDiscri1    = 0x1
	asicDiscri2    = 0x1
	// asicOtaqPowerADC  = 0x1
	asicEnOtaq = 0x0
	// asicSw50f0        = 0x1
	// asicSw100f0       = 0x1
	// asicSw100k0       = 0x1
	// asicSw50k0        = 0x1
	asicPowerOnFsb0 = 0x1
	asicPowerOnFsb1 = 0x1
	asicPowerOnFsb2 = 0x1
	asicSel1        = 0x0
	asicSel0        = 0x1
	// asicSw50f1        = 0x1
	// asicSw100f1       = 0x1
	// asicSw100k1       = 0x1
	// asicSw50k1        = 0x1
	// asicCmdB0fsb1     = 0x1
	// asicCmdB1fsb1     = 0x1
	// asicCmdB2fsb1     = 0x0
	// asicCmdB3fsb1     = 0x1
	// asicSw50f2        = 0x1
	// asicSw100f2       = 0x1
	// asicSw100k2       = 0x1
	// asicSw50k2        = 0x1
	// asicCmdB0fsb2     = 0x1
	// asicCmdB1fsb2     = 0x1
	// asicCmdB2fsb2     = 0x0
	// asicCmdB3fsb2     = 0x1
	asicPowerOnW      = 0x1
	asicPowerOnSs     = 0x0
	asicPowerOnBuff   = 0x1
	asicSwSsc         = 0x7
	asicCmdB0ss       = 0x0
	asicCmdB1ss       = 0x0
	asicCmdB2ss       = 0x0
	asicCmdB3ss       = 0x0
	asicPowerOnPreAmp = 0x1
)
