package defaultstore

import (
	"sync"
	"sync/atomic"
)

type atomicMap[K comparable, V any] struct {
	value atomic.Pointer[sync.Map]
	new   func() V
	mu    sync.Mutex
}

func newAtomicMap[K comparable, V any](new func() V) *atomicMap[K, V] {
	m := &atomicMap[K, V]{new: new}
	m.Reset()
	return m
}

func (m *atomicMap[K, V]) Reset() {
	m.value.Store(new(sync.Map))
}

func (m *atomicMap[K, V]) Load(key K) (V, bool) {
	value, ok := m.value.Load().Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return value.(V), true
}

func (m *atomicMap[K, V]) LoadOrStore(key K) (V, bool) {
	if value, ok := m.Load(key); ok {
		return value, true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if value, ok := m.Load(key); ok {
		return value, true
	}
	value := m.new()
	m.value.Load().Store(key, value)
	return value, false
}
