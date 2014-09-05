// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net"
	"time"
)

type serverPeerChans struct {
	conns chan *net.TCPConn
}

type Server struct {
	Port      uint16
	Listener  *net.TCPListener
	peerChans serverPeerChans
	quit      chan struct{}
}

func NewServer() *Server {
	sv := &Server{quit: make(chan struct{})}

	// Channel used to send new connections we receive to PeerManager
	sv.peerChans.conns = make(chan *net.TCPConn)

	var err error
	sv.Listener, err = net.ListenTCP("tcp4", &net.TCPAddr{net.ParseIP("0.0.0.0"), 0, ""})
	if err != nil {
		log.Fatal(err)
	}
	sv.Port = uint16(sv.Listener.Addr().(*net.TCPAddr).Port)
	log.Println("Server : Listening on port", sv.Port)

	return sv
}

// Serve accepts new TCP connections and hands them off to PeerManager
func (sv *Server) Serve() {
	log.Println("Server : Serve : Started")
	defer log.Println("Server : Serve : Completed")

	for {
		// Check if we should stop accepting connections and shutdown
		select {
		case <-sv.quit:
			log.Println("Server : Serve : Shutting Down")
			sv.Listener.Close()
			return
		default:
		}
		// Accept a new connection or timeout and loop again
		sv.Listener.SetDeadline(time.Now().Add(time.Second))
		conn, err := sv.Listener.AcceptTCP()
		if err != nil {
			// Loop if deadline expired
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			log.Fatal(err)
		}
		log.Println("Server: New connection from:", conn.RemoteAddr())
		// Hand the connection off to PeerManager
		go func() { sv.peerChans.conns <- conn }()
	}
}
