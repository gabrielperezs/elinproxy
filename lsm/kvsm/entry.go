package kvsm

import (
	"sync"
	"time"
)

var (
	entryPool = sync.Pool{
		New: func() interface{} {
			return &entry{}
		},
	}
)

func putEntry(e *entry) {
	*e = entry{}
	entryPool.Put(e)
}

type entry struct {
	expireAt int64
	key      uint64
	p        interface{}
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

// Clone return a copy of the current entry removing
// the pointers to prev and next
func (e *entry) Clone() entry {
	n := entry{}
	n = *e
	n.prev = nil
	n.next = nil
	return n
}
