module github.com/go-lpc/mim

go 1.14

require (
	github.com/go-daq/tdaq v0.14.0
	github.com/peterh/liner v1.2.0
	github.com/ziutek/ftdi v0.0.0-20181130113013-aef9e445a2fa
	golang.org/x/net v0.0.0-20200226121028-0de0cce0169b // indirect
	golang.org/x/sys v0.0.0-20200223170610-d5e6a3e2c0ae // indirect
)

replace github.com/ziutek/ftdi => github.com/go-daq/ftdi v0.0.1
