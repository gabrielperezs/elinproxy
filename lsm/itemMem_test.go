package lsm

import (
	"bytes"
	"fmt"
	"testing"
)

func TestLsmItemMemTempFileClose(t *testing.T) {
	body := bytes.Repeat([]byte("A"), itemMemLimitForTempFile*10)
	itm := getItemLen(0)
	itm.Write(body)
	if len(itm.Data) > 0 {
		t.Errorf("Data should be empty after write over %d: %d", itemMemLimitForTempFile, len(itm.Data))
	}
	itm.Write([]byte("\n"))
	itm.Close()
	if len(itm.Data) > 0 {
		t.Errorf("Data should be empty: %d", len(itm.Data))
	}
	if itm.w != nil {
		t.Errorf("File should be nil: %v", itm.w)
	}
}

func TestLsmItemMemTempFile(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0))
	body := bytes.Repeat([]byte("A"), 1024)
	itm := getItemLen(0)
	wrote := 0
	for i := 0; i <= 1024; i++ {
		n, _ := itm.Write(body)
		wrote += n
		n, _ = itm.Write([]byte("\n"))
		wrote += n
	}
	itm.WriteTo(buf)
	if itm.Len() != buf.Len() {
		t.Errorf("Error: %d / %d", itm.Len(), buf.Len())
	}
	buf.Reset()
	itm.WriteTo(buf)
	if itm.Len() != buf.Len() {
		t.Errorf("Error: %d / %d", itm.Len(), buf.Len())
	}
	itm.Close()
}

func TestLsmItemMemTempFileConcurr(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0))
	body := bytes.Repeat([]byte("A"), 1024)
	itm := getItemLen(0)
	wrote := 0
	for i := 0; i <= 1024; i++ {
		n, _ := itm.Write(body)
		wrote += n
		n, _ = itm.Write([]byte("\n"))
		wrote += n
	}
	itm.WriteTo(buf)
	if itm.Len() != buf.Len() {
		t.Errorf("Error: %d / %d", itm.Len(), buf.Len())
	}
	buf.Reset()
	itm.WriteTo(buf)
	if itm.Len() != buf.Len() {
		t.Errorf("Error: %d / %d", itm.Len(), buf.Len())
	}
	itm.Close()
}

func TestLsmItemMemRange(t *testing.T) {

	tests := []struct {
		content   int64
		httpRange []int64
		err       error
	}{
		{
			content:   8000,
			httpRange: nil,
			err:       nil,
		},
		{
			content:   1024,
			httpRange: []int64{0, 499},
			err:       nil,
		},
		{
			content:   1024,
			httpRange: []int64{0, 1023},
			err:       nil,
		},
		{
			content:   1024,
			httpRange: []int64{0, 1024},
			err:       nil,
		},
		{
			content:   1024,
			httpRange: []int64{0, 128},
			err:       nil,
		},
		{
			content:   1000 * 24,
			httpRange: []int64{0, 1000 * 8},
			err:       nil,
		},
		{
			content:   1024,
			httpRange: []int64{128, 256},
			err:       nil,
		},
		{
			content:   1024,
			httpRange: []int64{0, 8000},
			err:       nil,
		},
		{
			content:   int64(itemMemLimitForTempFile) + 1024,
			httpRange: []int64{0, 499},
			err:       nil,
		},
		{
			content:   int64(itemMemLimitForTempFile) + 1024,
			httpRange: []int64{0, 8000},
			err:       nil,
		},
		{
			content:   int64(itemMemLimitForTempFile) + 1024,
			httpRange: []int64{1024, 1024 * 2},
			err:       nil,
		},
	}

	for _, c := range tests {

		name := ""
		expectLen := int64(0)
		if c.httpRange == nil {
			expectLen = c.content
			name = fmt.Sprintf("Body %d - no range - Expect %d", c.content, expectLen)
		} else {
			expectLen = c.httpRange[1] - c.httpRange[0] + 1
			if expectLen > c.content {
				expectLen = c.content
			}
			name = fmt.Sprintf("Body %d - Range %d-%d - Expect %d", c.content, c.httpRange[0], c.httpRange[1], expectLen)
		}

		t.Run(name, func(t *testing.T) {

			result := bytes.NewBuffer(make([]byte, 0))
			body := bytes.Repeat([]byte("ABCDEFGHIJKMLNOPQRSTUVWXYZ0123456789"+name), int(c.content))
			body = body[0:c.content]

			itm := &ItemMem{}
			itm.Write(body)

			var n int64
			var err error
			var length int64

			from := int64(0)
			to := int64(len(body))

			if c.httpRange != nil {
				from, to, length, err = itm.ValidRange(c.httpRange[0], c.httpRange[1])
				n, err = itm.WriteToRange(result, from, to)
				if err != nil {
					if c.err == err {
						return
					}
					t.Fatal(err)
				}
			} else {
				n, err = itm.WriteTo(result)
				length = n
				if err != nil {
					if c.err == err {
						return
					}
					t.Fatal(err)
				}
			}

			if int(n) != result.Len() {
				t.Fatalf("Bytes returned %d don't match with the number of bytes %d", n, result.Len())
			}

			if result.Len() != int(length) {
				t.Fatalf("Invalid length after Validate the range %d != %d", length, result.Len())
			}

			if int64(result.Len()) != expectLen {
				t.Fatalf("Invalid result size %d expected %d", result.Len(), expectLen)
			}

			if len(body[from:to]) != int(n) {
				t.Fatalf("Expected bytes %d recived %d", len(body[from:to]), n)
			}

			if !bytes.Contains(body, result.Bytes()) {
				t.Fatalf("Total corrupted content")
			}

			if !bytes.Equal(result.Bytes(), body[from:to]) {
				t.Log("Expect", string(body[from:to]))
				t.Log("Response", string(result.Bytes()))
				t.Fatalf("Corrupt content")
			}
		})
	}
}
