// Copyright 2016 The Gosl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net"
	"net/http"

	"github.com/cpmech/lootbag/lio"
	"github.com/cpmech/lootbag/neto"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// Hub implements the set of peers
type Hub struct {
	providers      map[net.Conn]bool
	observers      map[net.Conn]bool
	chanRemoveConn chan net.Conn
	chanBroadcast  chan []byte
}

// NewHub creates a new Hub
func NewHub() *Hub {
	o := &Hub{
		make(map[net.Conn]bool),
		make(map[net.Conn]bool),
		make(chan net.Conn),
		make(chan []byte),
	}
	go o.run()
	return o
}

// AddProvider adds new provider to the hub
func (o *Hub) AddProvider(w http.ResponseWriter, r *http.Request) {
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		log.Printf("!!! cannot upgrade connection !!!")
		lio.Ff(w, neto.RjsonFailed(err))
		return
	}
	log.Printf("### added provider ###")
	o.providers[conn] = true
	go func() {
		defer conn.Close()
		log.Printf("+++ waiting for provider messages +++")
		for {
			msg, _, err := wsutil.ReadClientData(conn)
			if err != nil {
				o.chanRemoveConn <- conn
				return
			}
			o.chanBroadcast <- msg
		}
	}()
}

// AddObserver adds new observer to the hub
func (o *Hub) AddObserver(w http.ResponseWriter, r *http.Request) {
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		log.Printf("!!! cannot upgrade connection !!!")
		lio.Ff(w, neto.RjsonFailed(err))
		return
	}
	log.Printf("### added observer ###")
	o.observers[conn] = true
}

// run runs the hub
func (o *Hub) run() {
	for {
		select {
		case conn := <-o.chanRemoveConn:
			if _, ok := o.providers[conn]; ok {
				delete(o.providers, conn)
			}
			if _, ok := o.observers[conn]; ok {
				delete(o.observers, conn)
			}
		case message := <-o.chanBroadcast:
			log.Printf("got message: %v\n", string(message))
			for conn := range o.observers {
				err := wsutil.WriteServerMessage(conn, ws.OpText, message)
				if err != nil {
					log.Printf("!!! error while writing message !!!")
					conn.Close()
					delete(o.observers, conn)
				}
			}
		}
	}
}