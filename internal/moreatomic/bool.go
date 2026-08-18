package moreatomic

import "sync/atomic"

type Bool struct {
	val atomic.Uint32
}

func (b *Bool) Get() bool {
	return b.val.Load() > 0
}

func (b *Bool) Set(val bool) {
	var x = uint32(0)
	if val {
		x = 1
	}
	b.val.Store(x)
}

func (b *Bool) SetTrue() {
	b.val.Store(1)
}

func (b *Bool) SetFalse() {
	b.val.Store(0)
}

// Acquire sets bool to true if it's false and returns true, otherwise returns
// false.
func (b *Bool) Acquire() bool {
	return b.val.CompareAndSwap(0, 1)
}
