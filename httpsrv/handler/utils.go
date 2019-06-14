package handler

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// errNoBody is a sentinel error value used by failureToReadBody so we
// can detect that the lack of body was intentional.
var errNoBody = errors.New("sentinel error value")

// failureToReadBody is a io.ReadCloser that just returns errNoBody on
// Read. It's swapped in when we don't actually want to consume
// the body, but need a non-nil one, and want to distinguish the
// error from reading the dummy body.
type failureToReadBody struct{}

func (failureToReadBody) Read([]byte) (int, error) { return 0, errNoBody }
func (failureToReadBody) Close() error             { return nil }

// emptyBody is an instance of empty reader.
var emptyBody = ioutil.NopCloser(strings.NewReader(""))

// DumpResponse is like DumpRequest but dumps a response.
func DumpResponse(resp *http.Response, body bool, b io.Writer) error {
	var err error
	save := resp.Body
	savecl := resp.ContentLength

	if !body {
		// For content length of zero. Make sure the body is an empty
		// reader, instead of returning error through failureToReadBody{}.
		if resp.ContentLength == 0 {
			resp.Body = emptyBody
		} else {
			resp.Body = failureToReadBody{}
		}
	} else if resp.Body == nil {
		resp.Body = emptyBody
	} else {
		save, resp.Body, savecl, err = drainBody(resp.Body)
		if err != nil {
			log.Printf("ERROR httpsrv/handler DumpResponse/drainBody: %s", err)
			return err
		}
	}

	savecl, err = io.Copy(b, resp.Body)
	if err != nil {
		log.Printf("ERROR httpsrv/handler DumpResponse/copy: %s", err)
		return err
	}

	resp.Body = save
	resp.ContentLength = savecl
	return nil
}

// drainBody reads all of b to memory and then returns two equivalent
// ReadClosers yielding the same bytes.
//
// It returns an error if the initial slurp of all bytes fails. It does not attempt
// to make the returned ReadClosers have identical error-matching behavior.
func drainBody(b io.ReadCloser) (r1, r2 io.ReadCloser, size int64, err error) {
	if b == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		return http.NoBody, http.NoBody, 0, nil
	}
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(b); err != nil {
		return nil, b, int64(buf.Len()), err
	}
	if err = b.Close(); err != nil {
		return nil, b, int64(buf.Len()), err
	}
	return ioutil.NopCloser(&buf), ioutil.NopCloser(bytes.NewReader(buf.Bytes())), int64(buf.Len()), nil
}

// https://github.com/pkg4go/httprange/blob/bf50c5def1ce9973c14f5ae38e1560011f2a41d9/util.go
type httpRange struct {
	start  int64
	length int64
}

// Example:
//   "Range": "bytes=100-200"
//   "Range": "bytes=-50"
//   "Range": "bytes=150-"
//   "Range": "bytes=0-0,-1"
func parseRange(s string, defsize, size int64) ([]httpRange, error) {
	if s == "" || size == 0 {
		return nil, nil // header not present
	}

	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return nil, errors.New("invalid range")
	}
	var ranges []httpRange
	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = strings.TrimSpace(ra)
		if ra == "" {
			continue
		}

		var start int64
		var length int64
		length = size
		for i, pa := range strings.Split(ra, "-") {
			if len(pa) == 0 {
				continue
			}
			v, err := strconv.ParseInt(pa, 10, 64)
			if err != nil {
				return nil, errors.New("invalid range")
			}
			if i == 0 {
				start = v
			} else {
				length = v
			}
			//log.Printf("(i: %d) (pa: %v) (v: %v) | %d/%d", i, pa, v, start, length)
		}

		if start >= 0 {
			if length == -1 {
				length = start + defsize
			}
			ranges = append(ranges, httpRange{start, length})
		}
	}

	if len(ranges) == 0 {
		return nil, errors.New("no range")
	}
	return ranges, nil
}
