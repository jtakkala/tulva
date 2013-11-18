// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	//natpmp "code.google.com/p/go-nat-pmp"
	"fmt"
	"launchpad.net/tomb"
	"log"
	"math/rand"
	"net"
	"syscall"
	"time"
)

type Server struct {
	peersCh <-chan PeerTuple
	statsCh chan Stats
	Port    uint16
	Listener net.Listener
	t       tomb.Tomb
}

func NewServer() *Server {
	sv := new(Server)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var err error

	// Randomly choose a port between 16384 - 65535
	// Try up to 10 times, then exit if we can't bind
	for i := 0; ; i++ {
		sv.Port = uint16(r.Intn(49151)) + uint16(16384)
		portString := fmt.Sprintf(":%d", sv.Port)
		sv.Listener, err = net.Listen("tcp4", portString)
		if err != nil {
			if e, ok := err.(*net.OpError); ok {
				// If reason is EADDRINUSE, then try up to 10 times
				if (e.Err == syscall.EADDRINUSE) {
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
		break
	}

	return sv
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

	fmt.Println("Listening on port", sv.Port)
	for {
		select {
		case peer := <-sv.peersCh:
			/*
				_, ok := peers[peer]
				if ok {
					// peer already exists
					fmt.Println("Server already in map")
				} else {
					peers[peer] = "foo"
				}
			*/
			fmt.Println(peer)
		case <-sv.t.Dying():
			return
		}
	}
}
