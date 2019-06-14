package main

import (
	"fmt"
	"log"
	"time"

	mangos "nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/protocol"
	"nanomsg.org/go/mangos/v2/protocol/sub"
)

type client struct {
	name  string
	sock  protocol.Socket
	close chan bool
}

func newClient(name string) *client {
	c := &client{
		name: name,
	}
	c.connect()
	return c
}

func (c *client) connect() {
	for {
		err := c.realconnect()
		if err == nil {
			go c.listen()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (c *client) realconnect() (err error) {
	c.sock, err = sub.NewSocket()
	if err != nil {
		log.Printf("can't get new pair socket: %s", err)
		return
	}

	if err = c.sock.Dial(c.name); err != nil {
		log.Printf("ERROR Dial %s: %s", c.name, err)
		return
	}

	err = c.sock.SetOption(mangos.OptionSubscribe, []byte{})
	if err != nil {
		log.Printf("cannot subscribe: %v", err)
		c.sock.Close()
		return
	}
	return
}

func (c *client) reconnect() {
	c.sock.Close()
	c.connect()
}

func (c *client) listen() {
	for {
		msg, err := c.sock.Recv()
		if err != nil {
			if err != mangos.ErrClosed {
				log.Printf("Error: %s", err)
			}
			return
		}
		// Print to std output the content of the message
		// as a simple string
		fmt.Println(string(msg))
	}
}

func (c *client) Done() {
	c.sock.Close()
}
