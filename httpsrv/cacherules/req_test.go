package cacherules

import (
	"net/http"
	"net/url"
	"testing"
)

func TestNoReqContains(t *testing.T) {
	cr := InternalRules{
		NoReqPathContains: []string{"/feed/"},
	}

	req := &http.Request{}
	req.Method = http.MethodGet
	req.URL, _ = url.Parse("http://www.example.com/feed/")

	if cr.IsReqCachable(req) {
		t.Errorf("should not be cachable")
	}
}

func TestNoReqPrefix(t *testing.T) {
	cr := InternalRules{
		NoReqPathPrefix: []string{"/feed/"},
	}

	req := &http.Request{}
	req.Method = http.MethodGet
	req.URL, _ = url.Parse("http://www.example.com/feed/")

	if cr.IsReqCachable(req) {
		t.Errorf("should not be cachable")
	}
}

func TestNoReqSuffix(t *testing.T) {
	cr := InternalRules{
		NoReqPathSuffix: []string{"/feed/"},
	}

	req := &http.Request{}
	req.Method = http.MethodGet
	req.URL, _ = url.Parse("http://www.example.com/feed/")

	if cr.IsReqCachable(req) {
		t.Errorf("should not be cachable")
	}
}

func TestNoReqAll(t *testing.T) {
	cr := InternalRules{
		NoReqPathPrefix: []string{"/"},
	}

	req := &http.Request{}
	req.Method = http.MethodGet

	urls := []string{
		"http://www.example.com/",
		"http://www.example.com/feed/",
		"http://www.example.com/image/test.jpg",
	}

	for _, u := range urls {
		req.URL, _ = url.Parse(u)
		if cr.IsReqCachable(req) {
			t.Errorf("should not be cachable: %s", u)
		}
	}
}
