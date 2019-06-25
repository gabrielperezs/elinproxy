package main

import (
	"flag"
	"log"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/BurntSushi/toml"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gabrielperezs/elinproxy/httpsrv"
	"github.com/gabrielperezs/elinproxy/httpsrv/cacherules"
)

const (
	version = "0.3.1"
)

var (
	pprof          bool
	prometheusPort string
	serverConfFile string
	cacheConfFile  string
	server         *httpsrv.SRV
)

func main() {
	flag.StringVar(&serverConfFile, "server", "server.conf", "Config for the server")
	flag.StringVar(&cacheConfFile, "cache", "cache.conf", "Cache rules")
	flag.BoolVar(&pprof, "pprof", false, "Enable pprof in :6060")
	flag.StringVar(&prometheusPort, "prometheusport", ":2112", "Prometheus metrics via HTTP")
	flag.Parse()

	if pprof {
		go func() {
			log.Printf("Starting http/pprof port :6060")
			http.ListenAndServe(":6060", nil)
		}()
	}

	if prometheusPort != "false" {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("Prometheus metrics error: %s", err)
				}
			}()
			http.Handle("/metrics", promhttp.Handler())
			http.ListenAndServe(prometheusPort, nil)
		}()
	}

	startOrReload()
}

func startOrReload() {
	log.Printf("Version %s", version)

	srvConf, err := loadServerConf(serverConfFile)
	if err != nil {
		log.Printf("ERROR Server Config: %v", err)
		return
	}
	srvConf.Handler.CacheRules, err = loadCacheRules(cacheConfFile)
	if err != nil {
		log.Printf("ERROR Cache Config: %v", err)
		return
	}

	if server != nil {
		server.Reload(srvConf)
	} else {
		server = httpsrv.New(srvConf)
		server.Listen()
	}
}

func loadServerConf(file string) (*httpsrv.Config, error) {
	srvConf := &httpsrv.Config{}
	if _, err := toml.DecodeFile(serverConfFile, srvConf); err != nil {
		return nil, err
	}
	var err error
	srvConf.Handler.Cache.ExtraTTL, err = time.ParseDuration(srvConf.Handler.Cache.ExtraTTLString)
	if err != nil {
		log.Printf("D: Cache ExtraTTL: %s", srvConf.Handler.Cache.ExtraTTLString)
		return nil, err
	}
	srvConf.Handler.Cache.MinLSMTTL, err = time.ParseDuration(srvConf.Handler.Cache.MinLSMTTLString)
	if err != nil {
		log.Printf("D: Cache MinLSMTTL: %s", srvConf.Handler.Cache.MinLSMTTLString)
		return nil, err
	}
	return srvConf, nil
}

func loadCacheRules(file string) (*cacherules.Rules, error) {
	cacheConf := &cacherules.Rules{}
	if _, err := toml.DecodeFile(file, cacheConf); err != nil {
		return nil, err
	}
	cacheConf.Parse()
	return cacheConf, nil
}
