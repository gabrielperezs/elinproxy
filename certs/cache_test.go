package certs

import (
	"fmt"
	"sync"
	"testing"
)

func TestCacheOneNodeLockUnlock(t *testing.T) {
	c := newStorage(nil)
	wg := sync.WaitGroup{}
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tryLock(t, c, i)
		}(i)
	}
	wg.Wait()
}

func TestCacheMultiNodeLockUnlock(t *testing.T) {
	wg := sync.WaitGroup{}
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := newStorage(nil)
			tryLock(t, c, i)
		}(i)
	}
	wg.Wait()
}

func tryLock(t *testing.T, c *Storage, n int) {
	lockName := "/lockName"
	if err := c.Lock(lockName); err != nil {
		t.Fatalf("Lock %s", err)
		return
	}

	key := fmt.Sprintf("commonKey")

	for i := 1; i <= 5; i++ {
		body := []byte(fmt.Sprintf("body%d%d", n, i))
		//t.Logf("I(%d): %s", n, key)
		c.Store(key, body)
	}

	b, err := c.Load(key)
	if err != nil {
		t.Fatalf("Load: %s", err)
	}

	expectedBody := fmt.Sprintf("body%d%d", n, 5)
	if string(b) != expectedBody {
		t.Errorf("Mutex don't work: %s != %s", string(b), expectedBody)
		return
	}
	t.Logf("OK %d | Body: %s", n, string(b))

	if err := c.Unlock(lockName); err != nil {
		t.Fatalf("Lock %s", err)
		return
	}
}
