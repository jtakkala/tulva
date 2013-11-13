// Copyright 2013 Jari Takkala. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"launchpad.net/tomb"
	"log"
	"net"
)

// PeerTuple represents a single IP+port pair of a peer
type PeerTuple struct {
	IP net.IP
	Port uint16
}

type Peer struct {
	peer PeerTuple
}

type PeerManager struct {
	peersCh <-chan PeerTuple
	statsCh chan Stats
	t tomb.Tomb
}

func NewPeerManager(peersCh chan PeerTuple, statsCh chan Stats) *PeerManager {
	pm := new(PeerManager)
	pm.peersCh = peersCh
	pm.statsCh = statsCh
	return pm
}

func (pm *PeerManager) Stop() error {
	pm.t.Kill(nil)
	return pm.t.Wait()
}

func (pm *PeerManager) Run() {
	log.Println("PeerManager : Run : Started")
	defer pm.t.Done()
	defer log.Println("PeerManager : Run : Completed")

	for {
		select {
		case peer := <- pm.peersCh:
			/*
			_, ok := peers[peer]
			if ok {
				// peer already exists
				fmt.Println("Peer already in map")
			} else {
				peers[peer] = "foo"
			}
			*/
			fmt.Println(peer)
		case <- pm.t.Dying():
			return
		}
	}
}
