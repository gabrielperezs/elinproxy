package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"

	// register transports
	_ "nanomsg.org/go/mangos/v2/transport/all"
)

var (
	elinproxySocket string
	cli             *client
)

func main() {
	flag.StringVar(&elinproxySocket, "socket", "ipc:///tmp/elinproxy.sock", "UNIX socket of the elinproxy server")
	flag.Parse()

	// All logs messages just to ERROR output
	log.SetOutput(os.Stderr)

	cli = newClient(elinproxySocket)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(strings.Replace(elinproxySocket, "ipc://", "", 1))
	if err != nil {
		log.Fatal(err)
	}

	startWatcher(watcher)
}

func startWatcher(watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("modified file:", event.Name)
			}
			cli.reconnect()
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
			cli.reconnect()
		}
	}
}
