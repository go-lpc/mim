// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eda

import (
	"errors"
	"io"
	"testing"
)

func TestWBuff(t *testing.T) {
	w := &wbuf{
		p: make([]byte, 8),
	}

	n, err := w.Write(make([]byte, 9))
	if err != nil {
		t.Fatalf("could not write: %+v", err)
	}
	if got, want := n, 8; got != want {
		t.Fatalf("invalid write-len: got=%d, want=%d", got, want)
	}
	if got, want := n, len(w.p); got != want {
		t.Fatalf("invalid len: got=%d, want=%d", got, want)
	}

	n, err = w.Write(make([]byte, 1))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("got=%+v, want=%+v", err, io.EOF)
	}
	if got, want := n, 0; got != want {
		t.Fatalf("got=%d, want=%d", got, want)
	}
}
