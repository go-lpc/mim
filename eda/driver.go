// Copyright 2021 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

type driver interface {
	hrscSetBit(hr, addr, bit uint32)
	hrscGetBit(hr, addr uint32) uint32
	hrscCopyConf(hrDst, hrSrc uint32)
	hrscResetReadRegisters(rfm int) error
	hrscSetConfig(rfm int) error
	hrscSelectSlowControl() error
	hrscSelectReadRegister() error
	hrscResetSC() error
	hrscStartSC(rfm int) error
	hrscSCDone(rfm int) bool
}

var _ driver = (*board)(nil)
