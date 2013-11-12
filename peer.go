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

type Peer struct {
	IP net.IP
	Port uint16
}

type PeerManager struct {
	peersCh <-chan Peer
	statsCh chan Stats
	t tomb.Tomb
}

func (pm *PeerManager) Stop() error {
	pm.t.Kill(nil)
	return pm.t.Wait()
}

func (pm *PeerManager) Run() {
	log.Println("PeerManager : Run : Started")
	defer pm.t.Done()
	defer log.Println("PeerManager : Run : Completed")

	peers := make(map[string]uint16)

	for {
		select {
		case peer := <- pm.peersCh:
			peers[peer.IP.String()] = peer.Port
			fmt.Println(peer)
		case <- pm.t.Dying():
			return
		}
	}
}
