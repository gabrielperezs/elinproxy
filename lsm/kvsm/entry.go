package kvsm

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

var (
	noContent        interface{}
	noContentPointer = unsafe.Pointer(&noContent)
)

type entry struct {
	key      interface{}
	expireAt int64
	p        unsafe.Pointer
	prev     *entry
	next     *entry
}

// Value return true if is expired
func (e *entry) Expired() (ok bool) {
	if time.Now().UnixNano() > e.expireAt {
		ok = true
	}
	return
}

// Value store the value
func (e *entry) Value(v interface{}) {
	atomic.StorePointer(&e.p, unsafe.Pointer(&v))
}

// Value store the value
func (e *entry) ValueReset() {
	atomic.StorePointer(&e.p, noContentPointer)
}

// GetValue return the value
func (e *entry) GetValue() interface{} {
	v := atomic.LoadPointer(&e.p)
	return *(*interface{})(v)
}

// SwapValue store the new value and return the old value
func (e *entry) SwapValue(v interface{}) interface{} {
	old := atomic.SwapPointer(&e.p, unsafe.Pointer(&v))
	return *(*interface{})(old)
}

func newEntry(v interface{}) *entry {
	e := entryPool.Get().(*entry)
	e.Value(v)
	return e
}

var (
	entryPool = sync.Pool{
		New: func() interface{} {
			return &entry{}
		},
	}
)

func putEntry(e *entry) {
	e.ValueReset()
	e.prev = nil
	e.next = nil
	e.key = nil
	e.expireAt = 0
	entryPool.Put(e)
}
