// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mmap // import "github.com/go-lpc/mim/internal/mmap"

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

var (
	errClosed = errors.New("mmap: closed")
)

type Handle struct {
	data []byte
}

func HandleFrom(data []byte) *Handle {
	h := &Handle{data: data}
	runtime.SetFinalizer(h, (*Handle).Close)
	return h
}

// Close closes the mmap handle.
func (h *Handle) Close() error {
	if h == nil {
		return os.ErrInvalid
	}

	if h.data == nil {
		return nil
	}
	data := h.data
	h.data = nil
	runtime.SetFinalizer(h, nil)

	return unix.Munmap(data)
}

// Len returns the length of the underlying memory-mapped file.
func (h *Handle) Len() int {
	return len(h.data)
}

// At returns the byte at index i.
func (h *Handle) At(i int) byte {
	return h.data[i]
}

// ReadAt implements the io.ReaderAt interface.
func (h *Handle) ReadAt(p []byte, off int64) (int, error) {
	if h == nil {
		return 0, os.ErrInvalid
	}

	if h.data == nil {
		return 0, errClosed
	}
	if off < 0 || int64(len(h.data)) < off {
		return 0, fmt.Errorf("mmap: invalid ReadAt offset %d", off)
	}
	n := copy(p, h.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// WriteAt implements the io.WriterAt interface.
func (h *Handle) WriteAt(p []byte, off int64) (int, error) {
	if h == nil {
		return 0, os.ErrInvalid
	}

	if h.data == nil {
		return 0, errClosed
	}
	if off < 0 || int64(len(h.data)) < off {
		return 0, fmt.Errorf("mmap: invalid WriteAt offset %d", off)
	}
	n := copy(h.data[off:], p)
	if n < len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

var (
	_ io.ReaderAt = (*Handle)(nil)
	_ io.WriterAt = (*Handle)(nil)
	_ io.Closer   = (*Handle)(nil)
)
