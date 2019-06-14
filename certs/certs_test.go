package certs

import (
	"crypto/tls"
	"crypto/x509"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	c = New(&Config{})
)

func TestCertsStore(t *testing.T) {
	c := New(&Config{})
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Store(tls.Certificate{
			Leaf: &x509.Certificate{
				NotAfter: time.Now().Add(24 * time.Hour),
				DNSNames: []string{
					"elinproxy.com",
					"www.elinproxy.com",
					"*.stage.elinproxy.com",
				},
			},
		})
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Store(tls.Certificate{
			Leaf: &x509.Certificate{
				NotAfter: time.Now().Add(24 * time.Hour),
				DNSNames: []string{
					"testimg.com",
					"www.testing.com",
				},
			},
		})
	}()
	wg.Wait()

	if len(c.certs) != 4 {
		t.Errorf("Certs: %+v", c.certs)
	}

	if len(c.certsWC) != 1 {
		t.Errorf("Certs: %+v", c.certs)
	}
}

func TestCertsFind(t *testing.T) {
	c := New(&Config{})
	c.Store(tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: time.Now().Add(24 * time.Hour),
			DNSNames: []string{"elinproxy.com", "www.elinproxy.com", "*.stage.elinproxy.com"},
		},
	})
	c.Store(tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: time.Now().Add(24 * time.Hour),
			DNSNames: []string{"testing.com", "www.testing.com"},
		},
	})

	for _, domain := range []string{"testing.com", "invalid.testing.com", "valid.stage.elinproxy.com"} {
		_, err := c.Get(domain)
		if err != nil {
			if domain == "invalid.testing.com" && err == ErrCertNotFound {
				t.Logf("Correct result: %s (%s)", domain, ErrCertNotFound)
				continue
			}
			t.Errorf("Err in cert %s: %s", domain, err)
			continue
		}
		t.Logf("Correct result: %s", domain)
	}
}

func TestCertsFindExpired(t *testing.T) {
	c := New(&Config{})
	c.Store(tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: time.Now().Add(24 * time.Hour * 30 * 3),
			DNSNames: []string{"valid.com", "www.valid.com"},
		},
	})
	c.Store(tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: time.Now().Add(-1 * time.Hour * 24 * 15),
			DNSNames: []string{"expired.com", "www.expired.com"},
		},
	})

	for _, domain := range []string{"expired.com", "valid.com"} {
		foundCert, err := c.Get(domain)
		if domain == "expired.com" {
			if err != nil {
				t.Logf("Correct expired: %s %s", domain, foundCert.Leaf.NotAfter)
				continue
			}
			t.Errorf("Err in cert %s: %s", domain, foundCert.Leaf.NotAfter)
			continue
		}
		if err != nil {
			t.Errorf("Err in cert %s: %s", domain, err)
			continue
		}
		t.Logf("Correct result: %s %s", domain, foundCert.Leaf.NotAfter)
	}
}

func TestCertsStoreDelete(t *testing.T) {
	c := New(&Config{})

	c.Store(tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: time.Now().Add(24 * time.Hour),
			DNSNames: []string{"elinproxy.com", "www.elinproxy.com", "*.stage.elinproxy.com"},
		},
	})
	c.Store(tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: time.Now().Add(24 * time.Hour),
			DNSNames: []string{"testing.com", "www.testing.com"},
		},
	})
	time.Sleep(10 * time.Millisecond)

	deletable, _ := c.Get("testing.com")
	c.Delete(deletable)

	for _, domain := range []string{"testing.com", "www.elinproxy.com", "testing.stage.elinproxy.com"} {
		foundCert, err := c.Get(domain)
		if domain == "testing.com" {
			if err != nil {
				t.Logf("OK cert %s: %s", domain, err)
				continue
			}
			t.Errorf("ERROR Domain %s: %v", domain, foundCert)
			continue
		}
		if err != nil {
			t.Errorf("ERROR Domain %s: %s", domain, err)
			continue
		}
		t.Logf("OK domain %s in cert:%s", domain, strings.Join(foundCert.Leaf.DNSNames, ","))
	}
}

func storeCerts() {
	c.Store(tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: time.Now().Add(24 * time.Hour),
			DNSNames: []string{"elinproxy.com", "www.elinproxy.com", "*.stage.elinproxy.com"},
		},
	})
	c.Store(tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: time.Now().Add(24 * time.Hour),
			DNSNames: []string{"testimg.com", "www.testing.com"},
		},
	})
}

func BenchmarkLoadCerts(b *testing.B) {

	b.ResetTimer()
	b.ReportAllocs()

	storeCerts()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			crt, err := c.Get("www.testing.com")
			if err != nil || crt == nil {
				b.Error(err)
			}
		}
	})
}

func BenchmarkLoadCertsWC(b *testing.B) {

	b.ResetTimer()
	b.ReportAllocs()

	storeCerts()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Get("one.stage.elinproxy.com")
		}
	})
}
