// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command eda-spy spies the content of EDA registers.
package main // import "github.com/go-lpc/mim/cmd/eda-spy"

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-lpc/mim/eda"
)

func main() {
	dev, err := eda.NewDevice("/dev/mem", 0, "")
	if err != nil {
		log.Fatalf("could open device: %+v", err)
	}
	defer dev.Close()

	fmt.Printf("------------------------------------------------\n")
	const layout = "2006-01-02 15:04:05 MST"
	fmt.Printf("%v\n", time.Now().Format(layout))

	err = dev.DumpRegisters(os.Stdout)
	if err != nil {
		log.Fatalf("could not dump registers: %+v", err)
	}
}
