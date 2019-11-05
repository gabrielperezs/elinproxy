package certs

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/cpuid"
	"github.com/mholt/certmagic"
)

type PEMCert struct {
	Domains string
	Key     string
	Cert    string
}

type Certs struct {
	sync.RWMutex
	config  *Config
	acme    *certmagic.Config
	certsWC listCerts
	certs   listCerts
}

const (
	whileCardPrefix = "*."
)

var (
	ErrCertExpired  = errors.New("Certificate expired")
	ErrCertNotFound = errors.New("Certificate not found")
)

func New(cfg *Config) *Certs {

	if cfg == nil {
		log.Panicf("Can't load without configuration for certificates")
		return nil
	}

	c := &Certs{
		acme:    certmagic.NewDefault(),
		certs:   nil,
		certsWC: nil,
	}
	c.Reload(cfg)

	return c
}

func (c *Certs) TLSConfig() *tls.Config {
	tls := c.acme.TLSConfig()
	tls.CipherSuites = preferredDefaultCipherSuites()
	return tls
}

// preferredDefaultCipherSuites returns an appropriate
// cipher suite to use depending on hardware support
// for AES-NI.
//
// See https://github.com/mholt/caddy/issues/1674
func preferredDefaultCipherSuites() []uint16 {
	if cpuid.CPU.AesNi() {
		return defaultCiphersPreferAES
	}
	return defaultCiphersPreferChaCha
}

var (
	defaultCiphersPreferAES = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		// For windows 2010
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	}
	defaultCiphersPreferChaCha = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		// For windows 2010
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	}
)

func (c *Certs) Reload(cfg *Config) {
	c.LoadCertsFromConfig(cfg.PEM)
	c.Lock()
	c.acme.Storage = newStorage(cfg.EtcdEndpoints)
	c.acme.CA = cfg.CA
	c.acme.Agreed = true
	c.acme.Email = cfg.Email
	c.acme.AltHTTPPort = cfg.AltHTTPPort
	c.acme.AltTLSALPNPort = cfg.AltTLSALPNPort
	c.acme.DisableTLSALPNChallenge = true
	c.acme.OnDemand = &certmagic.OnDemandConfig{
		DecisionFunc: func(name string) error {
			log.Printf("[D] certs/certmagic OnDemand: %s", name)
			return nil
		},
	}
	c.Unlock()
}

func (c *Certs) Store(cert tls.Certificate) error {
	c.Lock()
	defer c.Unlock()

	newList, newListWC, err := c.copySwap(nil)
	if err != nil {
		return err
	}

	for _, n := range cert.Leaf.DNSNames {
		if n == "" {
			continue
		}

		if strings.HasPrefix(n, whileCardPrefix) {
			// Removed the *
			newListWC.Set(n[1:], &cert)
		} else {
			newList.Set(n, &cert)
		}
	}

	c.certs = newList
	c.certsWC = newListWC

	return nil
}

func (c *Certs) copySwap(exclude *tls.Certificate) (listCerts, listCerts, error) {
	newList := make(listCerts)
	if c.certs != nil {
		for n, cert := range c.certs {
			if exclude != nil && exclude == cert {
				continue
			}
			newList.Set(n, cert)
		}
	}

	newListWC := make(listCerts)
	if c.certsWC != nil {
		for n, cert := range c.certsWC {
			if exclude != nil && exclude == cert {
				continue
			}
			newListWC.Set(n, cert)
		}
	}

	return newList, newListWC, nil
}

func (c *Certs) Delete(cert *tls.Certificate) error {
	c.Lock()
	defer c.Unlock()

	newList, newListWC, err := c.copySwap(cert)
	if err != nil {
		return err
	}

	c.certs = newList
	c.certsWC = newListWC

	return nil
}

// Get gets a valid Certificate struct from a domain name
func (c *Certs) Get(domain string) (cert *tls.Certificate, err error) {
	var ok bool
	cert, ok = c.certs.Get(domain)
	if ok {
		if time.Now().UTC().After(cert.Leaf.NotAfter) {
			return cert, ErrCertExpired
		}
		return
	}

	cert, ok = c.certsWC.GetWC(domain)
	if ok {
		if time.Now().UTC().After(cert.Leaf.NotAfter) {
			return cert, ErrCertExpired
		}
		return
	}

	return nil, ErrCertNotFound
}

func (c *Certs) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello.ServerName == "" {
		return nil, ErrCertNotFound
	}

	cert, err := c.Get(hello.ServerName)
	if err == nil {
		return cert, err
	}

	if cert != nil {
		c.Delete(cert)
		log.Printf("Certs/GetCertificate Delete %s: %s %s", hello.ServerName, err, cert.Leaf.NotAfter.String())
	}

	acme := c.acme
	cert, err = acme.GetCertificate(hello)
	if err != nil {
		log.Printf("ERROR certs GetCertificate:Remote (%s): %s", hello.ServerName, err)
		return nil, err
	}

	if cert.Leaf == nil {
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, err
		}
	}

	// Everything ok, store the certificate
	c.Store(*cert)
	return cert, nil
}

func (c *Certs) LoadCertsFromConfig(pems []PEMCert) {
	for i, v := range pems {
		crt, err := tls.X509KeyPair([]byte(v.Cert), []byte(v.Key))
		if err != nil {
			log.Printf("ERROR certs X509KeyPair %d: %s", i, err)
			continue
		}
		crt.Leaf, err = x509.ParseCertificate(crt.Certificate[0])
		if err != nil {
			log.Printf("ERROR httpsrv ParseCertificate %d: %s", i, err)
			continue
		}
		log.Printf("certs/LoadCertsFromConfig %s (%s)", strings.Join(crt.Leaf.DNSNames, ","), crt.Leaf.NotAfter.String())
		c.Store(crt)
	}
}

func (c *Certs) HTTPChallengeHandler(h http.Handler) http.Handler {
	return c.acme.HTTPChallengeHandler(h)
}
