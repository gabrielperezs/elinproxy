package lsm

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestLsmItemDiskReadBytes(t *testing.T) {
	message := bytes.Repeat([]byte("A"), 7*1024*1024)

	fakeVFile := VFileNew(1*time.Hour, &Config{
		Dir: os.TempDir(),
	})
	defer fakeVFile.Close()
	itd := &ItemDisk{
		VFile: fakeVFile,
	}
	fakeVFile.Write(message)
	itd.BodySize = fakeVFile.Seek()

	if !bytes.Equal(message, itd.Bytes()) {
		t.Errorf("Invalid result reading bytes")
		t.Logf("Original: %s", message)
		t.Logf("Readed: %s", itd.Bytes())
	}
}

func TestLsmItemDiskWriteTo(t *testing.T) {
	message := bytes.Repeat([]byte("A"), 7*1024*1024)
	destBuffer := bytes.NewBuffer(make([]byte, 0))

	fakeVFile := VFileNew(1*time.Hour, &Config{
		Dir: os.TempDir(),
	})
	defer fakeVFile.Close()
	itd := &ItemDisk{
		VFile: fakeVFile,
	}
	fakeVFile.Write(message)
	itd.BodySize = fakeVFile.Seek()

	n, err := itd.WriteTo(destBuffer)
	if err != nil {
		t.Errorf("Error writing: %s", err)
		return
	}

	if n != int64(len(message)) {
		t.Errorf("Invalid result, don't have the same size original %d - dest %d", len(message), n)
		return
	}

	if !bytes.Equal(message, destBuffer.Bytes()) {
		t.Errorf("Invalid result reading bytes")
		t.Logf("Original: %s", message)
		t.Logf("Readed: %s", destBuffer.Bytes())
	}
}

func BenchmarkLsmItemDiskWriteTo(b *testing.B) {

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			message := bytes.Repeat([]byte("A"), rand.Intn(20*1024*1024))
			destBuffer := bytes.NewBuffer(make([]byte, 0))

			fakeVFile := VFileNew(1*time.Hour, &Config{
				Dir: os.TempDir(),
			})
			defer fakeVFile.Close()
			itd := &ItemDisk{
				VFile: fakeVFile,
			}
			fakeVFile.Write(message)
			itd.BodySize = fakeVFile.Seek()

			n, err := itd.WriteTo(destBuffer)
			if err != nil {
				b.Errorf("Error writing: %s", err)
				return
			}

			if n != int64(len(message)) {
				b.Errorf("Invalid result, don't have the same size original %d - dest %d", len(message), n)
				return
			}

			if !bytes.Equal(message, destBuffer.Bytes()) {
				b.Errorf("Invalid result reading bytes")
				b.Logf("Original: %s", message)
				b.Logf("Readed: %s", destBuffer.Bytes())
			}
		}
	})
}

func TestLsmItemDiskRange(t *testing.T) {

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
			httpRange: []int64{0, 127},
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

			fakeVFile := VFileNew(1*time.Hour, &Config{
				Dir: os.TempDir(),
			})
			defer fakeVFile.Close()

			result := bytes.NewBuffer(make([]byte, 0))
			body := bytes.Repeat([]byte("ABCDEFGHIJKMLNOPQRSTUVWXYZ0123456789"+name), int(c.content))
			body = body[:c.content]

			itd := &ItemDisk{
				VFile: fakeVFile,
			}
			fakeVFile.Write(body)
			itd.BodySize = fakeVFile.Seek()

			var n int64
			var err error
			var length int64

			from := int64(0)
			to := int64(len(body))

			if c.httpRange != nil {
				from, to, length, err = itd.ValidRange(c.httpRange[0], c.httpRange[1])
				if err != nil {
					t.Fatalf("Invalid range: %s", err)
				}

				n, err = itd.WriteToRange(result, from, to)
				if err != nil {
					if c.err == err {
						return
					}
					t.Fatal(err)
				}
			} else {
				n, err = itd.WriteTo(result)
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

			if int64(result.Len()) != expectLen {
				t.Fatalf("Invalid result size %d expected %d", result.Len(), expectLen)
			}

			if int64(result.Len()) != length {
				t.Fatalf("Invalid length after Validate the range %d != %d", length, result.Len())
			}

			if len(body[from:to]) != int(n) {
				t.Fatalf("Expected bytes %d recived %d", len(body[from:to]), n)
			}

			if !bytes.Contains(body, result.Bytes()) {
				t.Fatalf("Corrupted content")
			}

			if !bytes.Equal(result.Bytes(), body[from:to]) {
				t.Fatalf("Returned bytes are not valid: %v", len(body[from:to]))
			}
		})
	}
}
