package kvsm

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func TestListAdd(t *testing.T) {
	l := &linkedListTTL{}
	for i := 0; i <= 10; i++ {
		e := &entry{p: fmt.Sprintf("Entry %d", i)}
		t.Logf("add: %v", e.p)
		l.Add(e)
	}

	var e *entry
	i := 0
	for {
		e = l.Next(e)
		if e == nil {
			return
		}
		mustBe := fmt.Sprintf("Entry %d", i)
		if e.p.(string) != mustBe {
			t.Errorf("The pos %d don't match (%v - %v)", i, e.p, mustBe)
		}
		i++
	}
}

func TestLinkedListReplace(t *testing.T) {
	l := &linkedListTTL{}
	els := make([]*entry, 10)
	for i := 0; i < 10; i++ {
		els[i] = &entry{p: fmt.Sprintf("Entry %d", i)}
		t.Logf("add: %v", els[i])
		l.Add(els[i])
	}

	l.Replace(els[0], &entry{p: fmt.Sprintf("Replaced %d", 1)})
	l.Replace(els[5], &entry{p: fmt.Sprintf("Replaced %d", 5)})
	l.Replace(els[9], &entry{p: fmt.Sprintf("Replaced %d", 9)})

	t.Logf("First: %s - Last: %s", l.root.p.(string), l.last.p.(string))
	i := 0
	var e *entry
	for {
		e = l.Next(e)
		if e == nil {
			return
		}
		t.Logf("D: %s", e.p.(string))
		i++
	}
}

func BenchmarkListAdd(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	l := &linkedListTTL{}
	var i int64
	b.Run("add", func(pb *testing.B) {
		atomic.AddInt64(&i, 1)
		e := &entry{
			p: fmt.Sprintf("Entry %d", i),
		}
		l.Add(e)
	})
	t := int(atomic.LoadInt64(&i))
	if l.Len() != t {
		b.Errorf("Invalid final result %d/%d", t, l.Len())
	}
}

func TestListAddRemove(t *testing.T) {
	l := &linkedListTTL{}
	for i := 0; i <= 5; i++ {
		e := &entry{p: fmt.Sprintf("Entry %d", i)}
		t.Logf("add: %v", e.p)
		l.Add(e)
	}

	lastLen := l.Len()

	var e *entry
	count := 0
	for {
		e = l.Prev(e)
		if e == nil {
			break
		}

		l.Remove(e)
		t.Logf("Remove: %v", e.p)
		e = nil
		count++
	}

	if lastLen != count {
		t.Errorf("Don't match the elements added and the elements deleted")
	}

}

func BenchmarkListAddRemove(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	l := &linkedListTTL{}
	var i int64
	b.Run("add", func(pb *testing.B) {
		li := atomic.AddInt64(&i, 1)
		e := &entry{p: fmt.Sprintf("Entry %d", li)}
		l.Add(e)
		l.Remove(e)
	})
	if l.Len() != 0 {
		b.Errorf("Invalid final %d", l.Len())
	}
}

func TestListAddSwap(t *testing.T) {
	l := &linkedListTTL{}
	for i := 0; i < 5; i++ {
		e := &entry{p: fmt.Sprintf("Entry %d", i)}
		l.Add(e)
	}

	i := 0
	var e *entry
	for {
		e = l.Next(e)
		if e == nil {
			break
		}

		if i == 2 {
			l.Remove(e)
			l.Add(e)
			break
		}
		i++
	}

	resultText := ""
	e = nil
	for {
		e = l.Next(e)
		if e == nil {
			break
		}

		resultText += e.p.(string)
	}

	if resultText != "Entry 0Entry 1Entry 3Entry 4Entry 2" {
		t.Errorf("Invalid order after swap: %v", resultText)
	}
}
