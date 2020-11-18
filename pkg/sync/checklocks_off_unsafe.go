// Copyright 2020 The gVisor Authors.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !checklocks

package sync

import (
	"unsafe"
)

// Don't waste space when checking disabled.
type lockRank struct{}

// SetRank sets the ranking of this mutex for lock order checking.
//
// SetRank is a no-op when lock order checking is disabled.
//
// Preconditions:
//  * Must be called before first use of the mutex.
func (m *Mutex) SetRank(rank int32) {
}

// SetRank sets the ranking of this mutex for lock order checking.
//
// SetRank is a no-op when lock order checking is disabled.
//
// Preconditions:
//  * Must be called before first use of the mutex.
func (rw *RWMutex) SetRank(rank int32) {
}

// RegisterRank registers a lock type name with specified rank.
//
// Registration is not strictly required for lock rank checking, but it
// provides naming in reports and verifies that there are no rank conflicts.

// RegisterRank is a no-op when lock order checking is disabled.
//
// Preconditions:
//  * RegisterRank is only called during initialization.
func RegisterRank(rank int, name string) {
}

func noteLock(l unsafe.Pointer, rank lockRank) {
}

func noteUnlock(l unsafe.Pointer) {
}
