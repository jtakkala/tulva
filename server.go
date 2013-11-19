// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"launchpad.net/tomb"
	"log"
	"math/rand"
	"net"
	"syscall"
	"time"
)

type Server struct {
	connsCh  chan net.Conn
	statsCh  chan Stats
	Port     uint16
	Listener net.Listener
	t        tomb.Tomb
}

func NewServer() *Server {
	sv := new(Server)

	sv.connsCh = make(chan net.Conn)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var err error

	// Randomly choose a port between 16384 - 65535
	// Try up to 10 times, then exit if we can't bind
	for i := 0; ; i++ {
		sv.Port = uint16(r.Intn(49151)) + uint16(16384)
		portString := fmt.Sprintf(":%d", sv.Port)
		// TODO: Undo override of default port
		sv.Port = uint16(6881)
		portString = ":6881"
		sv.Listener, err = net.Listen("tcp4", portString)
		if err != nil {
			if e, ok := err.(*net.OpError); ok {
				// If reason is EADDRINUSE, then try up to 10 times
				if e.Err == syscall.EADDRINUSE {
					if i < 10 {
						log.Printf("Failed to bind to port %d. Trying again...\n", sv.Port)
						continue
					} else {
						log.Println("Unable to bind to port. Giving up")
						log.Fatal(err)
					}
				}
			}
			// Bail here on any other errors
			log.Fatal(err)
		}
		// Success
		break
	}
	log.Println("Server : Listening on port", sv.Port)

	return sv
}

func (sv *Server) Listen() {
	log.Println("Server : Listen : Started")
	defer sv.Listener.Close()
	defer log.Println("Server : Listen : Completed")

	for {
		conn, err := sv.Listener.Accept()
		if err != nil {
			log.Println(err)
			return
		}
		log.Println("Received new connection from:", conn.RemoteAddr())
		sv.connsCh <- conn
	}
}

func (sv *Server) Stop() error {
	log.Println("Server : Stop : Stopping")
	sv.t.Kill(nil)
	return sv.t.Wait()
}

func (sv *Server) Run() {
	log.Println("Server : Run : Started")
	defer sv.t.Done()
	defer log.Println("Server : Run : Completed")

	go sv.Listen()
	for {
		select {
		case <-sv.t.Dying():
			sv.Listener.Close()
			return
		}
	}
}
