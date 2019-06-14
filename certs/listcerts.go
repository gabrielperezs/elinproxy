package certs

import (
	"crypto/tls"
	"strings"
)

type listCerts map[string]*tls.Certificate

func (l listCerts) Set(domain string, cert *tls.Certificate) {
	l[domain] = cert
	return
}

func (l listCerts) Get(domain string) (cert *tls.Certificate, ok bool) {
	cert, ok = l[domain]
	return
}

func (l listCerts) GetWC(domain string) (cert *tls.Certificate, ok bool) {
	var candidate string
	for candidate, cert = range l {
		if strings.HasSuffix(domain, candidate) {
			ok = true
			return
		}
	}
	return
}
