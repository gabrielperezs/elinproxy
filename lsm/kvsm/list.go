package kvsm

import "log"

type linkedListTTL struct {
	root *entry
	last *entry
	size int
}

func (el *linkedListTTL) Prev(e *entry) *entry {
	if e == nil {
		return el.last
	}
	return e.prev
}

func (el *linkedListTTL) Next(e *entry) *entry {
	if e == nil {
		return el.root
	}
	return e.next
}

func (el *linkedListTTL) Len() int {
	return el.size
}

// Add will add an element to the linked list
// IMPORTANT: Not safe to use with concurrency
func (el *linkedListTTL) Add(e *entry) {
	el.size++
	currLast := el.last
	if el.root == nil {
		el.root = e
		e.prev = nil
		e.next = nil
	} else {
		e.prev = currLast
		e.next = nil
		currLast.next = e
	}
	el.last = e
}

func (el *linkedListTTL) Replace(o *entry, n *entry) {
	n.prev = o.prev
	n.next = o.next
	if n.prev == nil {
		el.root = n
	} else {
		n.prev.next = n
	}
	if n.next == nil {
		el.last = n
	} else {
		n.next.prev = n
	}
}

func (el *linkedListTTL) Remove(e *entry) {
	defer func() {
		e.prev = nil
		e.next = nil
	}()

	if el.size == 0 {
		log.Panic("Trying to remove an entry from a linkedList without elements")
		return
	}

	el.size--
	if el.size == 0 {
		el.root = nil
		el.last = nil
		return
	}

	if el.root == e {
		el.root = e.next
		return
	}

	if el.last == e {
		e.next = nil
		el.last = e.prev
		return
	}

	l := e.prev
	r := e.next
	if l == nil {
		el.root = l
	} else {
		l.next = r
	}

	if r == nil {
		el.last = l
	} else {
		r.prev = l
	}
}
