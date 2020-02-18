// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mim holds code for the MIM DAQ.
package mim // import "github.com/go-lpc/mim"

import (
	"fmt"
	"runtime/debug"
)

// Version returns the version of mim and its checksum.
// The returned values are only valid in binaries built with module support.
func Version() (version, sum string) {
	b, ok := debug.ReadBuildInfo()
	if !ok {
		return "", ""
	}
	return versionOf(b)
}

func versionOf(b *debug.BuildInfo) (version, sum string) {
	if b == nil {
		return "", ""
	}

	const root = "github.com/go-lpc/mim"
	for _, m := range b.Deps {
		if m.Path != root {
			continue
		}
		if m.Replace != nil {
			switch {
			case m.Replace.Version != "" && m.Replace.Path != "":
				return fmt.Sprintf("%s %s", m.Replace.Path, m.Replace.Version), m.Replace.Sum
			case m.Replace.Version != "":
				return m.Replace.Version, m.Replace.Sum
			case m.Replace.Path != "":
				return m.Replace.Path, m.Replace.Sum
			default:
				return m.Version + "*", ""
			}
		}
		return m.Version, m.Sum
	}
	return "", ""
}
