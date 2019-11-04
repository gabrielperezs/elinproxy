package lsm

import (
	"testing"
)

func TestItemPoolBuild(t *testing.T) {
	t.Logf("Len: %d", len(itemMemPool))
}

func TestGetPutItemByLen(t *testing.T) {
	var itm *ItemMem
	testList := []int{
		128,
		1 * 1024,
		1 * 1024 * 1024,
		2 * 1024 * 1024,
		10 * 1024 * 1024,
		33 * 1024 * 1024,
		34 * 1024 * 1024,
		35 * 1024 * 1024,
	}

	for c := 0; c < 4; c++ {
		for _, i := range testList {
			itm = getItemLen(i)
			t.Logf("Req: %d - Cap: %v (%d)", i, cap(itm.Data), cap(itm.Data)/itemMemMinSize)
			putItemMem(itm)
		}
	}
}
