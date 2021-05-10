// Copyright ©2021 The mim Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xbuild

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	hwlib "github.com/go-lpc/mim/internal/altera-hps"
)

const ImageName = "tomuvol-cyclone5-3.10"

func Docker() error {
	if !HasDocker() {
		return fmt.Errorf("docker not installed or unavailable")
	}

	if HasDockerImage(ImageName) {
		return nil
	}

	log.Printf("building docker image %q...", ImageName)

	dir, err := os.MkdirTemp("", "mim-xdocker-")
	if err != nil {
		return fmt.Errorf("could not create tmp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	err = mountHwLib(dir)
	if err != nil {
		return fmt.Errorf("could not mount altera-hps directory: %w", err)
	}

	err = os.WriteFile(filepath.Join(dir, "gen.go"), []byte(cgoBoot), 0644)
	if err != nil {
		return fmt.Errorf("could not create cgo x-compile bootstrap: %w", err)
	}

	cmd := exec.Command(
		"git", "clone", "--branch", "socfpga-3.10-ltsi",
		"git@github.com:sbinet-lpc/tomuvol-linux-socfpga",
		"./linux-socfpga",
	)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("could not fetch linux-socfpga sources: %w", err)
	}

	fname := filepath.Join(dir, "Dockerfile")
	err = os.WriteFile(fname, []byte(dockerImage), 0644)
	if err != nil {
		return fmt.Errorf("could not create Dockerfile: %w", err)
	}

	cmd = exec.Command("docker", "build", "-t", ImageName, ".")
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf(
			"could not build docker image %q: %w", ImageName, err,
		)
	}

	log.Printf("building docker image %q... [done]", ImageName)
	return nil
}

func HasDocker() bool {
	cmd := exec.Command("docker", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func HasDockerImage(name string) bool {
	buf := new(bytes.Buffer)
	cmd := exec.Command("docker", "images", name)
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("could not run cmd %q: %+v", cmd.Args, err)
		return false
	}

	sc := bufio.NewScanner(buf)
	if !sc.Scan() {
		return false
	}
	if !strings.HasPrefix(sc.Text(), "REPOSITORY") {
		return false
	}

	for sc.Scan() {
		txt := sc.Text()
		if strings.HasPrefix(txt, name+" ") {
			return true
		}
	}
	return false
}

const dockerImage = `
from ubuntu:12.04

run apt-get update -y
run apt-get install -y \
	bc binutils bison \
	coreutils curl \
	diffutils \
	flex \
	git \
	gcc gcc-multilib gcc-arm-linux-gnueabihf \
	make \
	socat \
	texinfo \
	u-boot-tools unzip \
	;

env GOVERSION 1.16.4
run curl -O -L https://golang.org/dl/go${GOVERSION}.linux-amd64.tar.gz && \
	tar -C /usr/local -xf go${GOVERSION}.linux-amd64.tar.gz && \
	/bin/rm ./go${GOVERSION}.linux-amd64.tar.gz
env PATH /usr/local/go/bin:$PATH

run go version

add ./gen.go        /tmp/x-cgo/gen.go

run cd /tmp/x-cgo && \
	go mod init xcgo && \
	go get github.com/go-sql-driver/mysql && \
	GOARCH=arm CC=arm-linux-gnueabihf-gcc CC_FOR_TARGET=arm-linux-gnueabihf-gcc \
	CGO_ENABLED=1 \
	go build -o /dev/null -v . && \
	cd / && \
	/bin/rm -fr /tmp/x-cgo

add ./linux-socfpga /build/linux
add ./hwlib         /build/soc_eds/ip/altera/hps/altera_hps/hwlib

env SOCEDS_DEST_ROOT /build/soc_eds

workdir /build/linux

run make CROSS_COMPILE=arm-linux-gnueabihf- ARCH=arm socfpga_defconfig
run make CROSS_COMPILE=arm-linux-gnueabihf- ARCH=arm -j8 uImage LOADADDR=0x8000
run make CROSS_COMPILE=arm-linux-gnueabihf- ARCH=arm -j8 modules
run make ARCH=arm INSTALL_MOD_PATH=/build/mnt modules_install

workdir /build
`

func mountHwLib(name string) error {
	dst := name
	src := hwlib.FS

	err := fs.WalkDir(src, "hwlib", func(path string, d fs.DirEntry, err error) error {
		name := filepath.Join(dst, path)
		switch {
		case d.Type().IsDir():
			return os.Mkdir(name, 0755)
		case d.Type().IsRegular():
			raw, err := src.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(name, raw, 0644)
		default:
			return fmt.Errorf("unknown dir-entry type %v", d.Type())
		}
	})

	if err != nil {
		log.Fatalf("could not mount %q: %+v", name, err)
	}

	return nil
}

const cgoBoot = `package main

//
import "C"

import (
	_ "log"
	_ "flag"
	_ "os"
	_ "io"
	_ "encoding/binary"
	_ "encoding/json"
	_ "net"
	_ "net/http"

	_ "github.com/go-sql-driver/mysql"
)

func main() {}
`
