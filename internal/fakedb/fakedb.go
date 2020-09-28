// Copyright 2020 The go-lpc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fakedb holds types to fake an in-memory DB.
package fakedb // import "github.com/go-lpc/mim/internal/fakedb"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
)

var query struct {
	mu   sync.Mutex
	rows Rows
}

func Run(ctx context.Context, rows Rows, f func(ctx context.Context) error) error {
	query.mu.Lock()
	defer query.mu.Unlock()
	query.rows = rows

	return f(ctx)
}

func init() {
	sql.Register("fakedb", &Driver{})
}

type Driver struct{}

// Open returns a new connection to the database.
// The name is a string in a driver-specific format.
//
// Open may return a cached connection (one previously
// closed), but doing so is unnecessary; the sql package
// maintains a pool of idle connections for efficient re-use.
//
// The returned connection is only used by one goroutine at a
// time.
func (drv *Driver) Open(name string) (driver.Conn, error) {
	return &Conn{}, nil
}

type Conn struct{}

// Prepare returns a prepared statement, bound to this connection.
func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	return &Stmt{}, nil
}

// Close invalidates and potentially stops any current
// prepared statements and transactions, marking this
// connection as no longer in use.
//
// Because the sql package maintains a free pool of
// connections and only calls Close when there's a surplus of
// idle connections, it shouldn't be necessary for drivers to
// do their own connection caching.
//
// Drivers must ensure all network calls made by Close
// do not block indefinitely (e.g. apply a timeout).
func (c *Conn) Close() error {
	return nil
}

// Begin starts and returns a new transaction.
//
// Deprecated: Drivers should implement ConnBeginTx instead (or additionally).
func (c *Conn) Begin() (driver.Tx, error) {
	panic("not implemented")
}

type Stmt struct{}

// Close closes the statement.
//
// As of Go 1.1, a Stmt will not be closed if it's in use
// by any queries.
//
// Drivers must ensure all network calls made by Close
// do not block indefinitely (e.g. apply a timeout).
func (stmt *Stmt) Close() error {
	return nil
}

// NumInput returns the number of placeholder parameters.
//
// If NumInput returns >= 0, the sql package will sanity check
// argument counts from callers and return errors to the caller
// before the statement's Exec or Query methods are called.
//
// NumInput may also return -1, if the driver doesn't know
// its number of placeholders. In that case, the sql package
// will not sanity check Exec or Query argument counts.
func (stmt *Stmt) NumInput() int {
	return -1
}

// Exec executes a query that doesn't return rows, such
// as an INSERT or UPDATE.
//
// Deprecated: Drivers should implement StmtExecContext instead (or additionally).
func (stmt *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	panic("not implemented")
}

// Query executes a query that may return rows, such as a
// SELECT.
//
// Deprecated: Drivers should implement StmtQueryContext instead (or additionally).
func (stmt *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	return &query.rows, nil
}

type StmtQueryContext struct{}

func (stmt *StmtQueryContext) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	panic("not implemented")
}

type Rows struct {
	Names  []string
	Values [][]driver.Value
}

// Columns returns the names of the columns. The number of
// columns of the result is inferred from the length of the
// slice. If a particular column name isn't known, an empty
// string should be returned for that entry.
func (rows *Rows) Columns() []string {
	return rows.Names
}

// Close closes the rows iterator.
func (rows *Rows) Close() error {
	return nil
}

// Next is called to populate the next row of data into
// the provided slice. The provided slice will be the same
// size as the Columns() are wide.
//
// Next should return io.EOF when there are no more rows.
//
// The dest should not be written to outside of Next. Care
// should be taken when closing Rows not to modify
// a buffer held in dest.
func (rows *Rows) Next(dest []driver.Value) error {
	if len(rows.Values) == 0 {
		return io.EOF
	}
	copy(dest, rows.Values[0])
	rows.Values = rows.Values[1:]
	return nil
}

var (
	_ driver.Driver           = (*Driver)(nil)
	_ driver.Conn             = (*Conn)(nil)
	_ driver.Stmt             = (*Stmt)(nil)
	_ driver.StmtQueryContext = (*StmtQueryContext)(nil)
	_ driver.Rows             = (*Rows)(nil)
)
