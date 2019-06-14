package certs

type Config struct {
	CA             string
	Email          string
	PEM            []PEMCert
	EtcdEndpoints  []string
	AltHTTPPort    int
	AltTLSALPNPort int
}
