package handler

import (
	"testing"
)

func TestUtilsRangeFullRange(t *testing.T) {
	r, err := parseRange("bytes=0-100000", 2048, 53712862)
	if err != nil {
		t.Error(err)
	}
	if r[0].start != 0 {
		t.Errorf("Invalid start should be 0 and is: %d", r[0].start)
	}
	t.Logf("true")
}

func TestUtilsRangePartial(t *testing.T) {
	r, err := parseRange("bytes=0-", 2048, 53712862)
	if err != nil {
		t.Error(err)
	}
	if r[0].start != 0 {
		t.Errorf("Invalid start should be 0 and is: %d", r[0].start)
	}
	t.Logf("true")
}
