// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"launchpad.net/tomb"
	"log"
	"net"
)

type serverPeerChans struct {
	conns chan *net.TCPConn
}

type Server struct {
	Port      uint16
	Listener  *net.TCPListener
	peerChans serverPeerChans
	t         tomb.Tomb
}

func NewServer() *Server {
	sv := new(Server)

	// Send any new connections we receive to PeerManager
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

func (sv *Server) Listen() {
	log.Println("Server : Listen : Started")
	defer sv.Listener.Close()
	defer log.Println("Server : Listen : Completed")

	for {
		conn, err := sv.Listener.AcceptTCP()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Server: New connection from:", conn.RemoteAddr())
		sv.peerChans.conns <- conn
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
