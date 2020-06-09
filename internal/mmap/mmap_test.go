// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mmap // import "github.com/go-lpc/mim/internal/mmap"

import (
	"errors"
	"os"
	"testing"
)

func TestHandle(t *testing.T) {
	t.Run("nil-handle", func(t *testing.T) {
		var h *Handle

		_, err := h.ReadAt(nil, 0)
		if !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("invalid read-at error: %+v", err)
		}

		_, err = h.WriteAt(nil, 0)
		if !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("invalid write-at error: %+v", err)
		}

		err = h.Close()
		if !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("invalid close error: %+v", err)
		}
	})
	t.Run("nil-data", func(t *testing.T) {
		var h Handle

		_, err := h.ReadAt(nil, 0)
		if !errors.Is(err, errClosed) {
			t.Fatalf("invalid read-at error: %+v", err)
		}

		_, err = h.WriteAt(nil, 0)
		if !errors.Is(err, errClosed) {
			t.Fatalf("invalid write-at error: %+v", err)
		}

		err = h.Close()
		if err != nil {
			t.Fatalf("error closing nil-data handle: %+v", err)
		}
	})
}

func TestHandleFrom(t *testing.T) {
	h := HandleFrom([]byte{0, 1, 2, 3})

	if got, want := h.Len(), 4; got != want {
		t.Fatalf("invalid len: got=%d, want=%d", got, want)
	}

	if got, want := h.At(1), byte(1); got != want {
		t.Fatalf("invalid value: got=%d, want=%d", got, want)
	}

	_, err := h.WriteAt(nil, -1)
	if got, want := err.Error(), "mmap: invalid WriteAt offset -1"; got != want {
		t.Fatalf("invalid error: %+v", err)
	}

	_, err = h.ReadAt(nil, -1)
	if got, want := err.Error(), "mmap: invalid ReadAt offset -1"; got != want {
		t.Fatalf("invalid error: %+v", err)
	}

}
