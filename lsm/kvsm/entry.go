package kvsm

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

var (
	noContent        = "no content"
	noContentPointer = unsafe.Pointer(&noContent)
)

type entry struct {
	key      interface{}
	expireAt int64
	p        unsafe.Pointer
	prev     *entry
	next     *entry
}

func (e *entry) Expired() (ok bool) {
	if time.Now().UnixNano() > e.expireAt {
		ok = true
	}
	return
}

func (e *entry) Value(v interface{}) {
	atomic.StorePointer(&e.p, unsafe.Pointer(&v))
}

func (e *entry) GetValue() interface{} {
	v := atomic.LoadPointer(&e.p)
	return *(*interface{})(v)
}

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
	atomic.StorePointer(&e.p, noContentPointer)
	e.prev = nil
	e.next = nil
	e.key = nil
	e.expireAt = 0
	entryPool.Put(e)
}
