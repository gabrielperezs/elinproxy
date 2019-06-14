package kvsm

import (
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	actionsBuffer = 1024
)

type replaceMsg struct {
	ttl      time.Duration
	oldEntry *entry
	newEntry *entry
}

type expMsg struct {
	ttl   time.Duration
	entry *entry
}

type KVSM struct {
	items       *sync.Map
	n           int64
	replaceLLCh chan replaceMsg
	delLLCh     chan expMsg
	addLLCh     chan expMsg
	onEvicted   func(v interface{}) int64
	listTTL     map[time.Duration]*linkedListTTL
}

func New() *KVSM {
	kv := &KVSM{
		items:       &sync.Map{},
		replaceLLCh: make(chan replaceMsg, actionsBuffer),
		delLLCh:     make(chan expMsg, actionsBuffer),
		addLLCh:     make(chan expMsg, actionsBuffer),
		listTTL:     make(map[time.Duration]*linkedListTTL, 50),
	}
	go kv.workerExpiration()
	return kv
}

func (kv *KVSM) SetOnEvicted(f func(v interface{}) int64) {
	kv.onEvicted = f
}

func (kv *KVSM) Len() int {
	return int(atomic.LoadInt64(&kv.n))
}

func (kv *KVSM) Set(k uint64, v interface{}, ttl time.Duration) {
	e := entryPool.Get().(*entry)
	e.key = k
	e.expireAt = time.Now().Add(ttl).UnixNano()
	e.Value(v)
	kv.items.Store(e.key, e)
	atomic.AddInt64(&kv.n, 1)
	kv.addLLCh <- expMsg{
		ttl:   ttl,
		entry: e,
	}
}

func (kv *KVSM) Get(k interface{}) (v interface{}, expired bool, ok bool) {
	eiface, ok := kv.items.Load(k)
	if !ok {
		expired = true
		return
	}
	e, ok := eiface.(*entry)
	if !ok {
		expired = true
		return
	}
	return e.GetValue(), e.Expired(), true
}

func (kv *KVSM) Swap(k uint64, v interface{}, ttl time.Duration) bool {
	eiface, ok := kv.items.Load(k)
	if !ok {
		kv.Set(k, v, ttl)
	} else {
		old := eiface.(*entry).SwapValue(v)
		if kv.onEvicted != nil {
			kv.onEvicted(old)
		}
	}
	return true
}

func (kv *KVSM) Remove(e *entry) {
	kv.items.Delete(e.key)
	atomic.AddInt64(&kv.n, -1)
	// Delete from linked list
	kv.delLLCh <- expMsg{
		entry: e,
	}
}

func (kv *KVSM) workerExpiration() {
	nextTry := 1 * time.Second
	move := time.NewTimer(nextTry)
	for {
		select {
		case e := <-kv.delLLCh:
			if kv.onEvicted != nil {
				p := e.entry.GetValue()
				if p != nil {
					kv.onEvicted(p)
				}
			}
			putEntry(e.entry)
		case e := <-kv.addLLCh:
			if kv.listTTL[e.ttl] == nil {
				kv.listTTL[e.ttl] = &linkedListTTL{}
			}
			ce := kv.listTTL[e.ttl]
			ce.Add(e.entry)
		case <-move.C:
			move.Stop()

			keys := make([]float64, 0, len(kv.listTTL))
			for k := range kv.listTTL {
				keys = append(keys, k.Seconds())
			}
			sort.Float64s(keys)

			for _, k := range keys {
				tkd := time.Duration(k) * time.Second
				if kv.listTTL[tkd].Len() == 0 {
					continue
				}

				count := 0
				var e *entry
				for {
					count++
					if count > 1000 {
						runtime.Gosched()
					}

					e = kv.listTTL[tkd].Next(e)
					if e == nil {
						break
					}

					p := e.GetValue()
					if p == nil || e.Expired() {
						kv.Remove(e)
						kv.listTTL[tkd].Remove(e)
						continue
					}
					break
				}
			}

			move.Reset(nextTry)
		}
	}
}
